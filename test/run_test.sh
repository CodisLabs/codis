#!/bin/bash

which etcd >/dev/null

if [ $? -ne 0 ]; then
    echo "missing etcd"
    exit 1
fi

echo "this is gonna take a while"

trap "kill 0" EXIT SIGQUIT SIGKILL SIGTERM


########################################
# cleanup
./cleanup.sh

########################################
# rebuild codis-*

make -C ../ -j4 || exit $?

cat ../config/dashboard.toml \
    | sed -e "s/Demo3/codis-test/g" \
    > dashboard.toml || exit $?

nohup etcd --name=codis-test &>etcd.log &

lastpid=$!
echo "starting etcd pid=$lastpid ..."
sleep 3

ps -p $lastpid >/dev/null || exit 1

nohup ../bin/codis-dashboard -c dashboard.toml --etcd="127.0.0.1:2379" &> dashboard.log &

lastpid=$!
pidlist=$lastpid
echo "starting dashboard pid=$lastpid"

########################################
# start codis-server

for p in {56379,56380,56479,56480}; do
    sed -e "s/6379/${p}/g" redis.temp > ${p}.cfg
    nohup ../bin/codis-server ${p}.cfg &>${p}.log &
    lastpid=$!
    pidlist="$pidlist $lastpid"
    echo "starting codis-server port=${p} pid=$lastpid"
done

# start codis-proxy

for i in {0..1}; do
    cat ../config/proxy.toml \
        | sed -e "s/Demo3/codis-test/g" \
        | sed -e "s/11080/1108${i}/g" \
        | sed -e "s/19000/1900${i}/g" \
        > proxy${i}.toml || exit $?
    nohup ../bin/codis-proxy -c proxy${i}.toml &>proxy${i}.log &
    lastpid=$!
    pidlist="$pidlist $lastpid"
    echo "starting proxy${i} pid=$lastpid"
done

sleep 3

for pid in $pidlist; do
    echo "checking pid=$pid"
    ps -p $pid >/dev/null || exit 1
done

../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" proxy --create -x 127.0.0.1:11080
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" proxy --create -x 127.0.0.1:11081

../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group --create -g1
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g1 --add -x 127.0.0.1:56379
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g1 --add -x 127.0.0.1:56479
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g1 --master-repair -i 0
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g1 --master-repair -i 1

../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group --create -g2
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g2 --add -x 127.0.0.1:56380
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g2 --add -x 127.0.0.1:56480
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g2 --master-repair -i 0
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" group          -g2 --master-repair -i 1


../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" action --create-range --slot-beg=0 --slot-end=1023 -g1 &>/dev/null
../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" action --set --interval=10

########################################
# start slots-migration

sleep 5

run_migration() {
    echo "start migration"
    let g=1
    while true; do
        now=`date +%T`
        ../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" slots | grep "state" | grep -v "\"\"" >/dev/null
        if [ $? -eq 0 ]; then
            echo $now waiting...
        else
            g=$((3-g))
            echo $now "migrate to group-[$g]"
            ../bin/codis-admin --dashboard=127.0.0.1:18080 -n "codis-test" action --create-range --slot-beg=0 --slot-end=1023 -g $g
        fi
        sleep 1
    done
}

run_migration 2>&1 | tee migration.log &

########################################
# start redis-tests

sleep 2

run_test() {
    cd ../extern/redis-test
    for ((i=0;i<3;i++)); do
        ./run_test.sh
    done
}

run_test 2>&1 | tee test.log

echo done
