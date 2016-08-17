# Miniredis

Pure Go Redis test server, used in Go unittests.


##

Sometimes you want to test code which uses Redis, without making it a full-blown
integration test.
Miniredis implements (parts of) the Redis server, to be used in unittests. It
enables a simple, cheap, in-memory, Redis replacement, with a real TCP interface. Think of it as the Redis version of `net/http/httptest`.

It saves you from using mock code, and since the redis server lives in the
test process you can query for values directly, without going through the server
stack.

There are no dependencies on external binaries, so you can easily integrate it in automated build processes.


## Commands

Implemented commands:

 - Connection (complete)
   - AUTH -- see RequireAuth()
   - ECHO
   - PING
   - SELECT
   - QUIT
 - Key 
   - DEL
   - EXISTS
   - EXPIRE
   - EXPIREAT
   - KEYS
   - MOVE
   - PERSIST
   - PEXPIRE
   - PEXPIREAT
   - PTTL
   - RENAME
   - RENAMENX
   - RANDOMKEY -- call math.rand.Seed(...) once before using.
   - TTL
   - TYPE
   - SCAN
 - Transactions (complete)
   - DISCARD
   - EXEC
   - MULTI
   - UNWATCH
   - WATCH
 - Server
   - DBSIZE
   - FLUSHALL
   - FLUSHDB
 - String keys (complete)
   - APPEND
   - BITCOUNT
   - BITOP
   - BITPOS
   - DECR
   - DECRBY
   - GET
   - GETBIT
   - GETRANGE
   - GETSET
   - INCR
   - INCRBY
   - INCRBYFLOAT
   - MGET
   - MSET
   - MSETNX
   - PSETEX
   - SET
   - SETBIT
   - SETEX
   - SETNX
   - SETRANGE
   - STRLEN
 - Hash keys (complete)
   - HDEL
   - HEXISTS
   - HGET
   - HGETALL
   - HINCRBY
   - HINCRBYFLOAT
   - HKEYS
   - HLEN
   - HMGET
   - HMSET
   - HSET
   - HSETNX
   - HVALS
   - HSCAN
 - List keys
   - LINDEX
   - LINSERT
   - LLEN
   - LPOP
   - LPUSH
   - LPUSHX
   - LRANGE
   - LREM
   - LSET
   - LTRIM
   - RPOP
   - RPOPLPUSH
   - RPUSH
   - RPUSHX
 - Set keys (complete)
   - SADD
   - SCARD
   - SDIFF
   - SDIFFSTORE
   - SINTER
   - SINTERSTORE
   - SISMEMBER
   - SMEMBERS
   - SMOVE
   - SPOP -- call math.rand.Seed(...) once before using.
   - SRANDMEMBER -- call math.rand.Seed(...) once before using.
   - SREM
   - SUNION
   - SUNIONSTORE
   - SSCAN
 - Sorted Set keys (complete)
   - ZADD
   - ZCARD
   - ZCOUNT
   - ZINCRBY
   - ZINTERSTORE
   - ZLEXCOUNT
   - ZRANGE
   - ZRANGEBYLEX
   - ZRANGEBYSCORE
   - ZRANK
   - ZREM
   - ZREMRANGEBYLEX
   - ZREMRANGEBYRANK
   - ZREMRANGEBYSCORE
   - ZREVRANGE
   - ZREVRANGEBYSCORE
   - ZREVRANK
   - ZSCORE
   - ZUNIONSTORE
   - ZSCAN


Since miniredis is intended to be used in unittests timeouts are not implemented.
You can use `Expire()` to see if an expiration is set. The value returned will
be that what the client set, without any interpretation. This is to keep things
testable.

## Example

``` Go
func TestSomething(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	// Optionally set some keys your code expects:
	s.Set("foo", "bar")
	s.HSet("some", "other", "key")

	// Run your code and see if it behaves.
	// An example using the redigo library from "github.com/garyburd/redigo/redis":
	c, err := redis.Dial("tcp", s.Addr())
	_, err = c.Do("SET", "foo", "bar")

	// Optionally check values in redis...
	if got, err := s.Get("foo"); err != nil || got != "bar" {
        t.Error("'foo' has the wrong value")
    }
    // ... or use a helper for that:
    s.CheckGet(t, "foo", "bar")
}
```

## Not supported

Commands which will probably not be implemented:

 - CLUSTER (all)
    - ~~CLUSTER *~~
 - GEO (all) -- unless someone needs these
    - ~~GEOADD~~
    - ~~GEODIST~~
    - ~~GEOHASH~~
    - ~~GEOPOS~~
    - ~~GEORADIUS~~
    - ~~GEORADIUSBYMEMBER~~
 - HyperLogLog (all) -- unless someone needs these
    - ~~PFADD~~
    - ~~PFCOUNT~~
    - ~~PFMERGE~~
 - Key
    - ~~DUMP~~
    - ~~MIGRATE~~
    - ~~OBJECT~~
    - ~~RESTORE~~
    - ~~WAIT~~
 - List keys
    - ~~BLPOP~~
    - ~~BRPOP~~
    - ~~BRPOPLPUSH~~
 - Pub/Sub (all)
    - ~~PSUBSCRIBE~~
    - ~~PUBLISH~~
    - ~~PUBSUB~~
    - ~~PUNSUBSCRIBE~~
    - ~~SUBSCRIBE~~
    - ~~UNSUBSCRIBE~~
 - Scripting (all)
    - ~~EVAL~~
    - ~~EVALSHA~~
    - ~~SCRIPT *~~
 - Server
    - ~~BGSAVE~~
    - ~~BGWRITEAOF~~
    - ~~CLIENT *~~
    - ~~COMMAND *~~
    - ~~CONFIG *~~
    - ~~DEBUG *~~
    - ~~INFO~~
    - ~~LASTSAVE~~
    - ~~MONITOR~~
    - ~~ROLE~~
    - ~~SAVE~~
    - ~~SHUTDOWN~~
    - ~~SLAVEOF~~
    - ~~SLOWLOG~~
    - ~~SYNC~~
    - ~~TIME~~
    

## &c.

See https://github.com/alicebob/miniredis_vs_redis for tests comparing
miniredis against the real thing. Tests are run against Redis 3.0.3 (Debian).


[![Build Status](https://travis-ci.org/alicebob/miniredis.svg?branch=master)](https://travis-ci.org/alicebob/miniredis) 
[![GoDoc](https://godoc.org/github.com/alicebob/miniredis?status.svg)](https://godoc.org/github.com/alicebob/miniredis)
