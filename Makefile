all: build

export GO15VENDOREXPERIMENT=1

build: build-version build-proxy build-config build-server

build-version:
	@bash genver.sh

build-proxy:
	go build -i -o bin/codis-proxy ./cmd/proxy

build-config:
	go build -i -o bin/codis-config ./cmd/cconfig
	@rm -rf bin/assets && cp -r cmd/cconfig/assets bin/

build-server:
	@mkdir -p bin
	make -j4 -C extern/redis-2.8.21/
	@rm -f bin/codis-server
	@cp -f extern/redis-2.8.21/src/redis-server bin/codis-server

clean:
	@rm -rf bin
	@rm -f *.rdb *.out *.log *.dump deploy.tar
	@rm -f extern/Dockerfile
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

distclean: clean
	@make --no-print-directory --quiet -C extern/redis-2.8.21 clean

gotest:
	go test ./pkg/... ./cmd/...
