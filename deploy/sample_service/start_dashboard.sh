#!/bin/sh
nohup ../bin/cconfig -c config.ini -L ./log/dashboard.log dashboard --addr=:18087 --http-log=./log/requests.log &

