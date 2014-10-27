#!/bin/sh

CODIS_CONF=./conf.ini
export CODIS_CONF

nohup ../bin/cconfig dashboard run -addr :8087 &> ./log/dashboard.stdout.log &

