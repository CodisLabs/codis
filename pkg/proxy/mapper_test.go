// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"testing"

	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/assert"
)

func TestGetOpStr(t *testing.T) {
	var m = map[string]string{
		"get":     "GET",
		"aBc":     "ABC",
		"おはよ":     "おはよ",
		"ni hao!": "NI HAO!",
		"":        "",
	}
	for k, v := range m {
		var multi = []*redis.Resp{redis.NewBulkBytes([]byte(k))}
		s, _, err := getOpInfo(multi)
		if v != "" {
			assert.MustNoError(err)
			assert.Must(s == v)
		} else {
			assert.Must(err != nil)
		}
	}
}

func TestGetOpStrCmd(t *testing.T) {
	var m = map[string]string{
		"del":              "DEL",
		"dump":             "DUMP",
		"exists":           "EXISTS",
		"expire":           "EXPIRE",
		"expireat":         "EXPIREAT",
		"persist":          "PERSIST",
		"pexpire":          "PEXPIRE",
		"pexpireat":        "PEXPIREAT",
		"pttl":             "PTTL",
		"restore":          "RESTORE",
		"sort":             "SORT",
		"ttl":              "TTL",
		"type":             "TYPE",
		"append":           "APPEND",
		"bitcount":         "BITCOUNT",
		"decr":             "DECR",
		"decrby":           "DECRBY",
		"get":              "GET",
		"getbit":           "GETBIT",
		"getrange":         "GETRANGE",
		"getset":           "GETSET",
		"incr":             "INCR",
		"incrby":           "INCRBY",
		"incrbyfloat":      "INCRBYFLOAT",
		"mget":             "MGET",
		"mset":             "MSET",
		"psetex":           "PSETEX",
		"set":              "SET",
		"setbit":           "SETBIT",
		"setex":            "SETEX",
		"setnx":            "SETNX",
		"setrange":         "SETRANGE",
		"strlen":           "STRLEN",
		"hdel":             "HDEL",
		"hexists":          "HEXISTS",
		"hget":             "HGET",
		"hgetall":          "HGETALL",
		"hincrby":          "HINCRBY",
		"hincrbyfloat":     "HINCRBYFLOAT",
		"hkeys":            "HKEYS",
		"hlen":             "HLEN",
		"hmget":            "HMGET",
		"hmset":            "HMSET",
		"hset":             "HSET",
		"hsetnx":           "HSETNX",
		"hvals":            "HVALS",
		"hscan":            "HSCAN",
		"lindex":           "LINDEX",
		"linsert":          "LINSERT",
		"llen":             "LLEN",
		"lpop":             "LPOP",
		"lpush":            "LPUSH",
		"lpushx":           "LPUSHX",
		"lrange":           "LRANGE",
		"lrem":             "LREM",
		"lset":             "LSET",
		"ltrim":            "LTRIM",
		"rpop":             "RPOP",
		"rpoplpush":        "RPOPLPUSH",
		"rpush":            "RPUSH",
		"rpushx":           "RPUSHX",
		"sadd":             "SADD",
		"scard":            "SCARD",
		"sdiff":            "SDIFF",
		"sdiffstore":       "SDIFFSTORE",
		"sinter":           "SINTER",
		"sinterstore":      "SINTERSTORE",
		"sismember":        "SISMEMBER",
		"smembers":         "SMEMBERS",
		"smove":            "SMOVE",
		"spop":             "SPOP",
		"srandmember":      "SRANDMEMBER",
		"srem":             "SREM",
		"sunion":           "SUNION",
		"sunionstore":      "SUNIONSTORE",
		"sscan":            "SSCAN",
		"zadd":             "ZADD",
		"zcard":            "ZCARD",
		"zcount":           "ZCOUNT",
		"zincrby":          "ZINCRBY",
		"zinterstore":      "ZINTERSTORE",
		"zlexcount":        "ZLEXCOUNT",
		"zrange":           "ZRANGE",
		"zrangebylex":      "ZRANGEBYLEX",
		"zrangebyscore":    "ZRANGEBYSCORE",
		"zrank":            "ZRANK",
		"zrem":             "ZREM",
		"zremrangebylex":   "ZREMRANGEBYLEX",
		"zremrangebyrank":  "ZREMRANGEBYRANK",
		"zremrangebyscore": "ZREMRANGEBYSCORE",
		"zrevrange":        "ZREVRANGE",
		"zrevrangebyscore": "ZREVRANGEBYSCORE",
		"zrevrank":         "ZREVRANK",
		"zscore":           "ZSCORE",
		"zunionstore":      "ZUNIONSTORE",
		"zscan":            "ZSCAN",
		"pfadd":            "PFADD",
		"pfcount":          "PFCOUNT",
		"pfmerge":          "PFMERGE",
		"eval":             "EVAL",
		"evalsha":          "EVALSHA",
	}
	for k, v := range m {
		var multi = []*redis.Resp{redis.NewBulkBytes([]byte(k))}
		s, _, err := getOpInfo(multi)
		assert.MustNoError(err)
		assert.Must(s == v)
	}
}

func TestHashSlot(t *testing.T) {
	var m = map[string]string{
		"{abc}":           "abc",
		"{{{abc1}abc2}":   "{{abc1",
		"abc1{abc2{abc3}": "abc2{abc3",
		"{{{{abc":         "{{{{abc",
		"{{{{abc}":        "{{{abc",
		"{{}{{abc":        "{",
		"abc}{abc":        "abc}{abc",
		"abc}{123}456":    "123",
		"123{abc}456":     "abc",
		"{}abc":           "",
		"abc{}123":        "",
		"123{456}":        "456",
	}
	for k, v := range m {
		i := Hash([]byte(k))
		j := Hash([]byte(v))
		assert.Must(i == j)
	}
}
