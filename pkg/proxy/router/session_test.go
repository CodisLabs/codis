package router

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/utils/assert"
)

func TestSessions(t *testing.T) {
	cleanupSessions(time.Now().Unix() + 100000)
	assert.Must(sessions.Len() == 0)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.MustNoError(err)
	defer l.Close()

	const cnt = 8

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < cnt; i++ {
			c, err := l.Accept()
			assert.MustNoError(err)
			NewSession(c)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < cnt; i++ {
			c, err := net.Dial("tcp", l.Addr().String())
			assert.MustNoError(err)
			NewSession(c)
		}
	}()

	wg.Wait()
	assert.Must(sessions.Len() == cnt*2)
	cleanupSessions(time.Now().Unix() + 10)
	assert.Must(sessions.Len() == 0)
}
