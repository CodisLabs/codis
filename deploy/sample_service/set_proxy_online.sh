#!/bin/sh
CODIS_CONF=./conf.ini
export CODIS_CONF

nohup ../bin/cconfig proxy set-status -proxy proxy_1 -status online &> ./log/proxy.op.log &

