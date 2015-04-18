// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/wandoulabs/codis/pkg/utils"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/group"
	"github.com/wandoulabs/codis/pkg/proxy/parser"
	"github.com/wandoulabs/codis/pkg/proxy/router/topology"

	log "github.com/ngaut/logging"

	"github.com/juju/errors"
	topo "github.com/ngaut/go-zookeeper/zk"
	stats "github.com/ngaut/gostats"

	respcoding "github.com/ngaut/resp"
)

var blackList = []string{
	"KEYS", "MOVE", "OBJECT", "RENAME", "RENAMENX", "SORT", "SCAN", "BITOP" /*"MGET",*/ /* "MSET",*/, "MSETNX", "SCAN",
	"BLPOP", "BRPOP", "BRPOPLPUSH", "PSUBSCRIBE，PUBLISH", "PUNSUBSCRIBE", "SUBSCRIBE", "RANDOMKEY",
	"UNSUBSCRIBE", "DISCARD", "EXEC", "MULTI", "UNWATCH", "WATCH", "SCRIPT EXISTS", "SCRIPT FLUSH", "SCRIPT KILL",
	"SCRIPT LOAD" /*, "AUTH" , "ECHO"*/ /*"QUIT",*/ /*"SELECT",*/, "BGREWRITEAOF", "BGSAVE", "CLIENT KILL", "CLIENT LIST",
	"CONFIG GET", "CONFIG SET", "CONFIG RESETSTAT", "DBSIZE", "DEBUG OBJECT", "DEBUG SEGFAULT", "FLUSHALL", "FLUSHDB",
	"LASTSAVE", "MONITOR", "SAVE", "SHUTDOWN", "SLAVEOF", "SLOWLOG", "SYNC", "TIME", "SLOTSMGRTONE", "SLOTSMGRT",
	"SLOTSDEL",
}

var (
	blackListCommand = make(map[string]struct{})
	OK_BYTES         = []byte("+OK\r\n")
)

func init() {
	for _, k := range blackList {
		blackListCommand[k] = struct{}{}
	}
}

func allowOp(op string) bool {
	_, black := blackListCommand[op]
	return !black
}

func isMulOp(op string) bool {
	if op == "MGET" || op == "DEL" || op == "MSET" {
		return true
	}

	return false
}

func validSlot(i int) bool {
	if i < 0 || i >= models.DEFAULT_SLOT_NUM {
		return false
	}

	return true
}

func WriteMigrateKeyCmd(w io.Writer, addr string, timeoutMs int, key []byte) error {
	hostPort := strings.Split(addr, ":")
	if len(hostPort) != 2 {
		return errors.Errorf("invalid address " + addr)
	}
	respW := respcoding.NewRESPWriter(w)
	err := respW.WriteCommand("slotsmgrttagone", hostPort[0], hostPort[1],
		strconv.Itoa(int(timeoutMs)), string(key))
	return errors.Trace(err)
}

type DeadlineReadWriter interface {
	io.ReadWriter
	SetWriteDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
}

func handleSpecCommand(cmd string, keys [][]byte, timeout int) ([]byte, bool, bool, error) {
	var b []byte
	shouldClose := false
	switch cmd {
	case "PING":
		b = []byte("+PONG\r\n")
	case "QUIT":
		b = OK_BYTES
		shouldClose = true
	case "SELECT":
		b = OK_BYTES
	case "AUTH":
		b = OK_BYTES
	case "ECHO":
		if len(keys) > 0 {
			var err error
			b, err = respcoding.Marshal(string(keys[0]))
			if err != nil {
				return nil, true, false, errors.Trace(err)
			}
		} else {
			return nil, true, false, nil
		}
	}

	if len(b) > 0 {
		return b, shouldClose, true, nil
	}

	return b, shouldClose, false, nil
}

func write2Client(redisReader *bufio.Reader, clientWriter io.Writer) (redisErr error, clientErr error) {
	resp, err := parser.Parse(redisReader)
	if err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	b, err := resp.Bytes()
	if err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	_, err = clientWriter.Write(b)
	return nil, errors.Trace(err)
}

func write2Redis(resp *parser.Resp, redisWriter io.Writer) error {
	// get resp in bytes
	b, err := resp.Bytes()
	if err != nil {
		return errors.Trace(err)
	}

	return writeBytes2Redis(b, redisWriter)
}

func writeBytes2Redis(b []byte, redisWriter io.Writer) error {
	// write to redis
	_, err := redisWriter.Write(b)
	return errors.Trace(err)
}

type BufioDeadlineReadWriter interface {
	DeadlineReadWriter
	BufioReader() *bufio.Reader
}

func forward(c DeadlineReadWriter, redisConn BufioDeadlineReadWriter, resp *parser.Resp, timeout int) (redisErr error, clientErr error) {
	redisReader := redisConn.BufioReader()
	if err := redisConn.SetWriteDeadline(time.Now().Add(time.Duration(timeout) * time.Second)); err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	if err := write2Redis(resp, redisConn); err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	if err := redisConn.SetReadDeadline(time.Now().Add(time.Duration(timeout) * time.Second)); err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	if err := c.SetWriteDeadline(time.Now().Add(time.Duration(timeout) * time.Second)); err != nil {
		return nil, errors.Trace(err)
	}

	// read and parse redis response
	return write2Client(redisReader, c)
}

func StringsContain(s []string, key string) bool {
	for _, val := range s {
		if val == key { //need our resopnse
			return true
		}
	}

	return false
}

func getRespOpKeys(c *session) (*parser.Resp, []byte, [][]byte, error) {
	resp, err := parser.Parse(c.r) // read client request
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}

	op, keys, err := resp.GetOpKeys()
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}

	if len(keys) == 0 {
		keys = [][]byte{[]byte("fakeKey")}
	}

	return resp, op, keys, nil
}

func filter(opstr string, keys [][]byte, c *session, timeoutSec int) (rawresp []byte, next bool, err error) {
	if !allowOp(opstr) {
		return nil, false, errors.Trace(fmt.Errorf("%s not allowed", opstr))
	}

	buf, shouldClose, handled, err := handleSpecCommand(opstr, keys, timeoutSec)
	if shouldClose { //quit command
		return buf, false, errors.Trace(io.EOF)
	}
	if err != nil {
		return nil, false, errors.Trace(err)
	}

	if handled {
		return buf, false, nil
	}

	return nil, true, nil
}

func GetEventPath(evt interface{}) string {
	return evt.(topo.Event).Path
}

func CheckUlimit(min int) {
	ulimitN, err := exec.Command("/bin/sh", "-c", "ulimit -n").Output()
	if err != nil {
		log.Warning("get ulimit failed", err)
	}

	n, err := strconv.Atoi(strings.TrimSpace(string(ulimitN)))
	if err != nil || n < min {
		log.Fatalf("ulimit too small: %d, should be at least %d", n, min)
	}
}

func GetOriginError(err *errors.Err) error {
	if err != nil {
		if err.Cause() == nil && err.Underlying() == nil {
			return err
		} else {
			return err.Underlying()
		}
	}

	return err
}

func recordResponseTime(c *stats.Counters, d time.Duration) {
	switch {
	case d < 5:
		c.Add("0-5ms", 1)
	case d >= 5 && d < 10:
		c.Add("5-10ms", 1)
	case d >= 10 && d < 50:
		c.Add("10-50ms", 1)
	case d >= 50 && d < 200:
		c.Add("50-200ms", 1)
	case d >= 200 && d < 1000:
		c.Add("200-1000ms", 1)
	case d >= 1000 && d < 5000:
		c.Add("1000-5000ms", 1)
	case d >= 5000 && d < 10000:
		c.Add("5000-10000ms", 1)
	default:
		c.Add("10000ms+", 1)
	}
}

type killEvent struct {
	done chan error
}

type Conf struct {
	proxyId     string
	productName string
	zkAddr      string
	f           topology.ZkFactory
	netTimeout  int    //seconds
	proto       string //tcp or tcp4
	provider    string
}

func LoadConf(configFile string) (*Conf, error) {
	srvConf := &Conf{}
	conf, err := utils.InitConfigFromFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	srvConf.productName, _ = conf.ReadString("product", "test")
	if len(srvConf.productName) == 0 {
		log.Fatalf("invalid config: product entry is missing in %s", configFile)
	}
	srvConf.zkAddr, _ = conf.ReadString("zk", "")
	if len(srvConf.zkAddr) == 0 {
		log.Fatalf("invalid config: need zk entry is missing in %s", configFile)
	}
	srvConf.zkAddr = strings.TrimSpace(srvConf.zkAddr)

	srvConf.proxyId, _ = conf.ReadString("proxy_id", "")
	if len(srvConf.proxyId) == 0 {
		log.Fatalf("invalid config: need proxy_id entry is missing in %s", configFile)
	}

	srvConf.netTimeout, _ = conf.ReadInt("net_timeout", 5)
	srvConf.proto, _ = conf.ReadString("proto", "tcp")
	srvConf.provider, _ = conf.ReadString("coordinator", "zookeeper")
	log.Infof("%+v", srvConf)

	return srvConf, nil
}

type Slot struct {
	slotInfo    *models.Slot
	groupInfo   *models.ServerGroup
	dst         *group.Group
	migrateFrom *group.Group
}

type OnSuicideFun func() error

func needResponse(receivers []string, self models.ProxyInfo) bool {
	var pi models.ProxyInfo
	for _, v := range receivers {
		err := json.Unmarshal([]byte(v), &pi)
		if err != nil {
			//is it old version of dashboard
			if v == self.Id {
				return true
			}
			return false
		}

		if pi.Id == self.Id && pi.Pid == self.Pid && pi.StartAt == self.StartAt {
			return true
		}
	}

	return false
}
