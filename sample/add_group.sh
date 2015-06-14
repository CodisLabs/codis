#!/bin/sh

let group=0

for port in 638{0..3}; do
    let group="1+group"
    echo "add group $group with a master(localhost:$port), Notice: do not use localhost when in produciton"
    ../bin/codis-config -c config.ini -L ./log/cconfig.log server add $group localhost:$port master
done
