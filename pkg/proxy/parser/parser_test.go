// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

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

	op, keys, err := resp.GetOpKeys()
	if !bytes.Equal(op, []byte("LLEN")) {
		t.Errorf("get op error, got %s, expect LLEN", string(op))
	}

	if !bytes.Equal(keys[0], []byte("mylist")) {
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
		b, err := resp.Bytes()
		if err != nil {
			t.Error(err)
		}

		if s != string(b) {
			t.Fatalf("not match, expect %s, got %s", s, string(b))
		}

		_, keys, err := resp.GetOpKeys()
		if err != nil {
			t.Error(err)
		}

		if len(keys) != 1 || string(keys[0]) != "bar" {
			t.Error("Keys failed", keys)
		}
	}
}

func TestMulOpKeys(t *testing.T) {
	table := []string{
		"*7\r\n$4\r\nmset\r\n$4\r\nkey1\r\n$6\r\nvalue1\r\n$4\r\nkey2\r\n$6\r\nvalue2\r\n$4\r\nkey3\r\n$0\r\n\r\n",
	}

	for _, s := range table {
		buf := bytes.NewBuffer([]byte(s))
		r := bufio.NewReader(buf)

		resp, err := Parse(r)
		if err != nil {
			t.Error(errors.ErrorStack(err))
		}
		b, err := resp.Bytes()
		if err != nil {
			t.Error(err)
		}

		if s != string(b) {
			t.Fatalf("not match, expect %s, got %s", s, string(b))
		}

		_, keys, err := resp.GetOpKeys()
		if err != nil {
			t.Error(err)
		}

		if len(keys) != 6 || string(keys[5]) != "" {
			t.Error("Keys failed", string(keys[5]))
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
		"*3\r\n$4\r\nEVAL\r\n$31\r\nreturn {1,2,{3,'Hello World!'}}\r\n$1\r\n0\r\n",
		"mget a b c\r\n",
	}

	for _, s := range table {
		buf := bytes.NewBuffer([]byte(s))
		r := bufio.NewReader(buf)

		resp, err := Parse(r)
		if err != nil {
			t.Fatal(errors.ErrorStack(err))
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

func TestEval(t *testing.T) {
	table := []string{
		"*3\r\n$4\r\nEVAL\r\n$31\r\nreturn {1,2,{3,'Hello World!'}}\r\n$1\r\n0\r\n",
	}

	for _, s := range table {
		buf := bytes.NewBuffer([]byte(s))
		r := bufio.NewReader(buf)

		resp, err := Parse(r)
		if err != nil {
			t.Fatal(errors.ErrorStack(err))
		}
		op, keys, err := resp.GetOpKeys()
		if err != nil {
			t.Fatal(err)
		}

		if string(op) != "EVAL" {
			t.Fatalf("op not match, expect %s, got %s", "EVAL", string(op))
		}

		if len(resp.Multi) != 3 {
			t.Fatal("argument count not match")
		}

		if len(keys) != 1 {
			t.Fatalf("key count not match, expect %d got %d", 1, len(keys))
		}

		_, err = resp.Bytes()
		if err != nil {
			t.Error(err)
		}
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

	if len(raw2Error(resp)) == 0 {
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
