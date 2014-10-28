#!/bin/sh
../bin/cconfig -c config.ini proxy offline proxy_1
nohup ../bin/proxy -c config.ini -L ./log/proxy.log  --cpu=8 --addr=0.0.0.0:19000 --http-addr=0.0.0.0:11000 &

echo "sleep 3s"
sleep 3
tail -n 30 ./log/proxy.log

