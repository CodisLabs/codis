#!/bin/sh

nohup ../bin/codis-server ./redis_conf/6381.conf &> ./log/redis.log &
echo "sleep 3s"
sleep 3
tail -n 30 ./log/redis.log

