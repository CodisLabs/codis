### Codis 是什么?

Codis 是 Wandoujia Infrastructure Team 开发的一个分布式 Redis 服务, 用户可以看成是一个无限内存的 Redis 服务, 有动态扩/缩容的能力. 对偏存储型的业务更实用, 如果你需要 SUBPUB 之类的指令, Codis 是不支持的. 时刻记住 Codis 是一个分布式存储的项目. 对于海量的 key, value不太大( <= 1M ), 随着业务扩展缓存也要随之扩展的业务场景有特效.

### 使用 Codis 有什么好处?

Redis获得动态扩容/缩容的能力，增减redis实例对client完全透明、不需要重启服务，不需要业务方担心 Redis 内存爆掉的问题. 也不用担心申请太大, 造成浪费. 业务方也不需要自己维护 Redis.

Codis支持水平扩容/缩容，扩容可以直接界面的 "Auto Rebalance" 按钮，缩容只需要将要下线的实例拥有的slot迁移到其它实例，然后在界面上删除下线的group即可。

### 我的服务能直接迁移到 Codis 上吗?

分两种情况: 
 
1) 原来使用 twemproxy 的用户:
可以, 使用codis项目内的redis-port工具, 可以实时的同步 twemproxy 底下的 redis 数据到你的 codis 集群上. 搞定了以后, 只需要你修改一下你的配置, 将 twemproxy 的地址改成 codis 的地址就好了. 除此之外, 你什么事情都不用做.

2) 原来使用 Redis 的用户:
如果你使用了[doc/unsupported_cmds](https://github.com/CodisLabs/codis/blob/master/doc/unsupported_cmds.md)中提到的命令，是无法直接迁移到 Codis 上的. 你需要修改你的代码, 用其他的方式实现.

### 相对于twemproxy的优劣？
codis和twemproxy最大的区别有两个：一个是codis支持动态水平扩展，对client完全透明不影响服务的情况下可以完成增减redis实例的操作；一个是codis是用go语言写的并支持多线程而twemproxy用C并只用单线程。
后者又意味着：codis在多核机器上的性能会好于twemproxy；codis的最坏响应时间可能会因为GC的STW而变大，不过go1.5发布后会显著降低STW的时间；如果只用一个CPU的话go语言的性能不如C，因此在一些短连接而非长连接的场景中，整个系统的瓶颈可能变成accept新tcp连接的速度，这时codis的性能可能会差于twemproxy。

### 相对于redis cluster的优劣？
redis cluster基于smart client和无中心的设计，client必须按key的哈希将请求直接发送到对应的节点。这意味着：使用官方cluster必须要等对应语言的redis driver对cluster支持的开发和不断成熟；client不能直接像单机一样使用pipeline来提高效率，想同时执行多个请求来提速必须在client端自行实现异步逻辑。
而codis因其有中心节点、基于proxy的设计，对client来说可以像对单机redis一样去操作proxy（除了一些命令不支持），还可以继续使用pipeline并且如果后台redis有多个的话速度会显著快于单redis的pipeline。同时codis使用zookeeper来作为辅助，这意味着单纯对于redis集群来说需要额外的机器搭zk，不过对于很多已经在其他服务上用了zk的公司来说这不是问题：）

### Codis是如何分片的？
Codis 采用 Pre-sharding 的技术来实现数据的分片, 默认分成 1024 个 slots (0-1023), 对于每个key来说, 通过以下公式确定所属的 Slot Id : SlotId = crc32(key) % 1024。

每一个 slot 都会有一个且必须有一个特定的 server group id 来表示这个 slot 的数据由哪个 server group 来提供。数据的迁移也是以slot为单位的。

### Codis 可以当队列使用吗?

可以, Codis 还是支持 LPUSH LPOP这样的指令, 但是注意, 并不是说你用了 Codis 就有了一个分布式队列. 对于单个 key, 它的 value 还是会在一台机器上, 所以, 如果你的队列特别大, 而且整个业务就用了几个 key, 那么就几乎退化成了单个 redis 了, 所以, 如果你是希望有一个队列服务, 那么我建议你:

1. List key 里存放的是任务id, 用另一个key-value pair来存放具体任务信息
2. 使用 Pull 而不是 Push, 由消费者主动拉取数据, 而不是生产者推送数据.
3. 监控好你的消费者, 当队列过长的时候, 及时报警. 
4. 可以将一个大队列拆分成多个小的队列, 放在不同的key中

### Codis 支持 MSET, MGET吗?

支持, 在比较早的版本中MSET/MGET的性能较差，甚至可能差过单机redis，但2.0开始会因为后台并发执行而比单机的mset/mget快很多。

### Codis 是多线程的吗?

虽然Redis是单线程的，但Codis proxy 是多线程的(严格来说是 goroutine), 启动的线程数是 CPU 的核数, 是可以充分利用起多核的性能的。

### Codis 支持 CAS 吗? 支持 Lua 脚本吗?

CAS 暂时不支持, 目前只支持eval的方式来跑lua脚本，需要配合TAG使用. 

### 有没有 zookeeper 的教程？

[请参考这里](http://www.juvenxu.com/2015/03/20/experiences-on-zookeeper-ops/)

### Codis的性能如何?

见Readme中的[Benchmark一节](https://github.com/CodisLabs/codis#performance-benchmark)。

### 我的数据在 Codis 上是安全的吗?

首先, 安全的定义有很多个级别, Codis 并不是一个多副本的系统 (用纯内存来做多副本还是很贵的), 如果 Codis 底下的 redis 机器没有配从, 也没开 bgsave, 如果挂了, 那么最坏情况下会丢失这部分的数据, 但是集群的数据不会全失效 (即使这样的, 也比以前单点故障, 一下全丢的好...-_-|||). 如果上一种情况下配了从, 这种情况, 主挂了, 到从切上来这段时间, 客户端的部分写入会失败. 主从之前没来得及同步的小部分数据会丢失.
第二种情况, 业务短时间内爆炸性增长, 内存短时间内不可预见的暴涨(就和你用数据库磁盘满了一样), Codis还没来得及扩容, 同时数据迁移的速度小于暴涨的速度, 此时会触发 Redis 的 LRU 策略, 会淘汰老的 Key. 这种情况也是无解...不过按照现在的运维经验, 我们会尽量预分配一些 buffer, 内存使用量大概 80% 的时候, 我们就会开始扩容.

除此之外, 正常的数据迁移, 扩容缩容, 数据都是安全的. 

### 你们如何保证数据迁移的过程中多个 Proxy 不会读到老的数据 (迁移的原子性) ? 

见 [Codis 数据迁移流程](http://0xffff.me/blog/2014/11/11/codis-de-she-ji-yu-shi-xian-part-2/)

### Codis支持etcd吗 ? 

支持，请参考使用教程，需要将配置文件中的coordinator=zookeeper改为etcd。

但是需要注意：请使用codis2.0.10或更新的版本，旧版对etcd的支持有一些问题；由于etcd在2.2引入了一个与旧版不兼容的坑爹改动，导致如果使用etcd的版本>=2.2.0，暂时需要手动改一处代码，详情见[相关issue](https://github.com/CodisLabs/codis/issues/488)。

### 现有redis集群上有上T的数据，如何迁移到Codis上来？

为了提高 Codis 推广和部署上的效率，我们为数据迁移提供了一个叫做 [redis-port](https://github.com/CodisLabs/redis-port) 的命令行工具，它能够：

+ 静态分析 RDB 文件，包括解析以及恢复 RDB 数据到 redis
+ 从 redis 上 dump RDB 文件以及从 redis 和 codis 之间动态同步数据

### 如果需要迁移现有 redis 数据到 codis，该如何操作？

+ 先搭建好 codis 集群并让 codis-proxy 正确运行起来
+ 对线上每一个 redis 实例运行一个 redis-port 来向 codis 导入数据，例如：

		for port in {6379,6380,6479,6480}; do
			nohup redis-port sync --ncpu=4 --from=redis-server:${port} \
				--target=codis-proxy:19000 > ${port}.log 2>&1 &
			sleep 5
		done
		tail -f *.log
		
	- 每个 redis-port 负责将对应的 redis 数据导入到 codis
	- 多个 redis-port 之间不互相干扰，除非多个 redis 上的 key 本身出现冲突
	- 单个 redis-port 可以将负责的数据并行迁移以提高速度，通过 --ncpu 来指定并行数
	- 导入速度受带宽以及 codis-proxy 处理速度限制(本质是大量的 slotsrestore 操作)
	
+ 完成数据迁移，在适当的时候将服务指向 Codis，并将原 redis 下线

	- 旧 redis 下线时，会导致 reids-port 链接断开，于是自动退出
		
### redis-port 是如何在线迁移数据的？

+ redis-port 本质是以 slave 的形式挂载到现有 redis 服务上去的

	1. redis 会生成 RDB DUMP 文件给作为 slave 的 redis-port
	2. redis-port 分析 RDB 文件，并拆分成 key-value 对，通过 [slotsrestore](https://github.com/CodisLabs/codis/blob/master/doc/redis_change_zh.md#slotsrestore-key1-ttl1-val1-key2-ttl2-val2-) 指令发给 codis
	3. 迁移过程中发生的修改，redis 会将这些指令在 RDB DUMP 发送完成后，再发给 redis-port，而 redis-port 收到这些指令后不作处理，而直接转发给 Codis
	
+ redis-port 处理还是很快的，参考：
	- https://github.com/sripathikrishnan/redis-rdb-tools
	- https://github.com/cupcake/rdb

### Dashboard 中 Ops 一直是 0？

检查你的启动 dashboard 进程的机器，看是否可以访问proxy的地址，对应的地址是 proxy 启动参数中的 debug_var_addr 中填写的地址。

###  zk: node already exists
无论是proxy还是dashboard，都会在zk上注册自己的节点，同时在程序正常退出的时候会删掉对应的节点，但如果异常退出或试用`kill -9 {pid}`就会导致zk的节点无法删除，在下一次启动的时候会报“zk: node already exists”的错误。

因此关闭服务的时候直接用`kill {pid}`不要-9，同时如果无法启动并且确认没有其他运行中的进程占用zk上的节点，可以在zk上手动删除/zk/codis/db_test/dashboard 或/zk/codis/db_test/fence/{host:port}.

### 编译报错  undefined: utils.Version
说明没有正确的设置go项目路径导致生成的文件找不到。见[安装教程](https://github.com/CodisLabs/codis/blob/master/doc/tutorial_zh.md#build-codis-proxy--codis-config)来正确配置环境变量并用正确的方式下载代码。

### zk: session has been expired by the server
因为使用的go-zookeeper库代码注释里写的session超时时间参数单位是秒，但实际上该参数的单位是毫秒，所以<=2.0.7的版本错误的将默认配置设成30。因此请将配置文件修改为30000。此外2.0.8起如果配置文件中的参数过小则自动将秒转化为毫秒。

Codis的proxy会注册在zk上并监听新的zk事件。因为涉及到数据一致性的问题，所有proxy必须能尽快知道slot状态的改变，因此一旦和zk的连接出了问题就无法知道最新的slot信息从而可能不得不阻塞一些请求以防止数据错误或丢失。

Proxy会每几秒给zk发心跳，proxy的load太高可能导致timeout时间内（默认30秒，配置文件中可以修改）没有成功发心跳导致zk认为proxy已经挂了（当然也可能proxy确实挂了），
这时如果client用了我们修改的Jedis, [Jodis](https://github.com/CodisLabs/jodis)，是会监控到zk节点上proxy少了一个从而自动避开请求这个proxy以保证客户端业务的可用性。如果用非Java语言可以根据Jodis代码DIY一个监听zk的客户端。
另外，如果需要异步请求，可以使用我们基于Netty开发的[Nedis](https://github.com/CodisLabs/nedis)。

当然，proxy收到session expired的错误也不意味着proxy一定要强制退出，但是这是最方便、安全的实现方式。而且实际使用中出现错误的情况通常是zk或proxy的load过高导致的，即使这时不退出可能业务的请求也会受影响，因此出现这个问题通常意味着需要加机器了。

如果不依赖zk/Jodis来做proxy的高可用性（虽然我们建议这样做），可以适当延长配置文件中的超时时间以降低出这个错误的概率。

### proxy异常退出
目前proxy异常退出主要有两种可能，一种是zk的session expired错误，原因见前一节；另一种是从zk获取集群信息时超时或者报错。总体来说都是因为proxy和zk的连接不稳定导致的，而现在还没有加上重试等逻辑。

不过，最新版的codis已经可以实现proxy自动启动、自动标记为online。因此可以外部通过脚本或supervisor等服务来监控proxy的进程，一旦进程退出可以直接自动重启。
