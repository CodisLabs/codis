##测试环境##
-----------

+ CPU - 2 x [Intel® Xeon® Processor E5-2620 v2](http://ark.intel.com/products/75789/Intel-Xeon-Processor-E5-2620-v2-15M-Cache-2_10-GHz) 
    
    | Specifications              |               |
    | --------------------------- |:--------------|
    | Launch Date                 | Q3'13         |
    | Processor Number            | E5-2620V2     |
    | Intel® Smart Cache          | 15 MB         |
    | Intel® QPI Speed            | 7.2 GT/s      |
    | \# of QPI Links             | 2             |
    | \# of Cores                 | 6             |
    | \# of Threads               | 12            |
    | Processor Base Frequency    | 2.1 GHz       |
    | Max Turbo Frequency         | 2.6 GHz       |     

+ MEM - 128G

+ Software

    - [Codis 8dc6b66](https://github.com/wandoulabs/codis/commit/886d84f0b3da45b20a085e79f6fbb35384ad0172)
    
        * 16 x Codis Server
            
    - [memtier_benchmark](http://highscalability.com/blog/2014/8/27/the-12m-opssec-redis-cloud-cluster-single-server-unbenchmark.html)
    
            $ memtier_benchmark -s localhost -p 19000 -t $NTHRD \
                --ratio=1:1 -n 100000 -d 100  -c 50 --pipeline=75 --key-pattern=S:S \
            
        + $NTHRD Threads
        + 50        Connections per thread
        + 100000    Requests per thread


##测试结果##
-----------

####测试1. proxy = 1 x 4cores ####
---------------------------------
+ 测试 NTHRD={1,2,4,8,16}

+ 线程/进程总数 = 1proxy x 4线程 + 1bench x NTHRD线程 + 16redis进程
            
    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU | Load  |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: | ----: |
    | 1     | 98770.99   | 37.913       | 13819.99  | 398       | 97        | 182       | 677       | 0.697 |
    | 2     | 97434.82   | 76.937       | 13633.03  | 399       | 179       | 185       | 763       | 1.448 |
    | 4     | 89694.19   | 166.763      | 12549.97  | 399       | 222       | 176       | 797       | 1.866 |
    | 8     | 89755.53   | 336.664      | 3147.00   | 398       | 250       | 179       | 827       | 1.810 |
    | 16    | 88331.46   | 679.699      | 3097.07   | 397       | 266       | 266       | 842       | 1.932 |
    
**备注：CPU 统计为测试开始后连续10s的 TOP 结果的平均值；多个 redis 实例或者 bench 实例 CPU 占用直接相加；以下同**
    
####测试2. Proxy = 1 x 8cores ####

+ 测试 NTHRD={1,2,4,8,16}

+ 线程/进程总数 = 1proxy x 8线程 + 1bench x NTHRD线程 + 16redis进程
            
    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU | Load  |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: | ----: |
    | 1     | 142875.77  | 26.230       | 19991.11  | 786       | 97        | 298       | 1181      | 1.003 |
    | 2     | 140354.52  | 53.546       | 19638.34  | 788       | 196       | 308       | 1292      | 2.491 |
    | 4     | 132988.00  | 112.782      | 18607.62  | 791       | 361       | 291       | 1443      | 4.455 |
    | 8     | 129121.65  | 232.185      | 4527.25   | 790       | 530       | 289       | 1609      | 5.221 |
    | 16    | 129908.10  | 464.887      | 4554.83   | 781       | 563       | 295       | 1639      | 7.849 |
    

####测试3. proxy = 1 x 16cores ####

+ 测试 NTHRD={1,2,4,8,16}

+ 线程/进程总数 = 1proxy x 16线程 + 1bench x NTHRD线程 + 16redis进程
            
    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU | Load  |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: | ----: |
    | 1     | 125341.63  | 29.859       | 17537.74  | 1317      | 98        | 291       | 1706      | 2.800 |
    | 2     | 126253.06  | 59.232       | 17665.27  | 1332      | 195       | 299       | 1826      | 7.207 |
    | 4     | 125533.90  | 119.253      | 17564.64  | 1322      | 357       | 332       | 1990      | 11.59 |
    | 8     | 131596.77  | 227.271      | 4614.04   | 1282      | 443       | 335       | 2057      | 13.65 |
    | 16    | 135754.45  | 440.771      | 4759.81   | 1263      | 462       | 335       | 2060      | 14.66 |

####测试4. proxy = 2 x 4cores ####

+ 两个 bench 实例分别压测不同 proxy；但不同 proxy 共享 redis 集群

+ 测试 NTHRD={1,2,4,8}

+ 线程/进程总数 = 2proxy x 4线程 + 2bench x NTHRD线程 + 16redis进程

    | NTHRD | Ops/sec   | Latency (ms) | MB/sec  | Proxy CPU | Bench CPU | Redis CPU | Total CPU | Load  |
    | ----- | --------: | -----------: | ------: | --------: | --------: | --------: | --------: | ----: |
    | 1     | 170741    | 43.885       | 23.3    | 797       | 196       | 367       | 1360      | 2.340 |
    | 2     | 169283    | 88.47        | 23.02   | 797       | 385       | 366       | 1548      | 4.248 |
    | 4     | 147759    | 202.295      | 20.05   | 783       | 575       | 344       | 1702      | 6.762 |
    | 8     | 149611    | 398.195      | 20.28   | 773       | 535       | 344       | 1652      | 11.33 |
    
**备注：吞吐取不同结果中第 30s 的和，延迟取第 30s 的平均值；以下同**

####测试5. proxy = 2 x 8cores ####

+ 测试 NTHRD={1,2,4,8}

+ 线程/进程总数 = 2proxy x 8线程 + 2bench x NTHRD线程 + 16redis进程
            
    | NTHRD | Ops/sec   | Latency (ms) | MB/sec  | Proxy CPU | Bench CPU | Redis CPU | Total CPU | Load  |
    | ----- | --------: | -----------: | ------: | --------: | --------: | --------: | --------: | ----: |
    | 1     | 251711    | 29.77        | 34.38   | 1452      | 195       | 535       | 2182      | 11.71 |
    | 2     | 240166    | 62.36        | 32.72   | 1392      | 336       | 504       | 2232      | 10.22 |
    | 4     | 235987    | 126.87       | 32.04   | 1339      | 353       | 496       | 2188      | 11.46 |
    | 8     | 236023    | 253.015      | 32.02   | 1307      | 377       | 486       | 2170      | 14.01 |
        
####测试5. proxy = 4 x 4cores ####

+ 测试 NTHRD={1,2,4}

+ 线程/进程总数 = 4proxy x 4线程 + 4bench x NTHRD线程 + 16redis进程
                
    | NTHRD | Ops/sec   | Latency (ms) | MB/sec  | Proxy CPU | Bench CPU | Redis CPU | Total CPU | Load  |
    | ----- | --------: | -----------: | ------: | --------: | --------: | --------: | --------: | ----: |
    | 1     | 240042    | 62.392       | 32.71   | 1291      | 362       | 512       | 2165      | 42.84 |
    | 2     | 246018    | 121.695      | 33.4    | 1291      | 359       | 509       | 2159      | 21.47 |
    | 4     | 237404    | 251.545      | 32.21   | 1267      | 376       | 496       | 2139      | 19.77 |    


###测试脚本###
============

```bash
#!/bin/bash

NCPU=4
NPROXY=2
NTHRD=1

trap "kill 0" EXIT SIGQUIT SIGKILL SIGTERM

for ((i=1;i<=$NPROXY;i++)); do
    codis-config proxy offline proxy_${i} 2>&1 >/dev/null
done

for ((i=1;i<=$NPROXY;i++)); do
    cat > config${i}.ini <<EOF
zk=localhost:2181
product=bench
proxy_id=proxy_${i}
EOF
    let a="${i}+19000"
    let b="${i}+10000"
    codis-proxy --cpu=$NCPU -c config${i}.ini -L proxy${i}.log \
        --addr=0.0.0.0:${a} --http-addr=0.0.0.0:${b} &
done

sleep 2

for ((i=1;i<=$NPROXY;i++)); do
    codis-config proxy online proxy_${i}
done

sleep 5

echo codis-proxy is ready

for ((i=1;i<=$NPROXY;i++)); do
    let a="${i}+19000"
    memtier_benchmark -s 127.0.0.1 -p ${a} \
        --ratio=1:1 -n 100000 -d 100 -t $NTHRD -c 50 \
        --pipeline=75 --key-pattern=S:S > bench${a}.log 2>&1 &
    pids="$pids $!"
done
top -b -n 10 > top.log &
pids="$pids $!"
wait $pids

echo done

sed -e "s/^M/\n/g" bench*.log -i # ^M should be <Ctrl-V><Ctrl-M> :P
```
