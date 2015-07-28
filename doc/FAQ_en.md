####What is Codis?
Codis is a distributed redis service developed by wandoujia infrasstructure team, codis can be viewed as an redis server with infinite memory, have the ability of dynamically elastic scaling,  it's more fit for storage business, if you need SUBPUB-like command, Codis is not supported, always remember Codis is a distributed storage system.

###Does Codis support etcd ? 

Yes, please read the tutorial


####Can I use Codis directly in my existing services?

That depends.  
Two cases:  
1) Twemproxy users:  
Yes, codis fully support twemproxy commands, further more, using redis-port tool, you can synchronization the data on twemproxy onto your Codis cluster.

2) Raw redis users:  
That depends, if you use the following commands:  

BGREWRITEAOF, BGSAVE, BITOP, BLPOP, BRPOP, BRPOPLPUSH, CLIENT, CONFIG, DBSIZE, DEBUG, DISCARD, EXEC, FLUSHALL, FLUSHDB, KEYS, LASTSAVE, MIGRATE, MONITOR, MOVE, MSETNX, MULTI, OBJECT, PSUBSCRIBE, PUBLISH, PUNSUBSCRIBE, RANDOMKEY, RENAME, RENAMENX, RESTORE, SAVE, SCAN, SCRIPT, SHUTDOWN, SLAVEOF, SLOTSCHECK, SLOTSDEL, SLOTSINFO, SLOTSMGRTONE, SLOTSMGRTSLOT, SLOTSMGRTTAGONE, SLOTSMGRTTAGSLOT, SLOWLOG, SUBSCRIBE, SYNC, TIME, UNSUBSCRIBE, UNWATCH, WATCH

you should modify your code, because Codis does not support these commands.
