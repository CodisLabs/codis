redis-port
===========

parse redis rdb file, sync data between redis master and slave

* **DECODE** dumped payload to human readable format (hex-encoding)

```sh
redis-port decode   [--ncpu=N]  [--input=INPUT]  [--output=OUTPUT]
```

* **RESTORE** rdb file to target redis

```sh
redis-port restore  [--ncpu=N]  [--input=INPUT]   --target=TARGET  [--extra]
```

* **DUMP** rdb file from master redis

```sh
redis-port dump     [--ncpu=N]   --from=MASTER   [--output=OUTPUT] [--extra]
```

* **SYNC** data from master to slave

```sh
redis-port sync     [--ncpu=N]   --from=MASTER    --target=TARGET  [--sockfile=FILE [--filesize=SIZE]] [--filterdb=DB]
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

+ -e, --extra

> dump or restore following redis backlog commands

+ --filterdb=DB

> filter specifed db number, default is '*'

Examples
-------

* **DECODE**

```sh
$ cat dump.rdb | ./redis-port decode 2>/dev/null
  {"db":0,"type":"string","expireat":0,"key":"a","key64":"YQ==","value64":"MTAwMDA="}
  {"db":0,"type":"string","expireat":0,"key":"b","key64":"Yg==","value64":"aGVsbG8ud29ybGQ="}
  {"db":0,"type":"hash","expireat":0,"key":"c","key64":"Yw==","field":"c1","field64":"YzE=","member64":"MTAw"
  {"db":0,"type":"hash","expireat":0,"key":"c","key64":"Yw==","field":"c2","field64":"YzI=","member64":"dGVzdC5zdHJpbmc="}
  {"db":0,"type":"list","expireat":0,"key":"d","key64":"ZA==","index":0,"value64":"bDE="}
  {"db":0,"type":"list","expireat":0,"key":"d","key64":"ZA==","index":1,"value64":"bDI="}
  {"db":0,"type":"zset","expireat":0,"key":"e","key64":"ZQ==","member":"e1","member64":"ZTE=","score":1.000000}
  {"db":0,"type":"zset","expireat":0,"key":"e","key64":"ZQ==","member":"e2","member64":"ZTI=","score":2.000000}
  ... ...
```

* **RESTORE**

```sh
$ ./redis-port restore -i dump.rdb -t 127.0.0.1:6379 -n 8
  2014/10/28 15:08:26 [ncpu=8] restore from 'dump.rdb' to '127.0.0.1:6379'
  2014/10/28 15:08:27 total = 280149161 -     14267777 [  5%]
  2014/10/28 15:08:28 total = 280149161 -     27325530 [  9%]
  2014/10/28 15:08:29 total = 280149161 -     40670677 [ 14%]
  ... ...                                                    
  2014/10/28 15:08:47 total = 280149161 -    278070563 [ 99%]
  2014/10/28 15:08:47 total = 280149161 -    280149161 [100%]
  2014/10/28 15:08:47 done
```

* **DUMP**

```sh
$ ./redis-port dump -f 127.0.0.1:6379 -o save.rdb
  2014/10/28 15:12:05 [ncpu=1] dump from '127.0.0.1:6379' to 'save.rdb'
  2014/10/28 15:12:06 -
  2014/10/28 15:12:07 -
  ... ...
  2014/10/28 15:12:10 total = 278110192 -            0 [  0%]
  2014/10/28 15:12:11 total = 278110192 -    278110192 [100%]
  2014/10/28 15:12:11 done

$ ./redis-port dump -f 127.0.0.1:6379 | tee save.rdb | ./redis-port decode -o save.log -n 8 2>/dev/null
  2014/10/28 15:12:55 [ncpu=1] dump from '127.0.0.1:6379' to '/dev/stdout'
  2014/10/28 15:12:56 -
  ... ...
  2014/10/28 15:13:10 total = 278110192  -   264373070 [  0%]
  2014/10/28 15:13:11 total = 278110192  -   278110192 [100%]
  2014/10/28 15:13:11 done
```

* **SYNC**

```sh
$ ./redis-port sync -f 127.0.0.1:6379 -t 127.0.0.1:6380 -n 8
  2014/10/28 15:15:41 [ncpu=8] sync from '127.0.0.1:6379' to '127.0.0.1:6380'
  2014/10/28 15:15:42 -
  2014/10/28 15:15:43 -
  2014/10/28 15:15:44 -
  2014/10/28 15:15:46 total = 278110192 -      9380927 [  3%]
  2014/10/28 15:15:47 total = 278110192 -     18605075 [  6%]
  ... ...                                              [    ]
  2014/10/28 15:16:14 total = 278110192 -    269990892 [ 97%]
  2014/10/28 15:16:15 total = 278110192 -    278110192 [100%]
  2014/10/28 15:16:15 done
  2014/10/28 15:16:17 pipe: send = 0             recv = 0
  2014/10/28 15:16:18 pipe: send = 0             recv = 0
  ... ...
```
