package topom

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/assert"
	"github.com/wandoulabs/codis/pkg/utils/log"
)

func TestFakeServer(x *testing.T) {
	s := newFakeServer()
	defer s.Close()

	p := NewRedisPool("foobar", time.Second*10)
	defer p.Close()

	c, err := p.GetClient(s.Addr())
	assert.MustNoError(err)
	defer p.PutClient(c)

	assert.MustNoError(c.SetMaster(""))
	assert.MustNoError(c.SetMaster("NO:ONE"))

	_, err = c.Info()
	assert.MustNoError(err)

	_, err = c.SlotsInfo()
	assert.MustNoError(err)

	_, err = c.MigrateSlot(0, s.Addr())
	assert.MustNoError(err)
}

type fakeServer struct {
	l net.Listener
}

func newFakeServer() *fakeServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	f := &fakeServer{l}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go f.Serve(c)
		}
	}()
	return f
}

func (s *fakeServer) Addr() string {
	return s.l.Addr().String()
}

func (s *fakeServer) Close() error {
	return s.l.Close()
}

func (s *fakeServer) Serve(c net.Conn) {
	defer c.Close()
	dec := redis.NewDecoder(bufio.NewReader(c))
	enc := redis.NewEncoder(bufio.NewWriter(c))
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
		case "AUTH", "SLAVEOF":
			resp = redis.NewBulkBytes([]byte("OK"))
		case "INFO":
			resp = redis.NewBulkBytes([]byte("#Fake Codis Server"))
		case "CONFIG":
			assert.Must(len(r.Array) == 3)
			sub := string(r.Array[1].Value)
			key := string(r.Array[2].Value)
			switch {
			case sub == "GET" && key == "maxmemory":
				resp = redis.NewArray([]*redis.Resp{
					redis.NewBulkBytes([]byte("maxmemory")),
					redis.NewInt([]byte("0")),
				})
			case sub == "SET" && key == "masterauth":
				resp = redis.NewBulkBytes([]byte("OK"))
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
