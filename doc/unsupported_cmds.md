These commands are disallowed in codis proxy, if you use them, proxy will close the connection to warn you.

|   Command Type   |   Command Name   |
|:----------------:|:---------------- |
|   Keys           | KEYS             |
|                  | MIGRATE          |
|                  | MOVE             |
|                  | OBJECT           |
|                  | RANDOMKEY        |
|                  | RENAME           |
|                  | RENAMENX         |
|                  | SCAN             |
|                  |                  |
|   Strings        | BITOP            |
|                  | MSETNX           |
|                  |                  |
|   Lists          | BLPOP            |
|                  | BRPOP            |
|                  | BRPOPLPUSH       |
|                  |                  |
|   Pub/Sub        | PSUBSCRIBE       |
|                  | PUBLISH          |
|                  | PUNSUBSCRIBE     |
|                  | SUBSCRIBE        |
|                  | UNSUBSCRIBE      |
|                  |                  |
|   Transactions   | DISCARD          |
|                  | EXEC             |
|                  | MULTI            |
|                  | UNWATCH          |
|                  | WATCH            |
|                  |                  |
|   Scripting      | SCRIPT           |
|                  |                  |
|   Server         | BGREWRITEAOF     |
|                  | BGSAVE           |
|                  | CLIENT           |
|                  | CONFIG           |
|                  | DBSIZE           |
|                  | DEBUG            |
|                  | FLUSHALL         |
|                  | FLUSHDB          |
|                  | LASTSAVE         |
|                  | LATENCY          |
|                  | MONITOR          |
|                  | PSYNC            |
|                  | REPLCONF         |
|                  | RESTORE          |
|                  | SAVE             |
|                  | SHUTDOWN         |
|                  | SLAVEOF          |
|                  | SLOWLOG          |
|                  | SYNC             |
|                  | TIME             |
|                  |                  |
|   Codis Slot     | SLOTSCHECK       |
|                  | SLOTSDEL         |
|                  | SLOTSINFO        |
|                  | SLOTSMGRTONE     |
|                  | SLOTSMGRTSLOT    |
|                  | SLOTSMGRTTAGONE  |
|                  | SLOTSMGRTTAGSLOT |


These commands is "half-supported". Codis does not support cross-node operation, so you must use Hash Tags (See [this blog](http://oldblog.antirez.com/post/redis-presharding.html)'s "Hash tags" section) to put all the keys which may shown in one request into the same slot then you can use these commands. Codis does not check if the keys have same tag, so if you don't use tag, your program will get wrong response.

|   Command Type   |   Command Name   |
|:----------------:|:---------------- |
|   Lists          | RPOPLPUSH        |
|                  |                  |
|   Sets           | SDIFF            |
|                  | SINTER           |
|                  | SINTERSTORE      |
|                  | SMOVE            |
|                  | SUNION           |
|                  | SUNIONSTORE      |
|                  |                  |
|   Sorted Sets    | ZINTERSTORE      |
|                  | ZUNIONSTORE      |
|                  |                  |
|   HyperLogLog    | PFMERGE          |
|                  |                  |
|   Scripting      | EVAL             |
|                  | EVALSHA          |

