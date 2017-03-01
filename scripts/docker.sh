#!/bin/bash

hostip=`ifconfig eth0 | grep "inet " | awk -F " " '{print $2}'`

if [ "x$hostip" == "x" ]; then
    echo "cann't resolve host ip address"
    exit 1
fi

mkdir -p log

case "$1" in
zookeeper)
    docker rm -f      "Codis-Z2181" &> /dev/null
    docker run --name "Codis-Z2181" -d \
            --read-only \
            -p 2181:2181 \
            jplock/zookeeper
    ;;

dashboard)
    docker rm -f      "Codis-D28080" &> /dev/null
    docker run --name "Codis-D28080" -d \
        --read-only -v `realpath ../config/dashboard.toml`:/codis/dashboard.toml \
                    -v `realpath log`:/codis/log \
        -p 28080:18080 \
        codis-image \
        codis-dashboard -l log/dashboard.log -c dashboard.toml --host-admin ${hostip}:28080
    ;;

proxy)
    docker rm -f      "Codis-P29000" &> /dev/null
    docker run --name "Codis-P29000" -d \
        --read-only -v `realpath ../config/proxy.toml`:/codis/proxy.toml \
                    -v `realpath log`:/codis/log \
        -p 29000:19000 -p 21080:11080 \
        codis-image \
        codis-proxy -l log/proxy.log -c proxy.toml --host-admin ${hostip}:21080 --host-proxy ${hostip}:29000
    ;;

server)
    for ((i=0;i<4;i++)); do
        let port="26379 + i"
        docker rm -f      "Codis-S${port}" &> /dev/null
        docker run --name "Codis-S${port}" -d \
            -v `realpath log`:/codis/log \
            -p $port:6379 \
            codis-image \
            codis-server --logfile log/${port}.log
    done
    ;;

fe)
    docker rm -f      "Codis-F8080" &> /dev/null
    docker run --name "Codis-F8080" -d \
         -v `realpath log`:/codis/log \
         -p 8080:8080 \
     codis-image \
     codis-fe -l log/fe.log --zookeeper ${hostip}:2181 --listen=0.0.0.0:8080 --assets=/gopath/src/github.com/CodisLabs/codis/bin/assets
    ;;

cleanup)
    docker rm -f      "Codis-D28080" &> /dev/null
    docker rm -f      "Codis-P29000" &> /dev/null
    for ((i=0;i<4;i++)); do
        let port="26379 + i"
        docker rm -f      "Codis-S${port}" &> /dev/null
    done
    docker rm -f      "Codis-Z2181" &> /dev/null
    ;;

*)
    echo "wrong argument(s)"
    ;;

esac
