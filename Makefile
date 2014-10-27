all: build

clean:
	rm -rf bin/cconfig
	rm *.rdb
	rm bin/*.log
	rm *.out
	rm bin/*.out

build:
	go build -o bin/cconfig ./cmd/cconfig
	go build -o bin/proxy ./cmd/proxy
	rm -rf bin/assets
	cp -r ./cmd/cconfig/assets ./bin/
