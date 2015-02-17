#!/bin/sh
nohup ../bin/codis-config -c config.ini -L ./log/dashboard.log dashboard --addr=:18087 --http-log=./log/requests.log &>/dev/null &

