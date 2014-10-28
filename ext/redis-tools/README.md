redis-tools
===========

parse redis rdb file, sync data between redis master and slave

* decode redis-dump payload to human readable format
```sh
redis-tools decode   [--ncpu=N]  [--input=INPUT]  [--output=OUTPUT]
```
* restore rdb file to redis
```sh
redis-tools restore  [--ncpu=N]  [--input=INPUT]   --target=TARGET
```
* dump rdb file from redis master
```
redis-tools dump     [--ncpu=N]   --from=MASTER   [--output=OUTPUT]
```
* sync data from master to redis (use slotsrestore as migration command)
```
redis-tools sync     [--ncpu=N]   --from=MASTER    --target=TARGET
```
