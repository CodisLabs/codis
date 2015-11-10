all: build-all

build-all: build-server build-dashboard build-proxy build-admin

build-godep:
	@bash genver.sh
	@go get -u github.com/tools/godep
	GOPATH=`godep path` godep restore

build-proxy: build-godep
	GOPATH=`godep path`:$$GOPATH go build -o bin/codis-proxy ./cmd/proxy

build-admin: build-godep
	GOPATH=`godep path`:$$GOPATH go build -o bin/codis-admin ./cmd/admin

build-dashboard: build-godep
	GOPATH=`godep path`:$$GOPATH go build -o bin/codis-dashboard ./cmd/dashboard

# build-config:
# 	GOPATH=`godep path`:$$GOPATH go build -o bin/codis-config ./cmd/cconfig
# 	@rm -rf bin/assets && cp -r cmd/cconfig/assets bin/

build-server:
	@mkdir -p bin
	make -j4 -C extern/redis-2.8.21/
	@rm -f bin/codis-server
	@cp -f extern/redis-2.8.21/src/redis-server bin/codis-server

clean:
	@rm -rf bin

distclean: clean
	@make --no-print-directory --quiet -C extern/redis-2.8.21 clean

gotest: build-all
	GOPATH=`godep path`:$$GOPATH go test ./pkg/...

docker:
	docker build --force-rm -t codis-image .

.PHONY: docker
