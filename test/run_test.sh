#!/bin/bash

echo "this is gonna take a while"

trap "kill 0" EXIT SIGQUIT SIGKILL SIGTERM

########################################
# cleanup
rm -f *.log *.log.* *.out

########################################
# rebuild codis-*

make -C ../ build || exit $?

########################################
# stop previous tests

pkill -9 codis-config 2>&1 >/dev/null
pkill -9 codis-server

# start dashboard
../bin/codis-config -c config.ini -L dashboard.log dashboard --addr=:18087 2>&1 >/dev/null &
echo "starting dashboard ..."
sleep 1
../bin/codis-config action remove-lock 2>&1

########################################
# restart codis-server

for p in {6379,6380,6479,6480}; do
    nohup ../bin/codis-server ../extern/redis-test/conf/${p}.conf &>nohup_${p}.out &
done

########################################
# restart codis-config & reset zookeeper slots-info

> config.log


for i in {1,2,3}; do
    ../bin/codis-config proxy offline proxy_${i} 2>&1 >/dev/null
done

../bin/codis-config slot init -f 2>&1 | tee -a config.log

sleep 2

../bin/codis-config server remove-group 1 2>&1 | tee -a config.log
../bin/codis-config server remove-group 2 2>&1 | tee -a config.log

../bin/codis-config server add 1 127.0.0.1:6379 master 2>&1 | tee -a config.log
../bin/codis-config server add 2 127.0.0.1:6380 master 2>&1 | tee -a config.log
../bin/codis-config slot range-set 0 1023 1 online     2>&1 | tee -a config.log

run_gc() {
    for((i=1;i<=1000000;i++));do
        sleep 10
        ../bin/codis-config action gc -n 30000
    done
}

run_gc 2>&1 | tee -a config.log &

########################################
# restart codis-proxy

../bin/codis-proxy -c config1.ini -L proxy1.log --addr=0.0.0.0:9000 --http-addr=0.0.0.0:10000 &
../bin/codis-proxy -c config2.ini -L proxy2.log --addr=0.0.0.0:9001 --http-addr=0.0.0.0:10001 &
../bin/codis-proxy -c config3.ini -L proxy3.log --addr=0.0.0.0:9002 --http-addr=0.0.0.0:10001 &

sleep 2

../bin/codis-config proxy online proxy_1 2>&1 | tee -a config.log
../bin/codis-config proxy online proxy_2 2>&1 | tee -a config.log
../bin/codis-config proxy online proxy_3 2>&1 | tee -a config.log

########################################
# restart slots-migration

sleep 5

run_migration() {
    echo "start migration"
    let i=0
    while true; do
        i=$((i%2+1))
        echo migrate $i
        ../bin/codis-config slot migrate 0 0 $i --delay=10 2>&1
        sleep 10
    done
}

run_migration 2>&1 | tee migration.log &

########################################
# restart redis-tests

sleep 2

run_test() {
    cd ../extern/redis-test
    for ((i=0;i<3;i++)); do
        ./run_test.sh
    done
}

run_test 2>&1 | tee test.log

echo done
