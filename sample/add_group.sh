#!/bin/sh
echo "add group 1 with a master(localhost:6381), Notice: do not use localhost when in produciton"
../bin/codis-config -c config.ini -L ./log/cconfig.log server add 1 localhost:6381 master

echo "add group 2 with a master(localhost:6382), Notice: do not use localhost when in produciton"
../bin/codis-config -c config.ini -L ./log/cconfig.log server add 2 localhost:6382 master


