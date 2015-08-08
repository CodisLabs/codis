#Codis - yet another fast distributed solution for Redis

[![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/wandoulabs/codis?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Build Status](https://travis-ci.org/wandoulabs/codis.svg)](https://travis-ci.org/wandoulabs/codis)

Codis is a proxy based high performance Redis cluster solution written in Go/C, an alternative to Twemproxy. It supports multiple stateless proxy with multiple redis instances and is engineered to elastically scale, Easily add or remove redis or proxy instances on-demand/dynamicly.

Codis is production-ready and widely used at [wandoujia.com](http://wandoujia.com) and many companies. You can see [Codis Releases](https://github.com/wandoulabs/codis/releases) for latest and most stable realeases.

##Major Changes in 2.0
In Codis 2.0, we:
* Redesign the request dispatcher, now pipeline and mget/mset requests are much faster than ever!
* Codis-server (forked redis) is upgrated to 2.8.21. It brings bugfix from upstream redis and also has optimizations, for example, lower memory consumption and faster migration.
* Optimize the zk connection, it is more stable now. 
* Migration (and auto-rebalance) tasks are saved on zk, it will be continued automatically when the dashboard is restarted.
* Support Redis AUTH command.
* More configuration options, see sample/config.ini

##Features
* Auto rebalance
* Extremely simple to use
* Support both redis or rocksdb transparently
* GUI dashboard & admin tools
* Supports most of Redis commands, Fully compatible with twemproxy(https://github.com/twitter/twemproxy)
* Native Redis clients are supported
* Safe and transparent data migration, Easily add or remove nodes on-demand.
* Command-line interface is also provided
* RESTful APIs

## Build and Install

* Install go & ZooKeeper
* go get -d github.com/wandoulabs/codis
* cd $GOPATH/src/github.com/wandoulabs/codis
* ./bootstrap.sh
* cd sample
* follow instructions in usage.md

## Tutorial

[简体中文](https://github.com/wandoulabs/codis/blob/master/doc/tutorial_zh.md)
[English](https://github.com/wandoulabs/codis/blob/master/doc/tutorial_en.md)

## FAQ

[简体中文](https://github.com/wandoulabs/codis/blob/master/doc/FAQ_zh.md)
[English (WIP) ](https://github.com/wandoulabs/codis/blob/master/doc/FAQ_en.md)

## High Availability

[简体中文](https://github.com/wandoulabs/codis/blob/master/doc/tutorial_zh.md#ha)
[English](https://github.com/wandoulabs/codis/blob/master/doc/tutorial_en.md#ha)

## Architecture

![architecture](doc/pictures/architecture.png)

## Snapshots

Dashboard
![main](doc/pictures/snapshot.png)

Migrate
![migrate](doc/pictures/snapshot_migrate.png)

Slots
![slots](doc/pictures/slots.png)

## Performance (Benchmark)
#### Intel(R) Core(TM) i7-4770 CPU @ 3.40GHz x 1 + 16G RAM
+ Archlinux: 4.0.5-1-ARCH #1 SMP PREEMPT Sat Jun 6 18:37:49 CEST 2015 x86_64 GNU/Linux

+ Go: go version go1.4.2 linux/amd64

+ Redis x 4:

```bash
  for i in {6380..6383}; do
    nohup codis-server ${i}.conf &
  done
```

+ Twemproxy - 1CPU:
  - nutcracker -c nutcracker.yml

```yml
alpha:
  listen: 127.0.0.1:22120
  hash: crc32a
  hash_tag: "{}"
  distribution: ketama
  auto_eject_hosts: false
  timeout: 400
  redis: true
  servers:
   - 127.0.0.1:6380:1
   - 127.0.0.1:6381:1
   - 127.0.0.1:6382:1
   - 127.0.0.1:6383:1
```

+ Codis - 4CPU:
```bash
codis-proxy --cpu=4 -c config.ini -L proxy.log \
  --addr=0.0.0.0:19000 --http-addr=0.0.0.0:10000 &
```

+ RedisBenchmark - 1CPU:
```bash
for clients in {1,2,4,8,16,32,64,100,200,300,500,800}; do
  redis-benchmark -p $target -c $clients -n 5000000 -P 100 \
    -r 1048576 -d 256 -t get,set,mset
done
```

+ Benchmark Results:

![main](doc/bench1/bench.png)

#### Intel(R) Xeon(R) CPU E5-2620 v2 @ 2.10GHz x 2 + 64G RAM
+ CentOS: 2.6.32-279.el6.x86_64 #1 SMP Fri Jun 22 12:19:21 UTC 2012 x86_64 x86_64 x86_64 GNU/Linux

+ Go: go version go1.3.3 linux/amd64

+ Redis x 8:

```bash
  for i in {6380..6387}; do
    nohup codis-server ${i}.conf &
  done
```

+ Twemproxy - 1CPU:
  - nutcracker -c nutcracker.yml

```yml
alpha:
  listen: 127.0.0.1:22120
  hash: crc32a
  hash_tag: "{}"
  distribution: ketama
  auto_eject_hosts: false
  timeout: 400
  redis: true
  servers:
   - 127.0.0.1:6380:1
   - 127.0.0.1:6381:1
   - 127.0.0.1:6382:1
   - 127.0.0.1:6383:1
   - 127.0.0.1:6384:1
   - 127.0.0.1:6385:1
   - 127.0.0.1:6386:1
   - 127.0.0.1:6387:1
```

+ Codis - 4CPU or 8CPU:
```bash
codis-proxy --cpu=4 -c config.ini -L proxy.log \
  --addr=0.0.0.0:19000 --http-addr=0.0.0.0:10000 &
```

```bash
codis-proxy --cpu=8 -c config.ini -L proxy.log \
  --addr=0.0.0.0:19000 --http-addr=0.0.0.0:10000 &
```

+ RedisBenchmark - 1CPU:
```bash
for clients in {1,2,4,8,16,32,64,100,200,300,500,800}; do
  redis-benchmark -p $target -c $clients -n 5000000 -P 100 \
    -r 1048576 -d 256 -t get,set,mset
done
```

+ MemtierBenchmark - 4CPU:
```bash
for i in {1,2,4,8,16,32,64,100,200,300,500,800}; do
  nthread=4
  if [ $i -lt 4 ]; then
    nthread=1
  fi
  let nclient="$i/$nthread"
  memtier_benchmark -p $target -t $nthread -c $nclient \
    --ratio=1:1 --test-time 30 -d 256 --key-pattern=S:S --pipeline=100
done
```

+ Benchmark Results:

![main](doc/bench2/bench.png)

## Authors

* [@goroutine](https://github.com/ngaut)
* [@c4pt0r](https://github.com/c4pt0r)
* [@spinlock9](https://github.com/spinlock)
* [@yangzhe1991](https://github.com/yangzhe1991)

Thanks:

* [@ivanzhaowy](https://github.com/ivanzhaowy)
* [@Apache9](https://github.com/apache9)

## License

Codis is licensed under MIT， see MIT-LICENSE.txt

-------------
*You are welcome to use Codis in your product, and feel free to let us know~ :)*
