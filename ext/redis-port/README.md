redis-port
===========

parse redis rdb file, sync data between redis master and slave

* decode redis-dump payload to human readable format
```sh
redis-port decode   [--ncpu=N]  [--input=INPUT]  [--output=OUTPUT]
```
* restore rdb file to redis
```sh
redis-port restore  [--ncpu=N]  [--input=INPUT]   --target=TARGET
```
* dump rdb file from redis master
```
redis-port dump     [--ncpu=N]   --from=MASTER   [--output=OUTPUT]
```
* sync data from master to redis (use slotsrestore as migration command)
```
redis-port sync     [--ncpu=N]   --from=MASTER    --target=TARGET
```
