// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import "testing"

func TestPing(t *testing.T) {
	c := client(t)
	checkstring(t, "PONG", c, "ping")
}

func TestEcho(t *testing.T) {
	c := client(t)
	checkstring(t, "HELLO", c, "echo", "HELLO")
}

func TestFlushAll(t *testing.T) {
	c := client(t)
	k := random(t)
	checknil(t, c, "get", k)
	checkint(t, 5, c, "append", k, "hello")
	checkint(t, 11, c, "append", k, " world")
	checkstring(t, "hello world", c, "get", k)
	checkok(t, c, "flushall")
	checknil(t, c, "get", k)
}
