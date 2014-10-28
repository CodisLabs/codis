#!/bin/sh
echo "slots initializing..."
../bin/cconfig -c config.ini slot init
echo "done"

echo "set slot ranges to server groups..."
../bin/cconfig -c config.ini slot range-set 0 341 1 online
../bin/cconfig -c config.ini slot range-set 342 682 2 online
../bin/cconfig -c config.ini slot range-set 683 1023 3 online
echo "done"

