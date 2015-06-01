// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"bufio"
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/wandoulabs/codis/pkg/proxy/parser"

	"github.com/juju/errors"
	stats "github.com/ngaut/gostats"
)

const (
	simple_request = "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$8\r\nmy value\r\n"
)

func TestStringsContain(t *testing.T) {
	s := []string{"abc", "bcd", "ab"}
	if StringsContain(s, "a") {
		t.Error("should not found")
	}

	if !StringsContain(s, "ab") {
		t.Error("shoud found")
	}
}

func TestAllowOp(t *testing.T) {
	if allowOp("SLOTSMGRT") || allowOp("SLOTSMGRTONE") {
		t.Error("should not allowed")
	}

	if !allowOp("SET") {
		t.Error("should be allowed")
	}
}

func TestIsMulOp(t *testing.T) {
	if isMulOp("GET") {
		t.Error("is not mulOp")
	}

	if !isMulOp("MGET") || !isMulOp("DEL") || !isMulOp("MSET") {
		t.Error("should be mulOp")
	}
}

func TestRecordResponseTime(t *testing.T) {
	c := stats.NewCounters("test")
	recordResponseTime(c, 1)
	recordResponseTime(c, 5)
	recordResponseTime(c, 10)
	recordResponseTime(c, 50)
	recordResponseTime(c, 200)
	recordResponseTime(c, 1000)
	recordResponseTime(c, 5000)
	recordResponseTime(c, 8000)
	recordResponseTime(c, 10000)
	cnts := c.Counts()
	if cnts["0-5ms"] != 1 {
		t.Fail()
	}
	if cnts["5-10ms"] != 1 {
		t.Fail()
	}
	if cnts["50-200ms"] != 1 {
		t.Fail()
	}
	if cnts["200-1000ms"] != 1 {
		t.Fail()
	}
	if cnts["1000-5000ms"] != 1 {
		t.Fail()
	}
	if cnts["5000-10000ms"] != 2 {
		t.Fail()
	}
	if cnts["10000ms+"] != 1 {
		t.Fail()
	}
}

func TestValidSlot(t *testing.T) {
	if validSlot(-1) {
		t.Error("should be invalid")
	}

	if validSlot(1024) {
		t.Error("should be invalid")
	}

	if !validSlot(0) {
		t.Error("should be valid")
	}
}

type fakeDeadlineReadWriter struct {
	r *bufio.Reader
	w *bufio.Writer
}

func (rw *fakeDeadlineReadWriter) BufioReader() *bufio.Reader {
	return rw.r
}

func (rw *fakeDeadlineReadWriter) SetReadDeadline(t time.Time) error {
	return nil
}

func (rw *fakeDeadlineReadWriter) SetWriteDeadline(t time.Time) error {
	return nil
}

func (rw *fakeDeadlineReadWriter) Read(p []byte) (int, error) {
	return rw.r.Read(p)
}

func (rw *fakeDeadlineReadWriter) Write(p []byte) (int, error) {
	return rw.w.Write(p)
}

func TestForward(t *testing.T) {
	client := &fakeDeadlineReadWriter{r: bufio.NewReader(bytes.NewBuffer([]byte(simple_request))),
		w: bufio.NewWriter(&bytes.Buffer{})}
	redis := &fakeDeadlineReadWriter{r: bufio.NewReader(bytes.NewBuffer([]byte(simple_request))),
		w: bufio.NewWriter(&bytes.Buffer{})}

	resp, err := parser.Parse(bufio.NewReader(bytes.NewBuffer([]byte(simple_request))))
	if err != nil {
		t.Error(err)
	}

	_, clientErr := forward(client, redis, resp, 5)
	if clientErr != nil {
		t.Error(clientErr)
	}
}

func TestWrite2Client(t *testing.T) {
	var result bytes.Buffer
	var input bytes.Buffer
	_, clientErr := write2Client(bufio.NewReader(&input), &result)
	if clientErr == nil {
		t.Error("should be error")
	}

	input.WriteString(simple_request)
	_, clientErr = write2Client(bufio.NewReader(&input), &result)
	if clientErr != nil {
		t.Error(clientErr)
	}

	if string(result.Bytes()) != simple_request {
		t.Error("not match")
	}
}

func TestWrite2Redis(t *testing.T) {
	var result bytes.Buffer
	var input bytes.Buffer
	input.WriteString(simple_request)
	resp, err := parser.Parse(bufio.NewReader(&input))
	if err != nil {
		t.Error(err)
	}

	err = write2Redis(resp, &result)
	if err != nil {
		t.Error(err)
	}

	if string(result.Bytes()) != simple_request {
		t.Error("not match")
	}
}

func TestGetOrginError(t *testing.T) {
	err := errors.Trace(io.EOF)
	if GetOriginError(errors.Trace(err).(*errors.Err)).Error() != io.EOF.Error() {
		t.Error("should be io.EOF")
	}
}

func TestHandleSpecCommand(t *testing.T) {
	var tbl = map[string]string{
		"PING":   "+PONG\r\n",
		"QUIT":   string(OK_BYTES),
		"SELECT": string(OK_BYTES),
		"AUTH":   string(OK_BYTES),
	}

	for k, v := range tbl {
		resp, err := parser.Parse(bufio.NewReader(bytes.NewBufferString(k + string(parser.NEW_LINE))))
		if err != nil {
			t.Error(err)
		}

		_, keys, err := resp.GetOpKeys()
		if err != nil {
			t.Error(errors.ErrorStack(err))
		}

		result, _, _, err := handleSpecCommand(k, keys, 5)
		if err != nil {
			t.Error(err)
		}

		if string(result) != v {
			t.Error("result not match", string(result))
		}
	}

	//"ECHO xxxx": "xxxx\r\n",
	{
		resp, err := parser.Parse(bufio.NewReader(bytes.NewBufferString("ECHO xxxx\r\n")))
		if err != nil {
			t.Error(errors.ErrorStack(err))
		}

		_, keys, _ := resp.GetOpKeys()

		result, _, _, err := handleSpecCommand("ECHO", keys, 5)
		if err != nil {
			t.Error(errors.ErrorStack(err))
		}

		if string(result) != "$4\r\nxxxx\r\n" {
			t.Error("result not match", string(result))
		}
	}

	//test empty key
	{
		resp, err := parser.Parse(bufio.NewReader(bytes.NewBufferString("ECHO\r\n")))
		if err != nil {
			t.Error(errors.ErrorStack(err))
		}

		_, keys, _ := resp.GetOpKeys()
		_, shouldClose, _, err := handleSpecCommand("ECHO", keys, 5)
		if !shouldClose {
			t.Error(errors.ErrorStack(err))
		}
	}

	//test not specific command
	{
		_, _, handled, err := handleSpecCommand("get", nil, 5)
		if handled {
			t.Error(errors.ErrorStack(err))
		}
	}
}
