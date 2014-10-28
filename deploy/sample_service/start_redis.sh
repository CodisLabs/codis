#!/bin/sh

nohup ../redis-2.8.13/src/redis-server ./redis_conf/6381.conf &> ./log/redis.log &
nohup ../redis-2.8.13/src/redis-server ./redis_conf/6382.conf &> ./log/redis.log &
nohup ../redis-2.8.13/src/redis-server ./redis_conf/6383.conf &> ./log/redis.log &
echo "sleep 3s"
sleep 3
tail -n 30 ./log/redis.log

