### redis 修改部分（增加若干指令） 
--------------------------------

##### SLOTSINFO [start] [count] 

+ 命令说明：获取 redis 中 slot 的个数以及每个 slot 的大小

+ 命令参数：缺省查询 [0, MAX\_SLOT\_NUM)

  - start - 起始的 slot 序号

    缺省 = 0

  - count - 查询的区间的大小，即查询范围为 [start, start + count)

    缺省 = MAX\_SLOT\_NUM

+ 返回结果：返回结果是 slotinfo 的 array；slotinfo 本身也是一个 array。

        response := []slotinfo{slot1, slot2, slot3, ...}
        slotinfo := []int{slotnum, slotsize}

        其中：
            INT slotnum  : slot 序号
            INT slotsize : slot 内数据个数

+ 例如：

        localhost:6379> slotsinfo 0 128
            1) 1) (integer) 23
               2) (integer) 2
            2) 1) (integer) 29
               2) (integer) 1

##### SLOTSSCAN slotnum cursor [COUNT count]

+ 命令说明：获取指定 slotnum 下的 key 列表

+ 命令参数：参数说明类似 SCAN 命令

    - slotnum - 查询的 slot 序号，[0, MAX\_SLOT\_NUM）

    - cursor - 说明参考 SCAN 命令

    - [COUNT count) - 说明参考 SCAN 命令

        - 暂不支持 MATCH 查询

+ 返回结果：参考 SCAN 命令

    - 返回更新后的 cursor 以及一组 key 列表

+ 例如:

        localhost:6379> slotsscan 579 0 COUNT 10
            1) "10752"
            2)  1) "{a}7836"
                2) "{a}2167"
                3) "{a}5332"
                4) "{a}6292"
                5) "{a}600"
                6) "{a}6094"
                7) "{a}7754"
                8) "{a}4929"
                9) "{a}9211"
               10) "{a}6596"

##### SLOTSDEL slot1 [slot2 …]

+ 命令说明：删除 redis 中若干 slot 下的全部 key-value

+ 命令参数：接受至少 1 个 slotnum 作为参数

+ 返回结果：格式参见 slotsinfo，不同的是：slotsize 表示删除后剩余大小，通常为 0。

+ 例如：

        localhost:6379> slotsdel 1013 990
            1) 1) (integer) 1013
               2) (integer) 0
            2) 1) (integer) 990
               2) (integer) 0

#### 数据迁移
---------------

**以下4个命令是一族命令：**

+ SLOTSMGRTSLOT - *O(1)*

    随机在某个 slot 下迁移一个 key-value 到目标机器

+ SLOTSMGRTONE - *O(1)*

    将指定的 key-value 迁移到目标机

+ SLOTSMGRTTAGSLOT - *O(log(n))*

    随机在某个 slot 下选择一个 key，并将与之有相同 tag 的 key-value 对全部迁移到目标机

+ SLOTSMGRTTAGONE - *O(log(n))*

    将与指定 key 具有相同 tag 的所有 key-value 对迁移到目标机


##### SLOTSMGRTSLOT host port timeout slot

+ 命令说明：随机选择 slot 下的 1 个 key-value 到迁移到目标机（同步 IO 操作）

    - 如果当前 slot 已经空了或者选择的 key 刚好过期，返回 0

    - 如果当前 slot 下面还有 key 则选择一个进行迁移

    - 同时返回当前 slot 剩余 key 的个数

    - 迁移过程在目标机器调用 slotsrestore 命令，迁移会 **覆盖旧值**


+ 命令参数：

    - host:port - 目标机

        redis 内部缓存到 host:port 的连接 30s，超时或错误则关闭

    - timeout - 操作超时，单位 ms

        过程需要 3 个同步操作：

        1. 建立连接（可被缓存优化）

        2. 发送 key-value 数据

        3. 接受目标机返回

        指令保证每个操作不超过 timeout

    - slot - 指定迁移的 slot 序号

+ 返回结果： 操作返回 int

        response := []int{succ,size}

        其中：
            INT succ : 表示迁移是否成功。
                0 表示当前 slot 已经空了（迁移成功个数=0）
                1 表示迁移一个 key 成功，并从本地删除（迁移成功个数=1）
            INT size : 表示 slot 下剩余 key 的个数

+ 例如：

        localhost:6379> set a 100            # set <a, 100>
            OK
        localhost:6379> slotsinfo            # slot 大小为 1
            1) 1) (integer) 579
               2) (integer) 1
        localhost:6379> slotsmgrt 127.0.0.1 6380 100 579
            (integer) 1                      # 成功迁移 value
        localhost:6379> slotsinfo
            (empty list or set)
        localhost:6379> slotsmgrt 127.0.0.1 6380 100 579 1
            (integer) 0                      # 成功成功个数为 0；当前 slot 已经空了


##### SLOTSMGRTONE host port timeout key

+ 命令说明：迁移 key 到目标机，与 slotsmgrtslot 相同

+ 命令参数：参见 slotsmgrtslot

+ 返回结果： 操作返回 整数 (int)

        response := int(succ)

        其中：
            INT succ : 与 slotsmgrtslot 相似

+ 例如：

        localhost:6379> set a 100            # set <a, 100>
            OK
        localhost:6379> slotsinfo
            1) 1) (integer) 579
               2) (integer) 1
        localhost:6379> slotsmgrtone 127.0.0.1 6380 100 a
            (integer) 1                      # 迁移成功
        localhost:6379> slotsmgrtone 127.0.0.1 6380 100 a
            (integer) 0                      # 放弃迁移，本地已经不存在了

##### SLOTSMGRTTAGONE host port timeout key

+ 命令说明：迁移与 key 有相同的 tag 的所有 key 到目标机

    - 当 key 中不包含合法 tag 时，命令退化为 slotsmgrtone，**复杂度为** ***O(1)***

    - 当 key 中包含合法 tag 时，命令会计算 tag 的 hash 值，并在 skiplist 中找到所有具有相同 hash 值的 key-value 对，原子的迁移到目标机，**复杂度为** ***O(log(n))***

    - **备注：修改的 redis 中，会将所有含有 tag 的 key，组织在 skiplist 中，并按照 tag 的 hash 值进行排序。当对按照某一 tag 进行迁移数据时，实际操作会将所有具有相同 hash 值的 tag 所涉及到的所有 key 一起迁移。也就是说，真正迁移的数据可能包含更多的 key，但是这么设计会减少 tag 迁移过程对字符串的比较次数，显著提升性能。**

+ 命令参数：参见 slotsmgrtone

+ 返回结果： 操作返回 整数 (int)

        response := int(succ)

        其中：
            INT succ : 表示成功迁移的 key 的个数。

+ 例如：

        localhost:6379> set a{tag} 100        # set <a{tag}, 100>
            OK
        localhost:6379> set b{tag} 100        # set <b{tag}, 100>
            OK
        localhost:6379> slotsmgrttag 127.0.0.1 6380 1000 {tag}
            (integer) 2
        localhost:6379> scan 0                # 迁移成功，本地不存在了
            1) "0"
            2) (empty list or set)
        localhost:6380> scan 0                # 数据一次成功迁移到目标机
            1) "0"
            2) 1) "a{tag}"
               2) "b{tag}"

##### SLOTSMGRTTAGSLOT host port timeout slot

+ 命令说明：与 slotsmgrtslot 对应的迁移指令

    - 其他说明参考 slotsmgrtslot 以及 slotsmgrttagone 的解释即可

##### SLOTSRESTORE key1 ttl1 val1 [key2 ttl2 val2 …]

+ 命令说明：该命令是对 redis-2.8 的 restore 命令的扩展

    - 可以对 restore 多个 key-value

    - 过程是原子的。

+ **备注：与 restore 不同的是，slotsrestore 只支持 replace，即一定** ***覆盖旧值*** **。如果旧值已经存在，那么只可能是 redis-slots 或者 proxy 的实现 bug，程序会通过 redisLog 打印一条冲突记录。**

#### 调试相关
---------------

##### SLOTSHASHKEY key1 [key2 …]

+ 命令说明：计算并返回给定 key 的 slot 序号

+ 命令参数：输入为 1 个或多个 key

+ 返回结果： 操作返回 array

        response := []int{slot1, slot2...}

        其中：
            INT slot : 表示对应 key 的 slot 序号，即 hash32(key) % NUM_OF_SLOTS

+ 例如：

        localhost:6379> slotshashkey a b c   # 计算 <a,b,c> 的 slot 序号
            1) (integer) 579
            2) (integer) 1017
            3) (integer) 879

##### SLOTSCHECK

+ 命令说明：对 redis 内的 slots 进行一致性检查，即满足如下两条

    - 每个 slot 中保存的 key 都能在 db 中找到对应的 val

    - 每个 db 中的 key 都能在对应的 slot 中查找到

+ 命令参数：0 参数

+ 返回结果： 操作返回 字符串 OK（如果 check 失败，会返回 ERR 并包含对应出错的 key）

+ 例如：

        localhost:6379> set a 100            # set <a, 100>
            OK
        localhost:6379> slotscheck
            OK                               # 检查通过
        …
        localhost:6379> slotscheck
            OK                               # 检查通过，但是耗时 1.07s
            (1.07s)

+ **备注**：***该操作比较慢，仅仅作为 redis 开发的调试工具使用，不能在线上使用***
