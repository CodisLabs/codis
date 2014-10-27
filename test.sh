#!/bin/bash

pkill cconfig

./bin/cconfig action remove-lock

./bin/cconfig proxy offline proxy_1
./bin/cconfig proxy offline proxy_2
./bin/cconfig proxy offline proxy_3

./bin/cconfig slot init

make || exit $?

pkill -9 redis-server

cd ext/redis-2.8.13 && make && cd -
./bin/cconfig server add 1 localhost:6379 master
./bin/cconfig server add 2 localhost:6380 master
./bin/cconfig slot range-set 0 1023 1 online

nohup ./ext/redis-2.8.13/src/redis-server ./ext/test/conf/6379.conf &
nohup ./ext/redis-2.8.13/src/redis-server ./ext/test/conf/6380.conf &
nohup ./ext/redis-2.8.13/src/redis-server ./ext/test/conf/6479.conf &
nohup ./ext/redis-2.8.13/src/redis-server ./ext/test/conf/6480.conf &


sleep 2
cd bin
> proxy1.log
> proxy2.log
> proxy3.log

./gc.sh &
./proxy -c config1.ini -L proxy1.log --addr=0.0.0.0:9000 --http-addr=0.0.0.0:10000 &
./proxy -c config2.ini -L proxy2.log --addr=0.0.0.0:9001 --http-addr=0.0.0.0:10001 &
./proxy -c config3.ini -L proxy3.log --addr=0.0.0.0:9001 --http-addr=0.0.0.0:10001 &

sleep 2
./cconfig proxy online proxy_1
./cconfig proxy online proxy_2
./cconfig proxy online proxy_3

sleep 5
cd ../ext/test && ./loopall.sh 2>&1 |tee test.log &
echo "wait for testing"
sleep 30
echo "start migrate"
./migrate.sh | tee migrate.log
