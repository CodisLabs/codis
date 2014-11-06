#!/bin/sh
echo "shut down proxy_1..."
../bin/cconfig -c config.ini proxy offline proxy_1
echo "done"

echo "start new proxy..."
nohup ../bin/proxy -c config.ini -L ./log/proxy.log  --cpu=8 --addr=0.0.0.0:19000 --http-addr=0.0.0.0:11000 &
echo "done"

echo "sleep 3s"
sleep 3
tail -n 30 ./log/proxy.log

