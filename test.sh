#!/bin/bash

trap "kill 0" EXIT SIGQUIT SIGKILL SIGTERM

pkill cconfig

cd test

# stop previous test
./bin/cconfig action remove-lock

./bin/cconfig proxy offline proxy_1
./bin/cconfig proxy offline proxy_2
./bin/cconfig proxy offline proxy_3

./bin/cconfig slot init

# rebuild codis
cd .. && make && cd - || exit $?

pkill -9 redis-server

# rebuild redis
cd ../ext/redis-2.8.13 && make && cd -

# start redis for testing
nohup ../ext/redis-2.8.13/src/redis-server ../ext/test/conf/6379.conf &
nohup ../ext/redis-2.8.13/src/redis-server ../ext/test/conf/6380.conf &
nohup ../ext/redis-2.8.13/src/redis-server ../ext/test/conf/6479.conf &
nohup ../ext/redis-2.8.13/src/redis-server ../ext/test/conf/6480.conf &

echo "sleep 2s"
sleep 2
> proxy1.log
> proxy2.log
> proxy3.log

./gc.sh &

./bin/cconfig server add 1 localhost:6379 master
./bin/cconfig server add 2 localhost:6380 master
./bin/cconfig slot range-set 0 1023 1 online

./bin/proxy -c config1.ini -L proxy1.log --addr=0.0.0.0:9000 --http-addr=0.0.0.0:10000 &
./bin/proxy -c config2.ini -L proxy2.log --addr=0.0.0.0:9001 --http-addr=0.0.0.0:10001 &
./bin/proxy -c config3.ini -L proxy3.log --addr=0.0.0.0:9001 --http-addr=0.0.0.0:10001 &

echo "sleep 2s"
sleep 2
./bin/cconfig proxy online proxy_1
./bin/cconfig proxy online proxy_2
./bin/cconfig proxy online proxy_3

echo "sleep 5s"
sleep 5
cd ../ext/test && ./loopall.sh 2>&1 |tee test.log &
echo "wait for testing"
sleep 30
echo "start migrate"
./test_migrate.sh | tee test_migrate.log
