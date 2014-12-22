build:
	go build -o redis-port main.go

test-decode: build
	./redis-port decode  --ncpu=8  --input=dump.rdb       --output=test.log

test-restore: build
	./redis-port restore --ncpu=8  --input=dump.rdb       --target=127.0.0.1:6379

test-dump: build
	./redis-port dump    --ncpu=8  --from=127.0.0.1:6379  --output=test.rdb

test-sync: build
	./redis-port sync    --ncpu=8  --from=127.0.0.1:6379  --target=127.0.0.1:6380

clean:
	rm -f redis-port

gotest:
	go test -cover ./...
