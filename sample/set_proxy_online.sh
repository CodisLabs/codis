#!/bin/sh
echo "set proxy_1 online"
../bin/codis-config -c config.ini proxy online proxy_1
echo "done"

