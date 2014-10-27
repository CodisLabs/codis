#!/bin/sh
CODIS_CONF=./conf.ini
export CODIS_CONF

echo "slots initializing..."
../bin/cconfig slot init
echo "done"

echo "set slot ranges to server groups..."
../bin/cconfig slot set-range -range 0-341 -status online  -group 1
../bin/cconfig slot set-range -range 342-682 -status online  -group 2
../bin/cconfig slot set-range -range 683-1023 -status online  -group 3
echo "done"

