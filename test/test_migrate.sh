for((i=1;i<=1000;i++));do
	slotNo=$((i%1024))
	r=$RANDOM
	if [ $r -eq 2 ];then
		./bin/cconfig slot migrate $slotNo $slotNo 1 --delay=10
	else
		./bin/cconfig slot migrate $slotNo $slotNo 2 --delay=10
	fi
	sleep 1
done;
