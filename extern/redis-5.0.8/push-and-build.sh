#!/usr/bin/env bash
SERVER_IP='112.74.183.123'
APP_PORT='6379'
REMOTE_SRC_PATH='/home/service/codis'
LOCAL_SRC_PATH='/Users/cheese/Src/CSrc/codis-change/second_change'
BLUE='\033[36m'
NC='\033[0m'

# Clean middle file
echo -e "${BLUE}[Clean] Clean local middle file. ${NC}"
make clean

# Kill service in remote server 
echo -e "${BLUE}[Stop] Stop current service on remote server. ${NC}"
echo $(ssh root@${SERVER_IP} "lsof -i tcp:${APP_PORT}") > pid-file
pid=$(awk '{print $11}' 'pid-file')
ssh root@${SERVER_IP} "kill ${pid}"

# Push file to remote server and package
rm -f ../redis-5.0.8.zip
zip -r ../redis-5.0.8.zip ../redis-5.0.8/*

scp -r ${LOCAL_SRC_PATH}/redis-5.0.8.zip root@${SERVER_IP}:${REMOTE_SRC_PATH}
ssh -tt root@${SERVER_IP} << exitSSHSignal
rm -rf /home/service/codis/redis-5.0.8
rm -rf Users

cd /home/service/codis
unzip redis-5.0.8.zip
rm -f /home/service/codis/redis-5.0.8.zip

cd redis-5.0.8
make
src/redis-server redis-dev.conf
exit
exitSSHSignal

# PID file remove
rm -f ./pid-file