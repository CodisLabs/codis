// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func TestRequestChan1(t *testing.T) {
	var ch = NewRequestChanBuffer(512)
	for i := 0; i < 8192; i++ {
		n := ch.PushBack(&Request{UnixNano: int64(i)})
		assert.Must(n == i+1)
	}
	for i := 0; i < 8192; i++ {
		r, ok := ch.PopFront()
		assert.Must(ok && r.UnixNano == int64(i))
	}
	assert.Must(ch.Len() == 0)

	ch.Close()

	_, ok := ch.PopFront()
	assert.Must(!ok)
}

func TestRequestChan2(t *testing.T) {
	var ch = NewRequestChanBuffer(512)
	for i := 0; i < 8192; i++ {
		n := ch.PushBack(&Request{UnixNano: int64(i)})
		assert.Must(n == i+1)
	}
	ch.Close()

	assert.Must(ch.Len() == 8192)

	for i := 0; i < 8192; i++ {
		r, ok := ch.PopFront()
		assert.Must(ok && r.UnixNano == int64(i))
	}
	assert.Must(ch.Len() == 0)

	_, ok := ch.PopFront()
	assert.Must(!ok)
}

func TestRequestChan3(t *testing.T) {
	var wg sync.WaitGroup
	var ch = NewRequestChanBuffer(512)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8192; i++ {
			ch.PushBack(&Request{UnixNano: int64(i)})
			if i%1024 == 0 {
				runtime.Gosched()
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 8192; i++ {
			r, ok := ch.PopFront()
			assert.Must(ok && r.UnixNano == int64(i))
		}
	}()

	wg.Wait()

	go func() {
		defer ch.Close()
		time.Sleep(time.Millisecond * 10)
	}()

	_, ok := ch.PopFront()
	assert.Must(!ok)
}

func BenchmarkRequestGoChannel(b *testing.B) {
	var request = &Request{
		Multi: make([]*redis.Resp, 1024*1024),
	}
	var ch = make(chan *Request, 1024)
	go func() {
		for i := 0; i < b.N; i++ {
			ch <- request
		}
	}()

	for i := 0; i < b.N; i++ {
		<-ch
	}
}

func benchmarkRequestChanN(b *testing.B, n int) {
	var request = &Request{
		Multi: make([]*redis.Resp, 1024*1024),
	}
	var ch = NewRequestChanBuffer(n)
	go func() {
		for i := 0; i < b.N; i++ {
			ch.PushBack(request)
			if i%1024 == 0 {
				runtime.Gosched()
			}
		}
	}()

	for i := 0; i < b.N; i++ {
		ch.PopFront()
	}
}

func BenchmarkRequestChan128(b *testing.B)  { benchmarkRequestChanN(b, 128) }
func BenchmarkRequestChan256(b *testing.B)  { benchmarkRequestChanN(b, 256) }
func BenchmarkRequestChan512(b *testing.B)  { benchmarkRequestChanN(b, 512) }
func BenchmarkRequestChan1024(b *testing.B) { benchmarkRequestChanN(b, 1024) }
func BenchmarkRequestChan2048(b *testing.B) { benchmarkRequestChanN(b, 2048) }
