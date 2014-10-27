all: build

clean:
	rm -rf bin/cconfig
	rm -f *.rdb
	rm -f bin/*.log
	rm -f *.out
	rm -f bin/*.out
	rm -f *.dump

build:
	go build -o bin/cconfig ./cmd/cconfig
	go build -o bin/proxy ./cmd/proxy
	rm -rf bin/assets
	cp -r ./cmd/cconfig/assets ./bin/
