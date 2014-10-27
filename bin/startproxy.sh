./cconfig proxy set-status -proxy proxy_1 -status mark_offline
sleep 3
cd .. && make && cd -
pkill -9 proxy
CODIS_CONF=./config1.ini ./proxy --addr 0.0.0.0:9000 --cpu 8 --httpAddr 0.0.0.0:10000 &
sleep 2
./cconfig proxy set-status -proxy proxy_1 -status online
