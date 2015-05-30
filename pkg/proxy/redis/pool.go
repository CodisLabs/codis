package redis

import (
	"container/list"
	"sync"
	"time"

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
		x := connPool.Remove(connPool.Front()).(*connPoolElem)
		c := x.conn
		if c.ReaderLastUnix < lastunix && c.WriterLastUnix < lastunix {
			log.Infof("pool conn: [%p] to %s, closed due to timeout", c, c.Sock.RemoteAddr())
			c.Close()
		} else {
			connPool.PushBack(x)
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
	c, err := DialTimeout(addr, time.Second)
	if err != nil {
		return nil, err
	}
	log.Infof("pool conn: [%p] to %s, create new connection", c, c.Sock.RemoteAddr())
	return c, nil
}

var mgrttagone = []byte("slotsmgrttagone")

func SlotsMgrtTagOne(addr string, host []byte, port []byte, key []byte) (*Resp, error) {
	c, err := getPoolConn(addr)
	if err != nil {
		return nil, err
	}
	defer putPoolConn(c, addr)

	c.ReaderTimeout = time.Minute
	c.WriterTimeout = time.Minute

	resp := NewArray([]*Resp{
		NewBulkBytes(mgrttagone),
		NewBulkBytes(host),
		NewBulkBytes(port),
		NewBulkBytes(itob(1000)),
		NewBulkBytes(key),
	})
	if err := c.Writer.Encode(resp, true); err != nil {
		return nil, err
	}
	return c.Reader.Decode()
}
