#!/bin/sh
CODIS_CONF=./conf.ini
export CODIS_CONF

../bin/cconfig server add -group 1 -type master -addr localhost:6381
../bin/cconfig server add -group 2 -type master -addr localhost:6382
../bin/cconfig server add -group 3 -type master -addr localhost:6383

