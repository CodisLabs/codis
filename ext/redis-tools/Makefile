build:
	go build -o redis-tools main.go
	@cp -f redis-tools redis-tools.`git log --date=iso --pretty=format:"%h" -1`

test-decode: build
	./redis-tools decode -in dump.rdb -out test.log -ncpu 8

test-restore: build
	./redis-tools restore -in dump.rdb -target 127.0.0.1:6379 -ncpu 8

test-dump: build
	./redis-tools dump -from 127.0.0.1:6379 -out test.rdb

test-sync: build
	./redis-tools sync -from 127.0.0.1:6379 -to 127.0.0.1:6380 -ncpu 8

clean:
	rm -f redis-tools redis-tools.*
