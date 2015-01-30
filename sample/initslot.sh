#!/bin/sh
echo "slots initializing..."
../bin/codis-config -c config.ini slot init -f
echo "done"

echo "set slot ranges to server groups..."
../bin/codis-config -c  config.ini slot range-set 0 511 1 online
../bin/codis-config -c  config.ini slot range-set 512 1023 2 online
echo "done"

