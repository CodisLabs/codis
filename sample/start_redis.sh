#!/bin/sh

nohup ../bin/codis-server ./redis_conf/6381.conf &> ./log/redis_6381.log &
nohup ../bin/codis-server ./redis_conf/6382.conf &> ./log/redis_6382.log &
echo "sleep 3s"
sleep 3
tail -n 30 ./log/redis_6381.log
tail -n 30 ./log/redis_6382.log

