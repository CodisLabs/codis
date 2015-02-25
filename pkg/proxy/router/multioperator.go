// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

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
	op     string
	keys   [][]byte
	result *[]byte
	wait   chan error
}

func NewMultiOperator(server string) *MultiOperator {
	oper := &MultiOperator{q: make(chan *MulOp, 128)}
	oper.pool = newPool(server, "")
	for i := 0; i < 64; i++ {
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

func (oper *MultiOperator) handleMultiOp(op string, keys [][]byte, result *[]byte) error {
	wait := make(chan error, 1)
	oper.q <- &MulOp{op: op, keys: keys, result: result, wait: wait}
	return <-wait
}

func (oper *MultiOperator) work() {
	for mop := range oper.q {
		switch mop.op {
		case "MGET":
			oper.mget(mop)
		case "MSET":
			oper.mset(mop)
		case "DEL":
			oper.del(mop)
		}
	}
}

func (oper *MultiOperator) mgetResults(mop *MulOp) ([]byte, error) {
	results := make([]interface{}, len(mop.keys))
	conn := oper.pool.Get()
	defer conn.Close()
	for i, key := range mop.keys {
		replys, err := redis.Values(conn.Do("mget", key))
		if err != nil {
			return nil, errors.Trace(err)
		}

		for _, reply := range replys {
			if reply != nil {
				results[i] = reply
			} else {
				results[i] = nil
			}
		}
	}

	b, err := respcoding.Marshal(results)
	return b, errors.Trace(err)
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
	*mop.result = b
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
	return b, errors.Trace(err)
}

func (oper *MultiOperator) msetResults(mop *MulOp) ([]byte, error) {
	conn := oper.pool.Get()
	defer conn.Close()
	for i := 0; i < len(mop.keys); i += 2 {
		log.Info(string(mop.keys[i]), string(mop.keys[i+1]))
		_, err := conn.Do("set", mop.keys[i], mop.keys[i+1]) //change mset to set
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	return OK_BYTES, nil
}

func (oper *MultiOperator) mset(mop *MulOp) {
	start := time.Now()
	defer func() { //todo:extra function
		if sec := time.Since(start).Seconds(); sec > 2 {
			log.Warning("too long to do del", sec)
		}
	}()

	b, err := oper.msetResults(mop)
	if err != nil {
		mop.wait <- errors.Trace(err)
		return
	}

	*mop.result = b
	mop.wait <- errors.Trace(err)
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

	*mop.result = b
	mop.wait <- errors.Trace(err)
}
