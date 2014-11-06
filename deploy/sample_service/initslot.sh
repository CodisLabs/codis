#!/bin/sh
echo "slots initializing..."
../bin/cconfig -c config.ini slot init
echo "done"

echo "set slot ranges to server groups..."
../bin/cconfig -c config.ini slot range-set 0 1023 1 online
echo "done"

