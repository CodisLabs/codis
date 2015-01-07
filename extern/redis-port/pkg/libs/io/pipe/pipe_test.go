// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package pipe

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/io/ioutils"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/tests"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/utils/bytesize"
)

func openPipe(t *testing.T, fileName string) (*PipeReader, *PipeWriter) {
	buffSize := bytesize.KB * 8
	fileSize := bytesize.MB * 32
	if fileName == "" {
		return PipeWithSize(buffSize)
	} else {
		f, err := OpenFile(fileName, false)
		tests.AssertNoError(t, err)
		return PipeWithFile(buffSize, fileSize, f)
	}
}

func testPipe1(t *testing.T, fileName string) {
	r, w := openPipe(t, fileName)

	s := "Hello world!!"

	go func(data []byte) {
		_, err := ioutils.WriteFull(w, data)
		tests.AssertNoError(t, err)
		tests.AssertNoError(t, w.Close())
	}([]byte(s))

	buf := make([]byte, 64)
	n, err := ioutils.ReadFull(r, buf)
	tests.Assert(t, errors.Equal(err, io.EOF))
	tests.Assert(t, n == len(s))
	tests.Assert(t, string(buf[:n]) == s)
	tests.AssertNoError(t, r.Close())
}

func TestPipe1(t *testing.T) {
	testPipe1(t, "")
	testPipe1(t, "/tmp/pipe.test")
}

func testPipe2(t *testing.T, fileName string) {
	r, w := openPipe(t, fileName)

	c := 1024 * 128
	s := "Hello world!!"

	go func() {
		for i := 0; i < c; i++ {
			m := fmt.Sprintf("[%d]%s ", i, s)
			_, err := ioutils.WriteFull(w, []byte(m))
			tests.AssertNoError(t, err)
		}
		tests.AssertNoError(t, w.Close())
	}()

	time.Sleep(time.Millisecond * 100)

	buf := make([]byte, len(s)*c*2)
	n, err := ioutils.ReadFull(r, buf)
	tests.Assert(t, errors.Equal(err, io.EOF))
	buf = buf[:n]
	for i := 0; i < c; i++ {
		m := fmt.Sprintf("[%d]%s ", i, s)
		tests.Assert(t, len(buf) >= len(m))
		tests.Assert(t, string(buf[:len(m)]) == m)
		buf = buf[len(m):]
	}
	tests.Assert(t, len(buf) == 0)
	tests.AssertNoError(t, r.Close())
}

func TestPipe2(t *testing.T) {
	testPipe2(t, "")
	testPipe2(t, "/tmp/pipe.test")
}

func testPipe3(t *testing.T, fileName string) {
	r, w := openPipe(t, fileName)

	c := make(chan int)

	size := 4096

	go func() {
		buf := make([]byte, size)
		for {
			n, err := r.Read(buf)
			if errors.Equal(err, io.EOF) {
				break
			}
			tests.AssertNoError(t, err)
			c <- n
		}
		tests.AssertNoError(t, r.Close())
		c <- 0
	}()

	go func() {
		buf := make([]byte, size)
		for i := 1; i < size; i++ {
			n, err := ioutils.WriteFull(w, buf[:i])
			tests.AssertNoError(t, err)
			tests.Assert(t, n == i)
		}
		tests.AssertNoError(t, w.Close())
	}()

	sum := 0
	for i := 1; i < size; i++ {
		sum += i
	}
	for {
		n := <-c
		if n == 0 {
			break
		}
		sum -= n
	}
	tests.Assert(t, sum == 0)
}

func TestPipe3(t *testing.T) {
	testPipe3(t, "")
	testPipe3(t, "/tmp/pipe.test")
}

func testPipe4(t *testing.T, fileName string) {
	r, w := openPipe(t, fileName)

	key := []byte("wandoujia aes128")

	block := aes.BlockSize
	count := 1024 * 1024 * 128 / block

	go func() {
		buf := make([]byte, count*block)
		m, err := aes.NewCipher(key)
		tests.AssertNoError(t, err)
		for i := 0; i < len(buf); i++ {
			buf[i] = byte(i)
		}

		e := cipher.NewCBCEncrypter(m, make([]byte, block))
		e.CryptBlocks(buf, buf)

		_, err = ioutils.WriteFull(w, buf)
		tests.AssertNoError(t, err)
		tests.AssertNoError(t, w.Close())
	}()

	buf := make([]byte, count*block)
	m, err := aes.NewCipher(key)
	tests.AssertNoError(t, err)

	_, err = ioutils.ReadFull(r, buf)
	tests.AssertNoError(t, err)

	e := cipher.NewCBCDecrypter(m, make([]byte, block))
	e.CryptBlocks(buf, buf)

	for i := 0; i < len(buf); i++ {
		tests.Assert(t, buf[i] == byte(i))
	}
	_, err = ioutils.ReadFull(r, buf)
	tests.Assert(t, errors.Equal(err, io.EOF))
	tests.AssertNoError(t, r.Close())
}

func TestPipe4(t *testing.T) {
	testPipe4(t, "")
	testPipe4(t, "/tmp/pipe.test")
}

type pipeTest struct {
	async   bool
	err     error
	witherr bool
}

func (p pipeTest) String() string {
	return fmt.Sprintf("async=%v err=%v witherr=%v", p.async, p.err, p.witherr)
}

var pipeTests = []pipeTest{
	{true, nil, false},
	{true, nil, true},
	{true, io.ErrShortWrite, true},
	{false, nil, false},
	{false, nil, true},
	{false, io.ErrShortWrite, true},
}

func delayClose(t *testing.T, closer PipeCloser, c chan int, u pipeTest) {
	time.Sleep(time.Millisecond * 100)
	var err error
	if u.witherr {
		err = closer.CloseWithError(u.err)
	} else {
		err = closer.Close()
	}
	tests.AssertNoError(t, err)
	c <- 0
}

func TestPipeReadClose(t *testing.T) {
	for _, u := range pipeTests {
		r, w := Pipe()
		c := make(chan int, 1)

		if u.async {
			go delayClose(t, w, c, u)
		} else {
			delayClose(t, w, c, u)
		}

		buf := make([]byte, 64)
		n, err := r.Read(buf)
		<-c

		expect := u.err
		if expect == nil {
			expect = io.EOF
		}
		tests.Assert(t, errors.Equal(err, expect))
		tests.Assert(t, n == 0)
		tests.AssertNoError(t, r.Close())
	}
}

func TestPipeReadClose2(t *testing.T) {
	r, w := Pipe()
	c := make(chan int, 1)

	go delayClose(t, r, c, pipeTest{})

	n, err := r.Read(make([]byte, 64))
	<-c

	tests.Assert(t, errors.Equal(err, io.ErrClosedPipe))
	tests.Assert(t, n == 0)
	tests.AssertNoError(t, w.Close())
}

func TestPipeWriteClose(t *testing.T) {
	for _, u := range pipeTests {
		r, w := Pipe()
		c := make(chan int, 1)

		if u.async {
			go delayClose(t, r, c, u)
		} else {
			delayClose(t, r, c, u)
		}
		<-c

		n, err := ioutils.WriteFull(w, []byte("hello, world"))
		expect := u.err
		if expect == nil {
			expect = io.ErrClosedPipe
		}
		tests.Assert(t, errors.Equal(err, expect))
		tests.Assert(t, n == 0)
		tests.AssertNoError(t, w.Close())
	}
}

func TestWriteEmpty(t *testing.T) {
	r, w := Pipe()

	go func() {
		_, err := w.Write([]byte{})
		tests.AssertNoError(t, err)
		tests.AssertNoError(t, w.Close())
	}()

	buf := make([]byte, 4096)
	n, err := ioutils.ReadFull(r, buf)
	tests.Assert(t, errors.Equal(err, io.EOF))
	tests.Assert(t, n == 0)
	tests.AssertNoError(t, r.Close())
}

func TestWriteNil(t *testing.T) {
	r, w := Pipe()

	go func() {
		_, err := w.Write(nil)
		tests.AssertNoError(t, err)
		tests.AssertNoError(t, w.Close())
	}()

	buf := make([]byte, 4096)
	n, err := ioutils.ReadFull(r, buf)
	tests.Assert(t, errors.Equal(err, io.EOF))
	tests.Assert(t, n == 0)
	tests.AssertNoError(t, r.Close())
}

func TestWriteAfterWriterClose(t *testing.T) {
	r, w := Pipe()

	s := "hello"

	errs := make(chan error)

	go func() {
		_, err := ioutils.WriteFull(w, []byte(s))
		tests.AssertNoError(t, err)
		tests.AssertNoError(t, w.Close())
		_, err = w.Write([]byte("world"))
		errs <- err
	}()

	buf := make([]byte, 4096)
	n, err := ioutils.ReadFull(r, buf)
	tests.Assert(t, errors.Equal(err, io.EOF))
	tests.Assert(t, string(buf[:n]) == s)

	err = <-errs
	tests.Assert(t, errors.Equal(err, io.ErrClosedPipe))
	tests.AssertNoError(t, r.Close())
}
