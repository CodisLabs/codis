#!/bin/sh
../bin/codis-config -c config.ini -L ./log/cconfig.log server add 1 localhost:6381 master

