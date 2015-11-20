.PHONY:    codis-server codis-dashboard codis-proxy codis-admin docker

build-all: codis-server codis-dashboard codis-proxy codis-admin

godep-env:
	@bash version
	@which godep &>/dev/null || go get -u github.com/tools/godep
	@GOPATH=`godep path` godep restore

codis-proxy: godep-env
	godep go build -i -o bin/codis-proxy ./cmd/proxy

codis-admin: godep-env
	godep go build -i -o bin/codis-admin ./cmd/admin

codis-dashboard: godep-env
	godep go build -i -o bin/codis-dashboard ./cmd/dashboard

codis-server:
	@mkdir -p bin
	make -j4 -C extern/redis-2.8.21/
	@rm -f bin/codis-server
	@cp -f extern/redis-2.8.21/src/redis-server bin/codis-server

clean:
	@rm -rf bin

distclean: clean
	@rm -rf Godeps/_workspace/pkg
	@make --no-print-directory --quiet -C extern/redis-2.8.21 clean

gotest: build-all
	godep go test ./pkg/...

docker:
	docker build --force-rm -t codis-image .
