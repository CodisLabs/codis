all: build

clean:
	rm -rf bin/cconfig

build:
	go build -o bin/cconfig ./cmd/cconfig
	go build -o bin/proxy ./cmd/proxy
	rm -rf bin/assets
	cp -r ./cmd/cconfig/assets ./bin/
