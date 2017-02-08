#!/bin/bash

press=10
proxy1=127.0.0.1:19000
proxy2=127.0.0.1:19001
master1=127.0.0.1:6379
master2=127.0.0.1:6380
slave1=127.0.0.1:6479
slave2=127.0.0.1:6480

run_shell() {
    if [ $# -eq 0 ]; then
        return
    fi
    echo '$' $@
    time $@
    e=$?
    if [ $e -ne 0 ]; then
        exit $e
    fi
    echo
    echo
}

run_basic_hash() {
    run_shell go run \
        basic_hash.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -nkeys=$((press * 1000))
}

run_basic_incr() {
    run_shell go run \
        basic_incr.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -round=$((press * 1000))
}

run_basic_mgrt() {
    run_shell go run \
        basic_mgrt.go \
        utils.go -ncpu=8 \
        -master1=$master1 \
        -master2=$master2 \
        -round=$((press * 1000))
}

run_test_mget() {
    run_shell go run \
        test_mget.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -round=10 \
        -nkeys=$((press * 1000)) \
        -ntags=1000
}

run_test_mset() {
    run_shell go run \
        test_mset.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -round=10 \
        -nkeys=$((press * 1000)) \
        -ntags=1000
}

run_test_incr1() {
    run_shell go run \
        test_incr1.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -round=$((press * 2)) \
        -nkeys=$((press * 1000))
}

run_test_incr2() {
    run_shell go run \
        test_incr2.go \
        utils.go -ncpu=8 \
        -proxy1=$proxy1 \
        -proxy2=$proxy2 \
        -group=8 \
        -round=$((100 / press)) \
        -nkeys=$((press * press * 1000)) \
        -ntags=1000
}

run_test_string() {
    run_shell go run \
        test_string.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -maxlen=$((press * press * 1000))
}

run_test_list() {
    run_shell go run \
        test_list.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -round=10 \
        -nkeys=$((press * 1000)) \
        -nvals=20 \
        -ntags=1000
}

run_test_hset() {
    run_shell go run \
        test_hset.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -round=10 \
        -nkeys=$((press * 100)) \
        -nvals=$((press * 10)) \
        -ntags=1000
}

run_test_pttl() {
    run_shell go run \
        test_pttl.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -round=10 \
        -nkeys=1000 \
        -ntags=1000 \
        -expire=3
}

run_extra_memleak() {
    run_shell go run \
        extra_memleak.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -group=8 \
        -nkeys=1000
}

run_extra_incr() {
    run_shell go run \
        extra_incr.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1 \
        -master1=$master1 \
        -master2=$master2 \
        -slave1=$slave1 \
        -slave2=$slave2 \
        -group=8 \
        -round=10 \
        -nkeys=$((press * 1000)) \
        -ntags=1000
}

run_extra_del() {
    run_shell go run \
        extra_del.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1
}

run_extra_mget() {
    run_shell go run \
        extra_mget.go \
        utils.go -ncpu=8 \
        -proxy=$proxy1
}

if [ "x$1" != "x" ]; then
    run_$1
else
    press=10
    # run_basic_hash
    # run_basic_mgrt
    # run_basic_incr
    run_test_incr1
    run_test_mget
    run_test_mset
    run_test_incr2
    run_test_string
    run_test_list
    run_test_hset
    run_test_pttl
    # run_extra_incr
fi
