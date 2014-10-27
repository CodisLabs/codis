#!/bin/sh
../bin/cconfig -c config.ini -L ./log/cconfig.log server add 1 localhost:6381 master
../bin/cconfig -c config.ini -L ./log/cconfig.log server add 2 localhost:6382 master
../bin/cconfig -c config.ini -L ./log/cconfig.log server add 3 localhost:6383 master

