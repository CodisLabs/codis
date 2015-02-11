all: build

build:
	@mkdir -p bin; rm -rf testdb-rocksdb/*
	go build -o bin/redis-binlog ./cmd && ./bin/redis-binlog -c conf/config.toml -n 4 --create

clean:
	rm -rf bin/* testdb-rocksdb sync.pipe

gotest:
	go test -cover -v ./pkg/... ./cmd/...
