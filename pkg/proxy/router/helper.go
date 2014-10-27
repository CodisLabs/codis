package router

import (
	"bufio"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/wandoulabs/codis/pkg/models"
	"github.com/wandoulabs/codis/pkg/proxy/parser"

	log "github.com/ngaut/logging"

	"github.com/juju/errors"
	topo "github.com/ngaut/go-zookeeper/zk"
	stats "github.com/ngaut/gostats"

	respcoding "github.com/ngaut/resp"
)

var blackList = []string{
	"KEYS", "MOVE", "OBJECT", "RENAME", "RENAMENX", "SORT", "SCAN", "BITOP" /*"MGET",*/ /* "MSET",*/, "MSETNX", "SCAN",
	"BLPOP", "BRPOP", "BRPOPLPUSH", "PSUBSCRIBE，PUBLISH", "PUNSUBSCRIBE", "SUBSCRIBE",
	"UNSUBSCRIBE", "DISCARD", "EXEC", "MULTI", "UNWATCH", "WATCH", "SCRIPT EXISTS", "SCRIPT FLUSH", "SCRIPT KILL",
	"SCRIPT LOAD" /*, "AUTH" , "ECHO"*/ /*"QUIT",*/ /*"SELECT",*/, "BGREWRITEAOF", "BGSAVE", "CLIENT KILL", "CLIENT LIST",
	"CONFIG GET", "CONFIG SET", "CONFIG RESETSTAT", "DBSIZE", "DEBUG OBJECT", "DEBUG SEGFAULT", "FLUSHALL", "FLUSHDB",
	"INFO", "LASTSAVE", "MONITOR", "SAVE", "SHUTDOWN", "SLAVEOF", "SLOWLOG", "SYNC", "TIME", "SLOTSMGRTONE", "SLOTSMGRT",
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
	if op == "MGET" || op == "DEL" {
		return true
	}

	return false
}

func validSlot(i int) bool {
	if i < 0 || i >= models.DEFAULT_SLOT_NUM {
		log.Errorf("invalid slot number, %+v", i)
		return false
	}

	return true
}

func WriteMigrateKeyCmd(w io.Writer, addr string, timeoutMs int, key []byte) error {
	hostPort := strings.Split(addr, ":")
	if len(hostPort) != 2 {
		return errors.Trace(errors.New("invalid address " + addr))
	}
	respW := respcoding.NewRESPWriter(w)
	err := respW.WriteCommand("slotsmgrttagone", hostPort[0], hostPort[1],
		strconv.Itoa(int(timeoutMs)), string(key))
	if err != nil {
		return errors.Trace(err)
	}

	return err
}

type DeadlineReadWriter interface {
	io.Writer
	io.Reader
	SetWriteDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
}

func handleSpecCommand(cmd string, clientWriter DeadlineReadWriter, resp *parser.Resp) (bool, bool, error) {
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
		keys, _ := resp.Keys()
		if len(keys) > 0 {
			var err error
			b, err = respcoding.Marshal(string(keys[0]))
			if err != nil {
				return true, false, errors.Trace(err)
			}
		} else {
			return true, false, nil
		}
	}

	if len(b) > 0 {
		clientWriter.SetWriteDeadline(time.Now().Add(5 * time.Second))
		_, err := clientWriter.Write(b)
		if err != nil {
			return shouldClose, true, errors.Errorf("%s, cmd:%s", err.Error(), cmd)
		}

		return shouldClose, true, nil
	}

	return shouldClose, false, nil
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
	if err != nil {
		return nil, errors.Trace(err)
	}

	return nil, nil
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
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

type BufioDeadlineReadWriter interface {
	DeadlineReadWriter
	BufioReader() *bufio.Reader
}

func forward(c DeadlineReadWriter, redisConn BufioDeadlineReadWriter, resp *parser.Resp) (redisErr error, clientErr error) {
	redisReader := redisConn.BufioReader()
	if err := redisConn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	if err := write2Redis(resp, redisConn); err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	if err := redisConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return errors.Trace(err), errors.Trace(err)
	}

	if err := c.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
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
		log.Fatalf("ulimit too small: %d, should be at least %d", min, n)
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

func getOpKey(resp *parser.Resp) ([]byte, []byte, error) {
	op, err := resp.Op()
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	if len(op) == 0 || len(op) > 50 {
		return nil, nil, errors.Errorf("error parse op %s", string(op))
	}

	k, err := resp.Key()
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	return op, k, nil
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
	default:
		c.Add(">=5000ms", 1)
	}
}

func isTheSameSlot(keys [][]byte) bool {
	firstSlot := -1
	for _, k := range keys {
		if firstSlot == -1 {
			firstSlot = mapKey2Slot(k)
		} else {
			if firstSlot != mapKey2Slot(k) {
				return false
			}
		}
	}

	return true
}
