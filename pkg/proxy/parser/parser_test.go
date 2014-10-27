package parser

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/juju/errors"
)

func TestBtoi(t *testing.T) {
	tbl := map[string]int{
		"-1": -1,
		"0":  0,
		"1":  1,
	}

	for k, v := range tbl {
		if n, _ := Btoi([]byte(k)); n != v {
			t.Error("value not match", n, v)
		}
	}
}

func TestParserBulk(t *testing.T) {
	sample := "*2\r\n$4\r\nLLEN\r\n$6\r\nmylist\r\n"
	buf := bytes.NewBuffer([]byte(sample))
	r := bufio.NewReader(buf)

	resp, err := Parse(r)
	if err != nil {
		t.Error(errors.ErrorStack(err))
	}
	b, err := resp.Bytes()
	if err != nil {
		t.Error(err)
	}
	if resp == nil {
		t.Error("unknown error")
	}
	if len(b) != len(sample) {
		t.Error("to bytes error", string(b),
			"................", sample)
	}

	op, err := resp.Op()
	if !bytes.Equal(op, []byte("LLEN")) {
		t.Error("get op error")
	}

	key, err := resp.Key()
	if !bytes.Equal(key, []byte("mylist")) {
		t.Error("get key error")
	}
}

func TestKeys(t *testing.T) {
	table := []string{
		"*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
	}

	for _, s := range table {
		buf := bytes.NewBuffer([]byte(s))
		r := bufio.NewReader(buf)

		resp, err := Parse(r)
		if err != nil {
			t.Error(errors.ErrorStack(err))
		}
		_, err = resp.Bytes()
		if err != nil {
			t.Error(err)
		}

		keys, err := resp.Keys()
		if err != nil {
			t.Error(err)
		}

		if len(keys) != 1 || string(keys[0]) != "bar" {
			t.Error("Keys failed", keys)
		}
	}
}

func TestParser(t *testing.T) {
	table := []string{
		"$6\r\nfoobar\r\n",
		"$0\r\n\r\n",
		"$-1\r\n",
		"*0\r\n",
		"*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
		"*3\r\n:1\r\n:2\r\n:3\r\n",
		"*-1\r\n",
		"+OK\r\n",
		"-Error message\r\n",
		"*2\r\n$1\r\n0\r\n*0\r\n",
	}

	for _, s := range table {
		buf := bytes.NewBuffer([]byte(s))
		r := bufio.NewReader(buf)

		resp, err := Parse(r)
		if err != nil {
			t.Error(err)
		}

		_, err = resp.Bytes()
		if err != nil {
			t.Error(err)
		}
	}

	//test invalid input
	buf := bytes.NewBuffer([]byte("*xx$**"))
	r := bufio.NewReader(buf)

	_, err := Parse(r)
	if err == nil {
		t.Error("should return error")
	}
}

func TestParserErrorHandling(t *testing.T) {
	buf := bytes.NewBuffer([]byte("-Error message\r\n"))
	r := bufio.NewReader(buf)

	resp, err := Parse(r)
	if err != nil {
		t.Error("should not return error")
	}

	if resp.Type != ErrorResp {
		t.Error("type not match")
	}

	if len(resp.Error) == 0 {
		t.Error("parse error message failed")
	}
}

func TestParserInvalid(t *testing.T) {
	table := []string{
		"",
		"$6\r\nfoobar\r",
		"$0\rn\r\n",
		"$-1\n",
		"*0",
		"*2n$3\r\nfoo\r\n$3\r\nbar\r\n",
		"3\r\n:1\r\n:2\r\n:3\r\n",
		"*-\r\n",
		"+OK\n",
		"-Error message\r",
	}

	for _, s := range table {
		//test invalid input
		buf := bytes.NewBuffer([]byte(s))
		r := bufio.NewReader(buf)

		_, err := Parse(r)
		if err == nil {
			t.Error("should return error", s)
		}
	}
}
