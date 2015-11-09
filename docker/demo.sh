#!/bin/bash

hostip=`ifconfig eth0 | grep "inet " | awk -F " " '{print $2}'`

mkdir -p log

case "$1" in
dashboard)
    docker rm -f      "Demo2-D28080" &> /dev/null
    docker run --name "Demo2-D28080" -d \
        --read-only -v `realpath ../config/dashboard.toml`:/codis/dashboard.toml \
                    -v `realpath log`:/codis/log \
        -p 28080:18080 \
        codis-image \
        codis-dashboard -l log/dashboard.log -c dashboard.toml --zookeeper ${hostip}:2181
    ;;

proxy)
    docker rm -f      "Demo2-P21000" &> /dev/null
    docker run --name "Demo2-P21000" -d \
        --read-only -v `realpath ../config/proxy.toml`:/codis/proxy.toml \
                    -v `realpath log`:/codis/log \
        -p 29000:19000 -p 21000:11000 \
        codis-image \
        codis-proxy -l log/proxy.log -c proxy.toml
    ;;

server)
    for ((i=0;i<4;i++)); do
        let port="26379 + i"
        docker rm -f      "Demo2-S${port}" &> /dev/null
        docker run --name "Demo2-S${port}" -d \
            -v `realpath log`:/codis/log \
            -p $port:6379 \
            codis-image \
            codis-server --logfile log/${port}.log
    done
    ;;

*)
    echo "wrong argument(s)"
    ;;

esac
