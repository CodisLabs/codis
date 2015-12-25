#!/bin/bash

make -C ../ -j4 || exit 1

PATH=$PATH:`realpath ../bin`

for x in etcd codis-dashboard codis-proxy codis-admin codis-server; do
    which $x >/dev/null
    if [ $? -ne 0 ]; then
        echo "missing $x"
        exit 1
    fi
done

rm -rf tmp; mkdir -p tmp && pushd tmp
if [ $? -ne 0 ]; then
    echo "pushd failed"
    exit 1
fi

trap "kill 0" EXIT SIGQUIT SIGKILL SIGTERM

nohup etcd --name=codis-test &>etcd.log &
lastpid=$!
pidlist=$lastpid
echo "etcd.pid=$lastpid"

for p in 56379 56380 56479 56480; do
    nohup codis-server --port ${p} &>redis-${p}.log &
    lastpid=$!
    pidlist="$pidlist $lastpid"
    echo "codis-server-${p}.pid=$lastpid"
done

for ((i=0;i<2;i++)); do
    let p1="11080+i"
    let p2="19000+i"
    cat > ${p1}.toml <<EOF
product_name = "codis-test"
product_auth = ""
proto_type = "tcp4"
admin_addr = "0.0.0.0:${p1}"
proxy_addr = "0.0.0.0:${p2}"
EOF
    nohup codis-proxy -c ${p1}.toml &>${p1}.log &
    lastpid=$!
    pidlist="$pidlist $lastpid"
    echo "proxy-${p1}x${p2}.pid=$lastpid"
done

cat > dashboard.toml <<EOF
coordinator_name = "etcd"
coordinator_addr = "127.0.0.1:2379"
product_name = "codis-test"
product_auth = ""
admin_addr = "0.0.0.0:18080"
EOF

nohup codis-dashboard -c dashboard.toml &> dashboard.log &
pidlist=$lastpid
echo "dashboard.pid=$lastpid"

sleep 3

for pid in $pidlist; do
    ps -p $pid >/dev/null
    if [ $? -ne 0 ]; then
        echo "pid=$pid not found"
        exit 1
    fi
done

codis_admin() {
    codis-admin --dashboard=127.0.0.1:18080 $@
    if [ $? -ne 0 ]; then
        echo "codis-admin error: $@"
        exit 1
    fi
}

codis_admin --create-group --gid 1
codis_admin --group-add    --gid 1 -x 127.0.0.1:56379
codis_admin --sync-action --create -x 127.0.0.1:56379
codis_admin --group-add    --gid 1 -x 127.0.0.1:56479
codis_admin --sync-action --create -x 127.0.0.1:56479
codis_admin --create-group --gid 2
codis_admin --group-add    --gid 2 -x 127.0.0.1:56380
codis_admin --sync-action --create -x 127.0.0.1:56380
codis_admin --group-add    --gid 2 -x 127.0.0.1:56480
codis_admin --sync-action --create -x 127.0.0.1:56480


for ((i=0;i<2;i++)); do
    let p1="11080+i"
    codis_admin --create-proxy -x 127.0.0.1:${p1}
done

codis_admin --slot-action --create-range --beg=0 --end=1023 --gid 1
codis_admin --slot-action --interval=10

sleep 5

run_migration() {
    echo "start migration"
    let g=1
    while true; do
        sleep 1
        now=`date +%T`
        codis_admin slots | grep "state" | grep -v "\"\"" >/dev/null
        if [ $? -eq 0 ]; then
            echo $now waiting...
            continue
        fi
        g=$((3-g))
        echo $now "migrate to group-[$g]"
        codis_admin --slot-action --create --sid 0 --gid $g
    done
}

run_migration 2>&1 | tee migration.log &

########################################
# start redis-tests

sleep 2

run_test() {
    cd ../../extern/redis-test
    for ((i=0;i<3;i++)); do
        ./run_test.sh
    done
}

run_test 2>&1 | tee test.log

echo done
