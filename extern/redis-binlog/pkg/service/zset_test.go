// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package service

import (
	"math"
	"strconv"
	"testing"
)

func checkzset(t *testing.T, s Session, k string, expect map[string]float64) {
	array := checkbytesarray(t, s, "zgetall", k)
	if expect == nil {
		checkerror(t, nil, array == nil)
	} else {
		checkerror(t, nil, len(array) == len(expect)*2)
		for i := 0; i < len(expect); i++ {
			k := string(array[i*2])
			v := string(array[i*2+1])
			f, err := strconv.ParseFloat(v, 64)
			checkerror(t, err, math.Abs(expect[k]-f) < 1e-9)
		}
	}
}

func TestZAdd(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 1, c, "zadd", k, 1, "one")
	checkint(t, 2, c, "zadd", k, 2, "two", 3, "three")
	checkint(t, 1, c, "zadd", k, 1.5, "one", 4, "four", 5, "four")
	checkzset(t, c, k, map[string]float64{"one": 1.5, "two": 2, "three": 3, "four": 5})
	checkint(t, 0, c, "zadd", k, 1, "one", 4, "four")
	checkzset(t, c, k, map[string]float64{"one": 1, "two": 2, "three": 3, "four": 4})
}

func TestZCard(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 0, c, "zcard", k)
	checkint(t, 1, c, "zadd", k, 1, "one")
	checkint(t, 1, c, "zcard", k)
	checkint(t, 2, c, "zadd", k, 2, "two", 3, "three")
	checkint(t, 3, c, "zcard", k)
	checkint(t, 0, c, "zadd", k, 4, "two")
	checkint(t, 3, c, "zcard", k)
}

func TestZScore(t *testing.T) {
	c := client(t)
	k := random(t)
	checknil(t, c, "zscore", k, "one")
	checkint(t, 1, c, "zadd", k, 1, "one")
	checkfloat(t, 1, c, "zscore", k, "one")
	checknil(t, c, "zscore", k, "two")
}

func TestZRem(t *testing.T) {
	c := client(t)
	k := random(t)
	checkint(t, 3, c, "zadd", k, 1, "key1", 2, "key2", 3, "key3")
	checkint(t, 0, c, "zrem", k, "key")
	checkint(t, 1, c, "zrem", k, "key1")
	checkzset(t, c, k, map[string]float64{"key2": 2, "key3": 3})
	checkint(t, 2, c, "zrem", k, "key1", "key2", "key3")
	checkzset(t, c, k, nil)
	checkint(t, -2, c, "ttl", k)
}

func TestZIncrBy(t *testing.T) {
	c := client(t)
	k := random(t)
	checkfloat(t, 1, c, "zincrby", k, 1, "one")
	checkfloat(t, 1, c, "zincrby", k, 1, "two")
	checkfloat(t, 2, c, "zincrby", k, 1, "two")
	checkzset(t, c, k, map[string]float64{"one": 1, "two": 2})
}
