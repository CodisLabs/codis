package redis

import (
	"container/list"
	"strconv"
	"sync"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/errors"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

var connPool struct {
	sync.Mutex
	list.List
}

type connPoolElem struct {
	addr string
	conn *Conn
}

func init() {
	go func() {
		for {
			time.Sleep(time.Second * 5)
			lastunix := time.Now().Unix() - 10
			cleanupPool(lastunix)
		}
	}()
}

func cleanupPool(lastunix int64) {
	connPool.Lock()
	for i := connPool.Len(); i != 0; i-- {
		e := connPool.Front()
		c := e.Value.(*connPoolElem).conn
		if c.IsTimeout(lastunix) {
			c.Close()
			log.Infof("pool conn: [%p] to %s, closed due to timeout", c, c.Sock.RemoteAddr())
			connPool.Remove(e)
		} else {
			connPool.MoveToBack(e)
		}
	}
	connPool.Unlock()
}

func putPoolConn(c *Conn, addr string) {
	if c.Reader.Err != nil || c.Writer.Err != nil {
		log.Infof("pool conn: [%p] to %s, closed due to error", c, c.Sock.RemoteAddr())
		c.Close()
	} else {
		connPool.Lock()
		connPool.PushFront(&connPoolElem{
			addr: addr, conn: c,
		})
		connPool.Unlock()
	}
}

func getPoolConn(addr string) (*Conn, error) {
	var c *Conn
	connPool.Lock()
	for e := connPool.Front(); e != nil; e = e.Next() {
		x := e.Value.(*connPoolElem)
		if x.addr == addr {
			c = x.conn
			connPool.Remove(e)
			break
		}
	}
	connPool.Unlock()
	if c != nil {
		return c, nil
	}
	c, err := DialTimeout(addr, 1024*64, time.Second)
	if err != nil {
		return nil, err
	}
	log.Infof("pool conn: [%p] to %s, create new connection", c, c.Sock.RemoteAddr())
	return c, nil
}

var mgrttagone = []byte("slotsmgrttagone")

func SlotsMgrtTagOne(addr string, host []byte, port []byte, key []byte) (int, error) {
	c, err := getPoolConn(addr)
	if err != nil {
		return 0, err
	}
	defer putPoolConn(c, addr)

	c.ReaderTimeout = time.Minute
	c.WriterTimeout = time.Minute

	resp := NewArray([]*Resp{
		NewBulkBytes(mgrttagone),
		NewBulkBytes(host),
		NewBulkBytes(port),
		NewBulkBytes(itob(3000)),
		NewBulkBytes(key),
	})
	if err := c.Writer.Encode(resp, true); err != nil {
		return 0, err
	}
	if resp, err := c.Reader.Decode(); err != nil {
		return 0, err
	} else if resp.IsError() {
		return 0, errors.Errorf("error resp: %s", resp.Value)
	} else if resp.IsInt() {
		if n, err := strconv.ParseInt(string(resp.Value), 10, 64); err != nil {
			return 0, errors.Trace(err)
		} else {
			return int(n), nil
		}
	} else {
		return 0, errors.Errorf("bad response of slotsmgrttagone, should be integer")
	}
}
