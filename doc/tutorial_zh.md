# Codis 使用文档

Codis 是一个分布式 Redis 解决方案, 对于上层的应用来说, 连接到 Codis Proxy 和连接原生的 Redis Server 没有明显的区别 ([不支持的命令列表](unsupported_cmds.md)), 上层应用可以像使用单机的 Redis 一样使用, Codis 底层会处理请求的转发, 不停机的数据迁移等工作, 所有后边的一切事情, 对于前面的客户端来说是透明的, 可以简单的认为后边连接的是一个内存无限大的 Redis 服务.

Codis 由四部分组成:

* Codis Proxy   (codis-proxy)
* Codis Dashboard (codis-config)
* Codis Redis   (codis-server)
* ZooKeeper/Etcd

codis-proxy 是客户端连接的 Redis 代理服务, codis-proxy 本身实现了 Redis 协议, 表现得和一个原生的 Redis 没什么区别 (就像 Twemproxy), 对于一个业务来说, 可以部署多个 codis-proxy, codis-proxy 本身是无状态的.

codis-config 是 Codis 的管理工具, 支持包括, 添加/删除 Redis 节点, 添加/删除 Proxy 节点, 发起数据迁移等操作. codis-config 本身还自带了一个 http server, 会启动一个 dashboard, 用户可以直接在浏览器上观察 Codis 集群的运行状态.

codis-server 是 Codis 项目维护的一个 Redis 分支, 基于 2.8.21 开发, 加入了 slot 的支持和原子的数据迁移指令. Codis 上层的 codis-proxy 和 codis-config 只能和这个版本的 Redis 交互才能正常运行.

Codis 依赖 ZooKeeper 来存放数据路由表和 codis-proxy 节点的元信息, codis-config 发起的命令都会通过 ZooKeeper 同步到各个存活的 codis-proxy.

Codis 支持按照 Namespace 区分不同的产品, 拥有不同的 product name 的产品, 各项配置都不会冲突.

##Build codis-proxy & codis-config

* 安装go[参考这里](https://golang.org/doc/install)
* 根据教程正确设置$GOPATH环境变量。注意$GOPATH是本机所有go项目（包括项目依赖的第三方库）的所在目录，而非单纯codis的所在目录。
* 将$GOPATH/bin设为$PATH的其中一个目录，例如直接PATH=$GOPATH/bin:$PATH，方便执行通过go get安装的命令
* 执行`go get -u -d github.com/wandoulabs/codis`下载codis代码
* 切换到`$GOPATH/src/github.com/wandoulabs/codis`目录执行`make`命令编译代码，并执行`make gotest`来跑测试

建议只通过go get命令来下载codis，除非你非常熟悉go语言的目录引用形式从而不会导致代码放错地方。该命令会下载master分支的最新版，我们会确保master分支的稳定。

执行全部指令后，会在 bin 文件夹内生成 codis-config、codis-proxy、codis-server三个可执行文件。另外, bin/assets 文件夹是 codis-config 的 dashboard http 服务需要的前端资源, 需要和 codis-config 放置在同一文件夹下)

```

$ bin/codis-config -h                                                                                                                                                                                                                           (master)
usage: codis-config  [-c <config_file>] [-L <log_file>] [--log-level=<loglevel>]
		<command> [<args>...]
options:
   -c	配置文件地址
   -L	日志输出文件地址
   --log-level=<loglevel>	输出日志级别 (debug < info (default) < warn < error < fatal)

commands:
	server            redis 服务器组管理
	slot              slot 管理
	dashboard         启动 dashboard 服务
	action            事件管理 (目前只有删除历史事件的日志)
	proxy             proxy 管理
```

```
$ bin/codis-proxy -h

usage: codis-proxy [-c <config_file>] [-L <log_file>] [--log-level=<loglevel>] [--cpu=<cpu_num>] [--addr=<proxy_listen_addr>] [--http-addr=<debug_http_server_addr>]

options:
   -c	配置文件地址
   -L	日志输出文件地址
   --log-level=<loglevel>	输出日志级别 (debug < info (default) < warn < error < fatal)
   --cpu=<cpu_num>		proxy占用的 cpu 核数, 默认1, 最好设置为机器的物理cpu数的一半到2/3左右
   --addr=<proxy_listen_addr>		proxy 的 redis server 监听的地址, 格式 <ip or hostname>:<port>, 如: localhost:9000, :9001
   --http-addr=<debug_http_server_addr>   proxy 的调试信息启动的http server, 可以访问 http://debug_http_server_addr/debug/vars
```

##部署
------------------------

###配置文件
`codis-config` 和 `codis-proxy` 在不加 -c 参数的时候, 默认会读取当前目录下的 config.ini 文件

见[config.ini](../config.ini)中的注释来根据需求修改

###流程

####启动 dashboard
执行 `bin/codis-config dashboard`, 该命令会启动 dashboard

####初始化 slots
执行 `bin/codis-config slot init`，该命令会在zookeeper上创建slot相关信息

####启动 Codis Redis
和官方的Redis Server参数一样

####添加 Redis Server Group
每一个 Server Group 作为一个 Redis 服务器组存在, 只允许有一个 master, 可以有多个 slave, ***group id 仅支持大于等于1的整数***

```
$ bin/codis-config server -h                                                                                                                                                                                                                   usage:
	codis-config server list
	codis-config server add <group_id> <redis_addr> <role>
	codis-config server remove <group_id> <redis_addr>
	codis-config server promote <group_id> <redis_addr>
	codis-config server add-group <group_id>
	codis-config server remove-group <group_id>
```
如: 添加两个 server group, 每个 group 有两个 redis 实例，group的id分别为1和2，
redis实例为一主一从。

添加一个group，group的id为1， 并添加一个redis master到该group
```
$ bin/codis-config server add 1 localhost:6379 master
```
添加一个redis slave到该group
```
$ bin/codis-config server add 1 localhost:6380 slave
```
类似的，再添加group，group的id为2
```
$ bin/codis-config server add 2 localhost:6479 master
$ bin/codis-config server add 2 localhost:6480 slave
```

####设置 server group 服务的 slot 范围
   Codis 采用 Pre-sharding 的技术来实现数据的分片, 默认分成 1024 个 slots (0-1023), 对于每个key来说, 通过以下公式确定所属的 Slot Id : SlotId = crc32(key) % 1024 
   每一个 slot 都会有一个且必须有一个特定的 server group id 来表示这个 slot 的数据由哪个 server group 来提供.

```
$ bin/codis-config slot -h                                                                                                                                                                                                                     
usage:
	codis-config slot init
	codis-config slot info <slot_id>
	codis-config slot set <slot_id> <group_id> <status>
	codis-config slot range-set <slot_from> <slot_to> <group_id> <status>
	codis-config slot migrate <slot_from> <slot_to> <group_id> [--delay=<delay_time_in_ms>]
```

如: 

设置编号为[0, 511]的 slot 由 server group 1 提供服务, 编号 [512, 1023] 的 slot 由 server group 2 提供服务

```
$ bin/codis-config slot range-set 0 511 1 online
$ bin/codis-config slot range-set 512 1023 2 online
```

####启动 codis-proxy
```
 bin/codis-proxy -c config.ini -L ./log/proxy.log  --cpu=8 --addr=0.0.0.0:19000 --http-addr=0.0.0.0:11000
```
刚启动的 codis-proxy 默认是处于 offline状态的, 然后设置 proxy 为 online 状态, 只有处于 online 状态的 proxy 才会对外提供服务
```
 bin/codis-config -c config.ini proxy online <proxy_name>  <---- proxy的id, 如 proxy_1
```

####打开浏览器
 访问http://localhost:18087/admin ， 现在可以在浏览器里面完成各种操作了。
  

##数据迁移
-----------------------------

安全和透明的数据迁移是 Codis 提供的一个重要的功能, 也是 Codis 区别于 Twemproxy 等静态的分布式 Redis 解决方案的地方。

数据迁移的最小单位是 key, 我们在 codis redis 中添加了一些指令, 实现基于key的迁移, 如 SLOTSMGRT等 (命令列表),  每次会将特定 slot 一个随机的 key 发送给另外一个 codis redis 实例, 这个命令会确认对方已经接收, 同时删除本地的这个  k-v 键值, 返回这个  slot 的剩余 key 的数量, 整个操作是原子的.

在 codis-config 管理工具中, 每次迁移任务的最小单位是 slot

如: 将slot id 为 [0-511] 的slot的数据, 迁移到 server group 2上,  --delay 参数表示每迁移一个 key 后 sleep 的毫秒数, 默认是 0, 用于限速.

```
$ bin/codis-config slot migrate 0 511 2 --delay=10
```

迁移的过程对于上层业务来说是安全且透明的, 数据不会丢失,  上层不会中止服务.

注意, 迁移的过程中打断是可以的, 但是如果中断了一个正在迁移某个slot的任务, 下次需要先迁移掉正处于迁移状态的 slot, 否则无法继续 (即迁移程序会检查同一时刻只能有一个 slot 处于迁移状态).


###Auto Rebalance 

Codis 支持动态的根据实例内存, 自动对slot进行迁移, 以均衡数据分布.

```
$ bin/codis-config slot rebalance
```

要求:
 * 所有的codis-server都必须设置了maxmemory参数
 * 所有的 slots 都应该处于 online 状态, 即没有迁移任务正在执行
 * 所有 server group 都必须有 Master

##HA

因为codis的proxy是无状态的，可以比较容易的搭多个proxy来实现高可用性并横向扩容。

对Java用户来说，可以使用经过我们修改过的Jedis，[Jodis](https://github.com/wandoulabs/jodis) ，来实现proxy层的HA。它会通过监控zk上的注册信息来实时获得当前可用的proxy列表，既可以保证高可用性，也可以通过轮流请求所有的proxy实现负载均衡。如果需要异步请求，可以使用我们基于Netty开发的[Nedis](https://github.com/wandoulabs/nedis)。

对下层的redis实例来说，当一个group的master挂掉的时候，应该让管理员清楚，并手动的操作，因为这涉及到了数据一致性等问题（redis的主从同步是最终一致性的）。因此codis不会自动的将某个slave升级成master。
不过我们也提供一种解决方案：[codis-ha](https://github.com/ngaut/codis-ha)。这是一个通过codis开放的api实现自动切换主从的工具。该工具会在检测到master挂掉的时候将其下线并选择其中一个slave提升为master继续提供服务。

需要注意，codis将其中一个slave升级为master时，该组内其他slave实例是不会自动改变状态的，这些slave仍将试图从旧的master上同步数据，因而会导致组内新的master和其他slave之间的数据不一致。因为redis的slave of命令切换master时会丢弃slave上的全部数据，从新master完整同步，会消耗master资源。因此建议在知情的情况下手动操作。使用 `codis-config server add <group_id> <redis_addr> slave` 命令刷新这些节点的状态即可。codis-ha不会自动刷新其他slave的状态。

##升级
我们会不断改进codis、修复bug，因此建议永远尽量使用master上的最新版。根据安装教程执行对应命令会自动更新代码，重新编译后用新的二级制文件替换旧的然后重启进程即可。如果没有特殊说明，codis是允许集群中存在多个版本的proxy或者proxy和dashboard版本不一致的，但是建议只作为升级过程的中间阶段，不要让这种混合多版本的状态持续过长时间。
