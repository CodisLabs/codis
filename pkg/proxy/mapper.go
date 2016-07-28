// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"bytes"
	"hash/crc32"
	"strings"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/errors"
)

var charmap [256]byte

func init() {
	for i := 0; i < len(charmap); i++ {
		c := byte(i)
		switch {
		case c >= 'A' && c <= 'Z':
			charmap[i] = c
		case c >= 'a' && c <= 'z':
			charmap[i] = c - 'a' + 'A'
		}
	}
}

var (
	blackList = make(map[string]bool, 256)
	fastOpStr = make(map[string]string, 512)
)

func init() {
	for _, s := range []string{
		"KEYS", "MOVE", "OBJECT", "RENAME", "RENAMENX", "SCAN", "BITOP", "MSETNX", "MIGRATE", "RESTORE",
		"BLPOP", "BRPOP", "BRPOPLPUSH", "PSUBSCRIBE", "PUBLISH", "PUNSUBSCRIBE", "SUBSCRIBE", "RANDOMKEY",
		"UNSUBSCRIBE", "DISCARD", "EXEC", "MULTI", "UNWATCH", "WATCH", "SCRIPT",
		"BGREWRITEAOF", "BGSAVE", "CLIENT", "CONFIG", "DBSIZE", "DEBUG", "FLUSHALL", "FLUSHDB",
		"LASTSAVE", "MONITOR", "SAVE", "SHUTDOWN", "SLAVEOF", "SLOWLOG", "SYNC", "TIME",
		"SLOTSINFO", "SLOTSDEL", "SLOTSMGRTSLOT", "SLOTSMGRTONE", "SLOTSMGRTTAGSLOT", "SLOTSMGRTTAGONE", "SLOTSCHECK",
	} {
		blackList[s] = true
	}
	for _, s := range []string{
		"GET", "SET", "SETNX", "SETEX", "PSETEX", "APPEND", "STRLEN", "DEL", "EXISTS", "SETBIT", "GETBIT",
		"SETRANGE", "GETRANGE", "SUBSTR", "INCR", "DECR", "MGET", "RPUSH", "LPUSH", "RPUSHX", "LPUSHX", "LINSERT",
		"RPOP", "LPOP", "BRPOP", "BRPOPLPUSH", "BLPOP", "LLEN", "LINDEX", "LSET", "LRANGE", "LTRIM", "LREM",
		"RPOPLPUSH", "SADD", "SREM", "SMOVE", "SISMEMBER", "SCARD", "SPOP", "SRANDMEMBER", "SINTER", "SINTERSTORE",
		"SUNION", "SUNIONSTORE", "SDIFF", "SDIFFSTORE", "SMEMBERS", "SSCAN", "ZADD", "ZINCRBY", "ZREM",
		"ZREMRANGEBYSCORE", "ZREMRANGEBYRANK", "ZREMRANGEBYLEX", "ZUNIONSTORE", "ZINTERSTORE", "ZRANGE",
		"ZRANGEBYSCORE", "ZREVRANGEBYSCORE", "ZRANGEBYLEX", "ZREVRANGEBYLEX", "ZCOUNT", "ZLEXCOUNT", "ZREVRANGE",
		"ZCARD", "ZSCORE", "ZRANK", "ZREVRANK", "ZSCAN", "HSET", "HSETNX", "HGET", "HMSET", "HMGET", "HINCRBY",
		"HINCRBYFLOAT", "HDEL", "HLEN", "HKEYS", "HVALS", "HGETALL", "HEXISTS", "HSCAN", "INCRBY", "DECRBY",
		"INCRBYFLOAT", "GETSET", "MSET", "MSETNX", "RANDOMKEY", "SELECT", "MOVE", "RENAME", "RENAMENX",
		"EXPIRE", "EXPIREAT", "PEXPIRE", "PEXPIREAT", "KEYS", "SCAN", "DBSIZE", "AUTH", "PING", "ECHO", "SAVE",
		"BGSAVE", "BGREWRITEAOF", "SHUTDOWN", "LASTSAVE", "TYPE", "MULTI", "EXEC", "DISCARD", "SYNC", "PSYNC",
		"REPLCONF", "FLUSHDB", "FLUSHALL", "SORT", "INFO", "MONITOR", "TTL", "PTTL", "PERSIST", "SLAVEOF", "ROLE",
		"DEBUG", "CONFIG", "SUBSCRIBE", "UNSUBSCRIBE", "PSUBSCRIBE", "PUNSUBSCRIBE", "PUBLISH", "PUBSUB", "WATCH",
		"UNWATCH", "RESTORE", "MIGRATE", "DUMP", "OBJECT", "CLIENT", "EVAL", "EVALSHA", "SLOWLOG", "SCRIPT", "TIME",
		"BITOP", "BITCOUNT", "BITPOS", "COMMAND", "PFSELFTEST", "PFADD", "PFCOUNT", "PFMERGE", "PFDEBUG", "LATENCY",
		"SLOTSINFO", "SLOTSDEL", "SLOTSMGRTSLOT", "SLOTSMGRTONE", "SLOTSMGRTTAGSLOT",
		"SLOTSMGRTTAGONE", "SLOTSHASHKEY", "SLOTSCHECK", "SLOTSRESTORE",
	} {
		fastOpStr[s] = s
	}
}

func isNotAllowed(opstr string) bool {
	return blackList[opstr]
}

var (
	ErrBadMultiBulk = errors.New("bad multi-bulk for command")
	ErrBadOpStrLen  = errors.New("bad command length, too short or too long")
)

const MaxOpStrLen = 64

func getOpStr(multi []*redis.Resp) (string, error) {
	if len(multi) < 1 {
		return "", errors.Trace(ErrBadMultiBulk)
	}

	var upper [MaxOpStrLen]byte

	var op = multi[0].Value
	if len(op) == 0 || len(op) > len(upper) {
		return "", ErrBadOpStrLen
	}
	for i := 0; i < len(op); i++ {
		if c := charmap[op[i]]; c != 0 {
			upper[i] = c
		} else {
			return strings.ToUpper(string(op)), nil
		}
	}
	op = upper[:len(op)]
	if opstr, ok := fastOpStr[string(op)]; ok {
		return opstr, nil
	}
	return string(op), nil
}

func hashSlot(key []byte) int {
	const (
		TagBeg = '{'
		TagEnd = '}'
	)
	if beg := bytes.IndexByte(key, TagBeg); beg >= 0 {
		if end := bytes.IndexByte(key[beg+1:], TagEnd); end >= 0 {
			key = key[beg+1 : beg+1+end]
		}
	}
	return int(crc32.ChecksumIEEE(key) % models.MaxSlotNum)
}

func getHashKey(multi []*redis.Resp, opstr string) []byte {
	var index = 1
	switch opstr {
	case "ZINTERSTORE", "ZUNIONSTORE", "EVAL", "EVALSHA":
		index = 3
	}
	if index < len(multi) {
		return multi[index].Value
	}
	return nil
}
