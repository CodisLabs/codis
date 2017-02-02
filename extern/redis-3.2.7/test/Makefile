all:

run:
	make -C ../src
	@python3 setup.py

clean:
	@rm -f *.conf *.toml *.pyc *.log *.rdb

00 01 10 11:
	../src/redis-cli -p 20$@

log:
	tail -f *.log

test:
	go test -v ./...
