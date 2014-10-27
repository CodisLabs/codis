#!/bin/sh

CODIS_CONF=./conf.ini
export CODIS_CONF

nohup ../bin/cconfig proxy set-status -proxy proxy_1 -status mark_offline &> ./log/proxy.log
nohup ../bin/proxy --cpu 8 --addr 0.0.0.0:19000 --httpAddr 0.0.0.0:11000 | tee ./log/proxy.log &

echo "sleep 3s"
sleep 3
tail -n 30 ./log/proxy.log

