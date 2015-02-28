##测试环境##
-----------

+ CPU - [Intel® Xeon® Processor E5-2620 v2](http://ark.intel.com/products/75789/Intel-Xeon-Processor-E5-2620-v2-15M-Cache-2_10-GHz) x 2

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

+ MEM - 64GB

+ Software

    - [Codis a915d3e](https://github.com/wandoulabs/codis/tree/a915d3e1bc5b22a11a37292c2717ad8ce88291c1)

        * 16 x Codis Redis

    - [memtier_benchmark](http://highscalability.com/blog/2014/8/27/the-12m-opssec-redis-cloud-cluster-single-server-unbenchmark.html)

            $ memtier_benchmark -s localhost -p 19000 -t $NTHRD \
                --ratio=1:1 -n 100000 -d 100  -c 50 --key-pattern=S:S [--pipeline=75]

        + $NTHRD Threads
        + 50        Connections per thread
        + 100000    Requests per thread

    - [twemproxy-0.4](https://github.com/twitter/twemproxy)

        * 1 x nutcracker

##测试结果##
-----------

####测试0. twemproxy ####
----------------------------------------------

+ pipeline = Yes

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 200515.02  | 18.65700     | 28055.97  | 91        | 30        | 36        | 157       |
    | 2     | 158598.74  | 47.23900     | 22191.06  | 99        | 23        | 34        | 156       |
    | 4     | 146977.70  | 102.00100    | 20565.05  | 99        | 21        | 30        | 150       |
    | 8     | 142634.77  | 210.27500    | 5001.05   | 99        | 21        | 29        | 149       |

+ pipeline = No

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 70873.60   | 0.70400      | 9916.60   | 99        | 98        | 51        | 248       |
    | 2     | 71723.20   | 1.39200      | 10035.48  | 99        | 131       | 45        | 275       |
    | 4     | 80848.61   | 2.47500      | 11312.30  | 99        | 156       | 32        | 287       |
    | 8     | 89063.30   | 4.49900      | 3122.73   | 99        | 186       | 25        | 310       |

**备注：CPU 统计为测试开始后连续10s的 TOP 结果的平均值；多个 redis 实例或者 bench 实例 CPU 占用直接相加；以下同**


####测试1. 4core-proxy ####
----------------------------------------------

+ pipeline = Yes

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 172016.32  | 21.82200     | 24068.44  | 398       | 98        | 140       | 636       |
    | 2     | 186390.20  | 40.38600     | 26079.63  | 398       | 194       | 112       | 704       |
    | 4     | 202224.24  | 74.66500     | 28295.12  | 398       | 380       | 92        | 870       |

+ pipeline = No

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 60970.10   | 0.81700      | 8530.91   | 366       | 99        | 102       | 567       |
    | 2     | 110429.13  | 0.89700      | 15451.19  | 397       | 195       | 145       | 737       |
    | 4     | 153360.21  | 1.30300      | 21458.09  | 398       | 339       | 100       | 837       |


####测试2. 8core-proxy ####
----------------------------------------------

+ pipeline = Yes

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 263014.73  | 14.19900     | 36800.90  | 567       | 98        | 198       | 863       |
    | 2     | 306140.57  | 24.67600     | 42835.05  | 792       | 195       | 235       | 1222      |
    | 4     | 329416.80  | 45.94000     | 46091.84  | 793       | 388       | 193       | 1374      |
    | 8     | 325865.33  | 92.25100     | 11425.47  | 792       | 687       | 138       | 1617      |

+ pipeline = No

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 57244.61   | 0.87100      | 8009.64   | 513       | 99        | 103       | 715       |
    | 2     | 97680.20   | 1.01200      | 13667.37  | 726       | 197       | 184       | 1107      |
    | 4     | 148428.94  | 1.32900      | 20768.11  | 779       | 388       | 240       | 1407      |
    | 8     | 128279.81  | 3.11400      | 4497.74   | 760       | 562       | 222       | 1544      |


####测试3. 12core-proxy ####
----------------------------------------------

+ pipeline = Yes

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 297792.25  | 12.58900     | 41666.95  | 754       | 100       | 229       | 1083      |
    | 2     | 348552.28  | 21.54300     | 48769.27  | 1173      | 196       | 291       | 1660      |
    | 4     | 371880.34  | 40.22200     | 52033.32  | 1171      | 388       | 226       | 1785      |
    | 8     | 396201.81  | 76.07700     | 13891.60  | 1164      | 549       | 178       | 1891      |

+ pipeline = No

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 51946.31   | 0.96000      | 7268.30   | 508       | 99        | 96        | 703       |
    | 2     | 82324.00   | 1.21300      | 11518.74  | 837       | 198       | 163       | 1198      |
    | 4     | 136064.57  | 1.46900      | 19038.09  | 1091      | 390       | 260       | 1741      |
    | 8     | 114749.57  | 3.45000      | 4023.34   | 972       | 438       | 219       | 1629      |


####测试4. 16core-proxy ####
----------------------------------------------

+ pipeline = Yes

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 292297.43  | 12.80000     | 40898.12  | 941       | 100       | 256       | 1297      |
    | 2     | 367385.40  | 20.44800     | 51404.39  | 1476      | 187       | 343       | 2006      |
    | 4     | 386378.88  | 38.69800     | 54061.95  | 1427      | 384       | 276       | 2087      |
    | 8     | 412814.28  | 72.45600     | 14474.07  | 1433      | 415       | 220       | 2068      |

+ pipeline = No

    | NTHRD | Ops/sec    | Latency (ms) | KB/sec    | Proxy CPU | Bench CPU | Redis CPU | Total CPU |
    | ----- | ---------: | -----------: | --------: | --------: | --------: | --------: | --------: |
    | 1     | 49071.19   | 1.01600      | 6866.02   | 478       | 99        | 89        | 666       |
    | 2     | 79192.66   | 1.25600      | 11080.60  | 858       | 198       | 158       | 1214      |
    | 4     | 127074.67  | 1.57000      | 17780.23  | 1211      | 388       | 254       | 1853      |
    | 8     | 114165.92  | 3.49500      | 4002.88   | 1058      | 395       | 224       | 1677      |


###测试脚本###
------------

+ test codis-proxy

```bash
#!/bin/bash

NCPU=4
NPROXY=1
NTHRD=1

trap "kill 0" EXIT SIGQUIT SIGKILL SIGTERM

for ((i=1;i<=$NPROXY;i++)); do
    codis-config proxy offline proxy_${i} 2>&1 >/dev/null
done

for ((i=1;i<=$NPROXY;i++)); do
    cat > config${i}.ini <<EOF
zk=localhost:2181
product=codis_bench
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

+ test nutcracker

```bash
#!/bin/bash

NCPU=4
NPROXY=1
NTHRD=1

trap "kill 0" EXIT SIGQUIT SIGKILL SIGTERM

for ((i=1;i<=$NPROXY;i++)); do
    let a="${i}+19000"
    cat > config${i}.yml <<EOF
alpha:
  listen: 127.0.0.1:${a}
  hash: crc32a
  hash_tag: "{}"
  distribution: ketama
  preconnect: true
  auto_eject_hosts: false
  redis: true
  servers:
    - 127.0.0.1:16380:1
    - 127.0.0.1:16381:1
    - 127.0.0.1:16382:1
    - 127.0.0.1:16383:1
    - 127.0.0.1:16384:1
    - 127.0.0.1:16385:1
    - 127.0.0.1:16386:1
    - 127.0.0.1:16387:1
    - 127.0.0.1:16388:1
    - 127.0.0.1:16389:1
    - 127.0.0.1:16390:1
    - 127.0.0.1:16391:1
    - 127.0.0.1:16392:1
    - 127.0.0.1:16393:1
    - 127.0.0.1:16394:1
    - 127.0.0.1:16395:1
EOF
    nutcracker -c config${i}.yml &
done

sleep 5

echo nutcracker is ready

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
Saturday, 28. February 2015 12:39PM 
