#!/bin/sh

for i in 638{0..3}; do
    nohup ../bin/codis-server ./redis_conf/${i}.conf &> ./log/redis_${i}.log &
done

echo "sleep 3s"
sleep 3
tail -n 30 ./log/redis_*.log

