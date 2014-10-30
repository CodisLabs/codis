package router

import (
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/juju/errors"
	log "github.com/ngaut/logging"
	respcoding "github.com/ngaut/resp"
)

type MultiOperator struct {
	q    chan *MulOp
	pool *redis.Pool
}

type MulOp struct {
	op   string
	keys [][]byte
	w    DeadlineReadWriter
	wait chan error
}

func NewMultiOperator(server string) *MultiOperator {
	oper := &MultiOperator{q: make(chan *MulOp, 5)}
	oper.pool = newPool(server, "")
	for i := 0; i < 5; i++ {
		go oper.work()
	}

	return oper
}

func newPool(server, password string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     5,
		IdleTimeout: 2400 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			//if _, err := c.Do("AUTH", password); err != nil {
			//	c.Close()
			//	return nil, err
			//}
			return c, err
		},
		//	TestOnBorrow: func(c redis.Conn, t time.Time) error {
		//		_, err := c.Do("PING")
		//		return err
		//	},
	}
}

func (oper *MultiOperator) handleMultiOp(op string, keys [][]byte, w DeadlineReadWriter) error {
	wait := make(chan error, 1)
	oper.q <- &MulOp{op: op, keys: keys, w: w, wait: wait}
	return <-wait
}

func (oper *MultiOperator) work() {
	for mop := range oper.q {
		switch mop.op {
		case "MGET":
			oper.mget(mop)
		case "DEL":
			oper.del(mop)
		}
	}
}

type pair struct {
	key []byte
	pos int
}

func getSlotMap(keys [][]byte) map[int][]*pair {
	slotmap := make(map[int][]*pair)
	for i, k := range keys { //get slots
		slot := mapKey2Slot(k)
		vec, exist := slotmap[slot]
		if !exist {
			vec = make([]*pair, 0)
		}
		vec = append(vec, &pair{key: k, pos: i})

		slotmap[slot] = vec
	}

	return slotmap
}

func (oper *MultiOperator) mgetResults(mop *MulOp) ([]byte, error) {
	slotmap := getSlotMap(mop.keys)
	results := make([]interface{}, len(mop.keys))
	conn := oper.pool.Get()
	defer conn.Close()
	for _, vec := range slotmap {
		req := make([]interface{}, 0, len(vec))
		for _, p := range vec {
			req = append(req, p.key)
		}

		replys, err := redis.Values(conn.Do("mget", req...))
		if err != nil {
			return nil, errors.Trace(err)
		}

		for i, reply := range replys {
			if reply != nil {
				results[vec[i].pos] = reply
			} else {
				results[vec[i].pos] = nil
			}
		}
	}

	b, err := respcoding.Marshal(results)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return b, nil
}

func (oper *MultiOperator) mget(mop *MulOp) {
	start := time.Now()
	defer func() {
		if sec := time.Since(start).Seconds(); sec > 2 {
			log.Warning("too long to do mget", sec)
		}
	}()

	b, err := oper.mgetResults(mop)
	if err != nil {
		mop.wait <- errors.Trace(err)
		return
	}

	if err := mop.w.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		mop.wait <- errors.Trace(err)
		return
	}

	_, err = mop.w.Write(b)
	mop.wait <- errors.Trace(err)
}

func (oper *MultiOperator) delResults(mop *MulOp) ([]byte, error) {
	var results int64
	conn := oper.pool.Get()
	defer conn.Close()
	for _, k := range mop.keys {
		n, err := conn.Do("del", k)
		if err != nil {
			return nil, errors.Trace(err)
		}
		results += n.(int64)
	}

	b, err := respcoding.Marshal(int(results))
	if err != nil {
		return nil, errors.Trace(err)
	}

	return b, nil
}

func (oper *MultiOperator) del(mop *MulOp) {
	start := time.Now()
	defer func() { //todo:extra function
		if sec := time.Since(start).Seconds(); sec > 2 {
			log.Warning("too long to do del", sec)
		}
	}()

	b, err := oper.delResults(mop)
	if err != nil {
		mop.wait <- errors.Trace(err)
		return
	}

	if err := mop.w.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		mop.wait <- errors.Trace(err)
		return
	}

	_, err = mop.w.Write(b)
	mop.wait <- errors.Trace(err)
}
