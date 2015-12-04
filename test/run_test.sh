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

cat > dashboard.toml <<EOF
coordinator_name = "etcd"
coordinator_addr = "127.0.0.1:2379"
product_name = "codis-test"
product_auth = ""
admin_addr = "127.0.0.1:18080"
EOF

nohup etcd --name=codis-test &>etcd.log &

lastpid=$!
echo "starting etcd pid=$lastpid ..."
sleep 3

ps -p $lastpid >/dev/null || exit 1

nohup ../bin/codis-dashboard -c dashboard.toml &> dashboard.log &

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

../bin/codis-admin --dashboard=127.0.0.1:18080 --create-proxy -x 127.0.0.1:11080
../bin/codis-admin --dashboard=127.0.0.1:18080 --create-proxy -x 127.0.0.1:11081
../bin/codis-admin --dashboard=127.0.0.1:18080 --create-group --gid 1
../bin/codis-admin --dashboard=127.0.0.1:18080 --group-add    --gid 1 -x 127.0.0.1:56379
../bin/codis-admin --dashboard=127.0.0.1:18080 --group-add    --gid 1 -x 127.0.0.1:56479
../bin/codis-admin --dashboard=127.0.0.1:18080 --create-group --gid 2
../bin/codis-admin --dashboard=127.0.0.1:18080 --group-add    --gid 2 -x 127.0.0.1:56380
../bin/codis-admin --dashboard=127.0.0.1:18080 --group-add    --gid 2 -x 127.0.0.1:56480

../bin/codis-admin --dashboard=127.0.0.1:18080 --sync-action --create -x 127.0.0.1:56379
../bin/codis-admin --dashboard=127.0.0.1:18080 --sync-action --create -x 127.0.0.1:56380
../bin/codis-admin --dashboard=127.0.0.1:18080 --sync-action --create -x 127.0.0.1:56479
../bin/codis-admin --dashboard=127.0.0.1:18080 --sync-action --create -x 127.0.0.1:56480

../bin/codis-admin --dashboard=127.0.0.1:18080 --slot-action --create-range --beg=0 --end=1023 --gid 1 &>/dev/null
../bin/codis-admin --dashboard=127.0.0.1:18080 --slot-action --interval=10

########################################
# start slots-migration

sleep 5

run_migration() {
    echo "start migration"
    let g=1
    while true; do
        now=`date +%T`
        ../bin/codis-admin --dashboard=127.0.0.1:18080 slots | grep "state" | grep -v "\"\"" >/dev/null
        if [ $? -eq 0 ]; then
            echo $now waiting...
        else
            g=$((3-g))
            echo $now "migrate to group-[$g]"
            ../bin/codis-admin --dashboard=127.0.0.1:18080 --slot-action --create --sid 0 --gid $g
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
