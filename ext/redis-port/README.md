redis-port
===========

parse redis rdb file, sync data between redis master and slave

* **DECODE** dumped payload to human readable format (hex-encoding)

```sh
redis-port decode   [--ncpu=N]  [--input=INPUT]  [--output=OUTPUT]
```

* **RESTORE** rdb file to target redis

```sh
redis-port restore  [--ncpu=N]  [--input=INPUT]   --target=TARGET
```

* **DUMP** rdb file from master redis

```sh
redis-port dump     [--ncpu=N]   --from=MASTER   [--output=OUTPUT]
```

* **SYNC** data from master to slave

```sh
redis-port sync     [--ncpu=N]   --from=MASTER    --target=TARGET
```

Options
-------
+ -n _N_, --ncpu=_N_

> set runtime.GOMAXPROCS to _N_

+ -i _INPUT_, --input=_INPUT_

> use _INPUT_ as input file, or if it is not given, redis-port reads from stdin (means '/dev/stdin')

+ -o _OUTPUT_, --output=_OUTPUT_

> use _OUTPUT_ as output file, or if it is not given, redis-port writes to stdout (means '/dev/stdout')

+ -m _MASTER_, --master=_MASTER_

> specify the master redis

+ -t _TARGET_, --target=_TARGET_

> specify the slave redis (or target redis)


Examples
-------

* **DECODE**

```sh
$ cat dump.rdb | ./redis-port decode 2>/dev/null
  db=0 type=string expireat=0 key={a|61} value={10000|3130303030}
  db=0 type=string expireat=0 key={b|62} value={hello.world|68656c6c6f20776f726c64}
  db=0 type=hset expireat=0 key={c|63} field={c1|6331} member={100|313030}
  db=0 type=hset expireat=0 key={c|63} field={c2|6332} member={test.string|7465737420737472696e67}
  db=0 type=list expireat=0 key={d|64} element={l2|6c32}
  db=0 type=list expireat=0 key={d|64} element={l1|6c31}
  db=0 type=zset expireat=0 key={e|65} member={e1|6531} score=1.000000
  db=0 type=zset expireat=0 key={e|65} member={e2|6532} score=2.000000
  ... ...
```

* **RESTORE**

```sh
$ ./redis-port restore -i dump.rdb -t 127.0.0.1:6379 -n 8
  2014/10/28 15:08:26 [ncpu=8] restore from 'dump.rdb' to '127.0.0.1:6379'
  2014/10/28 15:08:27 total = 280149161  -   5%, read=14267777       restore=97006
  2014/10/28 15:08:28 total = 280149161  -   9%, read=27325530       restore=186450
  2014/10/28 15:08:29 total = 280149161  -  14%, read=40670677       restore=277160
  ... ...
  2014/10/28 15:08:47 total = 280149161  -  99%, read=278070563      restore=1896369
  2014/10/28 15:08:47 total = 280149161  -  99%, read=280149152      restore=1910976
  2014/10/28 15:08:47 done
```

* **DUMP**

```sh
$ ./redis-port dump -f 127.0.0.1:6379 -o save.rdb
  2014/10/28 15:12:05 [ncpu=1] dump from '127.0.0.1:6379' to 'save.rdb'
  2014/10/28 15:12:06 -
  2014/10/28 15:12:07 -
  ... ...
  2014/10/28 15:12:10 total = 278110192  -   0%, read=0              write=0
  2014/10/28 15:12:11 total = 278110192  - 100%, read=278110192      write=278110192
  2014/10/28 15:12:11 done

$ ./redis-port dump -f 127.0.0.1:6379 | tee save.rdb | ./redis-port decode -o save.log -n 8 2>/dev/null
  2014/10/28 15:12:55 [ncpu=1] dump from '127.0.0.1:6379' to '/dev/stdout'
  2014/10/28 15:12:56 -
  ... ...
  2014/10/28 15:13:10 total = 278110192  -  95%, read=264373070      write=264372046
  2014/10/28 15:13:11 total = 278110192  - 100%, read=278110192      write=278110192
  2014/10/28 15:13:11 done
```

* **SYNC**

```sh
$ ./redis-port sync -f 127.0.0.1:6379 -t 127.0.0.1:6380 -n 8
  2014/10/28 15:15:41 [ncpu=8] sync from '127.0.0.1:6379' to '127.0.0.1:6380'
  2014/10/28 15:15:42 -
  2014/10/28 15:15:43 -
  2014/10/28 15:15:44 -
  2014/10/28 15:15:46 sync: total = 278110192  -   3%, read=9380927        restore=63131
  2014/10/28 15:15:47 sync: total = 278110192  -   6%, read=18605075       restore=125077
  ... ...
  2014/10/28 15:16:14 sync: total = 278110192  -  97%, read=269990892      restore=1825706
  2014/10/28 15:16:15 sync: total = 278110192  -  99%, read=278110183      restore=1880596
  2014/10/28 15:16:15 sync: done
  2014/10/28 15:16:16 pipe: send=42             recv=21
  2014/10/28 15:16:17 pipe: send=0              recv=0
  2014/10/28 15:16:18 pipe: send=0              recv=0
  ... ...
```
