all: build

build: build-version godep build-proxy build-config build-server

godep:
	@go get -u github.com/tools/godep
	GOPATH=`godep path` godep restore

build-version:
	@bash genver.sh

build-proxy:
	GOPATH=`godep path`:$$GOPATH go build -o bin/codis-proxy ./cmd/proxy

build-config:
	GOPATH=`godep path`:$$GOPATH go build -o bin/codis-config ./cmd/cconfig
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
	@rm -f sample/log/*.log sample/nohup.out
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

distclean: clean
	@make --no-print-directory --quiet -C extern/redis-2.8.21 clean

gotest:
	GOPATH=`godep path`:$$GOPATH go test ./pkg/... ./cmd/...
