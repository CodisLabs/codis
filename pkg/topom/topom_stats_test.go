// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"container/list"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/assert"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

func TestProxyStats(x *testing.T) {
	t := openTopom()
	defer t.Close()

	check := func(succ, fail []string) {
		w, err := t.RefreshProxyStats(time.Second * 5)
		assert.MustNoError(err)
		m := w.Wait()
		assert.Must(len(m) == len(succ)+len(fail))
		for _, t := range succ {
			s, ok := m[t].(*ProxyStats)
			assert.Must(ok && s != nil && s.Stats != nil)
		}
		for _, t := range fail {
			s, ok := m[t].(*ProxyStats)
			assert.Must(ok && s != nil && s.Stats == nil)
			assert.Must(s.Error != nil)
		}
	}

	p1, c1 := openProxy()
	defer c1.Shutdown()

	p2, c2 := openProxy()
	defer c2.Shutdown()

	contextCreateProxy(t, p1)
	check([]string{p1.Token}, []string{})

	contextCreateProxy(t, p2)
	check([]string{p1.Token, p2.Token}, []string{})

	assert.MustNoError(c1.Shutdown())
	check([]string{p2.Token}, []string{p1.Token})

	assert.MustNoError(c2.Shutdown())
	check([]string{}, []string{p1.Token, p2.Token})

	p3, c3 := openProxy()
	defer c3.Shutdown()

	contextCreateProxy(t, p3)
	check([]string{p3.Token}, []string{p1.Token, p2.Token})

	contextRemoveProxy(t, p1)
	check([]string{p3.Token}, []string{p2.Token})
}

func TestRedisStats(x *testing.T) {
	t := openTopom()
	defer t.Close()

	check := func(succ, fail []string) {
		w, err := t.RefreshRedisStats(time.Second * 5)
		assert.MustNoError(err)
		m := w.Wait()
		assert.Must(len(m) == len(succ)+len(fail))
		for _, addr := range succ {
			s, ok := m[addr].(*RedisStats)
			assert.Must(ok && s != nil && s.Stats != nil)
		}
		for _, addr := range fail {
			s, ok := m[addr].(*RedisStats)
			assert.Must(ok && s != nil && s.Stats == nil)
			assert.Must(s.Error != nil)
		}
	}

	g := &models.Group{Id: 1}

	s1 := newFakeServer()
	defer s1.Close()

	s2 := newFakeServer()
	defer s2.Close()

	g.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: s1.Addr},
		&models.GroupServer{Addr: s2.Addr},
	}

	check([]string{}, []string{})

	contextCreateGroup(t, g)
	check([]string{s1.Addr, s2.Addr}, []string{})

	s1.Close()
	check([]string{s2.Addr}, []string{s1.Addr})

	s2.Close()
	check([]string{}, []string{s1.Addr, s2.Addr})

	s3 := newFakeServer()
	defer s3.Close()

	g.Servers = []*models.GroupServer{
		&models.GroupServer{Addr: s3.Addr},
	}
	contextUpdateGroup(t, g)
	check([]string{s3.Addr}, []string{})

	contextRemoveGroup(t, g)
	check([]string{}, []string{})
}

type fakeServer struct {
	net.Listener
	list.List
	Addr string
}

func newFakeServer() *fakeServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	f := &fakeServer{Listener: l, Addr: l.Addr().String()}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			f.PushBack(c)
			go f.Serve(c)
		}
	}()
	return f
}

func (s *fakeServer) Close() error {
	for e := s.List.Front(); e != nil; e = e.Next() {
		e.Value.(net.Conn).Close()
	}
	return s.Listener.Close()
}

func (s *fakeServer) Serve(c net.Conn) {
	defer c.Close()
	dec := redis.NewDecoder(c)
	enc := redis.NewEncoder(c)
	var multi int
	for {
		r, err := dec.Decode()
		if err != nil {
			return
		}
		assert.Must(r.Type == redis.TypeArray && len(r.Array) != 0)
		var resp *redis.Resp
		switch cmd := string(r.Array[0].Value); cmd {
		case "SLOTSINFO":
			resp = redis.NewArray([]*redis.Resp{})
		case "AUTH":
			resp = redis.NewBulkBytes([]byte("OK"))
		case "INFO":
			resp = redis.NewBulkBytes([]byte("#Fake Codis Server"))
		case "MULTI":
			assert.Must(multi == 0)
			multi++
			continue
		case "SLAVEOF", "CLIENT":
			assert.Must(multi != 0)
			multi++
			continue
		case "EXEC":
			assert.Must(multi != 0)
			resp = redis.NewArray([]*redis.Resp{})
			for i := 1; i < multi; i++ {
				resp.Array = append(resp.Array, redis.NewBulkBytes([]byte("OK")))
			}
			multi = 0
		case "CONFIG":
			if multi != 0 {
				multi++
				continue
			}
			assert.Must(len(r.Array) >= 2)
			var sub = strings.ToUpper(string(r.Array[1].Value))
			var key string
			if len(r.Array) >= 3 {
				key = string(r.Array[2].Value)
			}
			switch {
			case sub == "GET" && key == "maxmemory":
				assert.Must(len(r.Array) == 3)
				resp = redis.NewArray([]*redis.Resp{
					redis.NewBulkBytes([]byte("maxmemory")),
					redis.NewInt([]byte("0")),
				})
			default:
				log.Panicf("unknown subcommand of <%s>", cmd)
			}
		case "SLOTSMGRTTAGSLOT":
			resp = redis.NewArray([]*redis.Resp{
				redis.NewInt([]byte("0")),
				redis.NewInt([]byte("0")),
			})
		default:
			log.Panicf("unknown command <%s>", cmd)
		}
		assert.MustNoError(enc.Encode(resp, true))
	}
}
