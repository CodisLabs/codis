package router

import (
	"bytes"
	"hash/crc32"
	"strings"

	"github.com/wandoulabs/codis/pkg/proxy/redis"
	"github.com/wandoulabs/codis/pkg/utils/errors"
)

var charmap [128]byte

func init() {
	for i := 0; i < len(charmap); i++ {
		c := byte(i)
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		charmap[i] = c
	}
}

var (
	blacklist = make(map[string]bool)
)

func init() {
	for _, s := range []string{
		"KEYS", "MOVE", "OBJECT", "RENAME", "RENAMENX", "SORT", "SCAN", "BITOP" /*"MGET",*/ /* "MSET",*/, "MSETNX", "SCAN",
		"BLPOP", "BRPOP", "BRPOPLPUSH", "PSUBSCRIBE，PUBLISH", "PUNSUBSCRIBE", "SUBSCRIBE", "RANDOMKEY",
		"UNSUBSCRIBE", "DISCARD", "EXEC", "MULTI", "UNWATCH", "WATCH", "SCRIPT EXISTS", "SCRIPT FLUSH", "SCRIPT KILL",
		"SCRIPT LOAD" /*, "AUTH" , "ECHO"*/ /*"QUIT",*/ /*"SELECT",*/, "BGREWRITEAOF", "BGSAVE", "CLIENT KILL", "CLIENT LIST",
		"CONFIG GET", "CONFIG SET", "CONFIG RESETSTAT", "DBSIZE", "DEBUG OBJECT", "DEBUG SEGFAULT", "FLUSHALL", "FLUSHDB",
		"LASTSAVE", "MONITOR", "SAVE", "SHUTDOWN", "SLAVEOF", "SLOWLOG", "SYNC", "TIME", "SLOTSMGRTONE", "SLOTSMGRT",
		"SLOTSDEL",
	} {
		blacklist[s] = true
	}
}

func isNotAllowed(opstr string) bool {
	return blacklist[opstr]
}

var (
	ErrBadRespType = errors.New("bad resp type for command")
	ErrBadOpStrLen = errors.New("bad opstr length, too short or too lang")
)

func getOpStr(resp *redis.Resp) (string, error) {
	if resp.Type != redis.TypeArray {
		return "", ErrBadRespType
	}
	if len(resp.Array) == 0 {
		return "", ErrBadRespType
	}
	for _, r := range resp.Array {
		if r.Type == redis.TypeBulkBytes {
			continue
		}
		return "", ErrBadRespType
	}

	var upper [64]byte

	var op = resp.Array[0].Value
	if len(op) == 0 || len(op) > len(upper) {
		return "", ErrBadOpStrLen
	}
	for i := 0; i < len(op); i++ {
		c := uint8(op[i])
		if k := int(c); k < len(charmap) {
			upper[i] = charmap[k]
		} else {
			return strings.ToUpper(string(op)), nil
		}
	}
	return string(upper[:len(op)]), nil
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
	return int(crc32.ChecksumIEEE(key) % MaxSlotNum)
}

func getHashKey(resp *redis.Resp, opstr string) []byte {
	var index = 1
	switch opstr {
	case "ZINTERSTORE", "ZUNIONSTORE", "EVAL", "EVALSHA":
		index = 3
	}
	if index < len(resp.Array) {
		return resp.Array[index].Value
	}
	return nil
}
