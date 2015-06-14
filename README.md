#Codis - yet another fast distributed solution for Redis

[![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/wandoulabs/codis?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![Build Status](https://travis-ci.org/wandoulabs/codis.svg)](https://travis-ci.org/wandoulabs/codis)

Codis is a proxy based high performance Redis cluster solution written in Go/C, an alternative to Twemproxy.

Codis supports multiple stateless proxy with multiple redis instances.

Codis is engineered to elastically scale, Easily add or remove redis or proxy instances on-demand/dynamicly.

Codis is production-ready and is widely used by [wandoujia.com](http://wandoujia.com).




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
* make gotest
* cd sample
* follow instructions in usage.md

## Tutorial

[简体中文](https://github.com/wandoulabs/codis/blob/master/doc/tutorial_zh.md)  
[English](https://github.com/wandoulabs/codis/blob/master/doc/tutorial_en.md)

## FAQ

[简体中文](https://github.com/wandoulabs/codis/blob/master/doc/FAQ_zh.md)  
[English (WIP) ](https://github.com/wandoulabs/codis/blob/master/doc/FAQ_en.md)

## Performance (Benchmark)
+ Intel(R) Core(TM) i7-4770 CPU @ 3.40GHz
+ MemTotal: 16 GB
+ System:
  - Linux: 4.0.5-1-ARCH #1 SMP PREEMPT Sat Jun 6 18:37:49 CEST 2015 x86_64 GNU/Linux
  - Go: go version go1.4.2 linux/amd64
+ Redis:  
    nohup codis-server 6380.conf &  
    nohup codis-server 6381.conf &  
    nohup codis-server 6382.conf &  
    nohup codis-server 6383.conf &  

+ Twemproxy:  
    redis-benchmark -p 22120 -c $clients -n 5000000 -P 100 -r 1048576 -d 256 -t get,set,mset
  
+ Codis:  
    redis-benchmark -p 19000 -c $clients -n 5000000 -P 100 -r 1048576 -d 256 -t get,set,mset

+ Result:

![main](doc/bench/bench.png)

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

## Authors

* [@goroutine](https://github.com/ngaut)
* [@c4pt0r](https://github.com/c4pt0r)
* [@spinlock9](https://github.com/spinlock)

Thanks:

* [@ivanzhaowy](https://github.com/ivanzhaowy)
* [@Apache9](https://github.com/apache9)

## License

Codis is licensed under MIT， see MIT-LICENSE.txt

-------------
*You are welcome to use Codis in your product, and feel free to let us know~ :)*
