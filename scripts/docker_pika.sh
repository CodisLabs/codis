#!/bin/bash

hostip=`ifconfig ens5 | grep "inet " | awk -F " " '{print $2}'`

pika_out_data_path="/data/chenbodeng/pika_data"

if [ "x$hostip" == "x" ]; then
    echo "cann't resolve host ip address"
    exit 1
fi

mkdir -p log

if [[ "$hostip" == *":"* ]]; then
    echo "hostip format"
    tmp=$hostip
    IFS=':' read -ra ADDR <<< "$tmp"
    for i in "${ADDR[@]}"; do
    # process "$i"
        hostip=$i
    done
fi

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
        codis-image:v3.2 \
        codis-dashboard -l log/dashboard.log -c dashboard.toml --host-admin ${hostip}:28080
    ;;

proxy)
    docker rm -f      "Codis-P29000" &> /dev/null
    docker run --name "Codis-P29000" -d \
        --read-only -v `realpath ../config/proxy.toml`:/codis/proxy.toml \
                    -v `realpath log`:/codis/log \
        -p 29000:19000 -p 21080:11080 \
        codis-image:v3.2 \
        codis-proxy -l log/proxy.log -c proxy.toml --host-admin ${hostip}:21080 --host-proxy ${hostip}:29000
    ;;

# server)
#     for ((i=0;i<4;i++)); do
#         let port="26379 + i"
#         docker rm -f      "Codis-S${port}" &> /dev/null
#         docker run --name "Codis-S${port}" -d \
#             -v `realpath log`:/codis/log \
#             -p $port:6379 \
#             codis-image \
#             codis-server --logfile log/${port}.log
#     done
#     ;;

pika)
    for ((i=0;i<4;i++)); do
        let port="29221 + i"
        let rsync_port="30221 + i"
        let slave_port="31221 + i"
        docker rm -f      "Codis-Pika${port}" &> /dev/null
        docker run --name "Codis-Pika${port}" -d \
            -p $port:9221 \
            -p $rsync_port:10221 \
            -p $slave_port:11221 \
            -v "${pika_out_data_path}/Pika${port}":/data/pika \
            -v `realpath ../pika/pika.conf`:/pika/conf/pika.conf \
            pikadb/pika:v3.3.6
        # docker run -dit --name pika_one_sd -p 9221:9221  -v /data2/chenbodeng/pika/conf/pika.conf:/pika/conf/pika.conf pikadb/pika:v3.3.6
    done
    ;;

fe)
    docker rm -f      "Codis-F8080" &> /dev/null
    docker run --name "Codis-F8080" -d \
         -v `realpath log`:/codis/log \
         -p 8080:8080 \
     codis-image:v3.2 \
     codis-fe -l log/fe.log --zookeeper ${hostip}:2181 --listen=0.0.0.0:8080 --assets=/gopath/src/github.com/CodisLabs/codis/bin/assets
    ;;

cleanup)
    docker rm -f      "Codis-D28080" &> /dev/null
    docker rm -f      "Codis-P29000" &> /dev/null
    docker rm -f      "Codis-F8080" &> /dev/null
    for ((i=0;i<5;i++)); do
        let port="29221 + i"
        docker rm -f      "Codis-Pika${port}" &> /dev/null
        rm -rf "${pika_out_data_path}/Pika${port}"
    done
    docker rm -f      "Codis-Z2181" &> /dev/null
    ;;

cleanup_pika)
    for ((i=0;i<5;i++)); do
        let port="29221 + i"
        docker rm -f   "Codis-Pika${port}"
        rm -rf "${pika_out_data_path}/Pika${port}"
    done
    ;;

*)
    echo "wrong argument(s)"
    ;;

esac

