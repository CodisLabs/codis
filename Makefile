.PHONY:    codis-server codis-dashboard codis-proxy codis-admin codis-fe docker

build-all: codis-server codis-dashboard codis-proxy codis-admin codis-fe

godep-env:
	@bash version
	@command -v godep 2>&1 >/dev/null || go get -u github.com/tools/godep
	@GOPATH=`godep path` godep restore

codis-proxy: godep-env
	godep go build -i -o bin/codis-proxy ./cmd/proxy

codis-admin: godep-env
	godep go build -i -o bin/codis-admin ./cmd/admin

codis-dashboard: godep-env
	godep go build -i -o bin/codis-dashboard ./cmd/dashboard

codis-fe: godep-env
	godep go build -i -o bin/codis-fe ./cmd/fe
	@rm -rf bin/assets; cp -rf cmd/fe/assets bin/

codis-server:
	@mkdir -p bin
	make -j4 -C extern/redis-2.8.21/
	@rm -f bin/codis-server
	@cp -f extern/redis-2.8.21/src/redis-server bin/codis-server

clean:
	@rm -rf bin

distclean: clean
	@rm -rf Godeps/_workspace/pkg
	@rm -rf scripts/tmp test/tmp
	@make --no-print-directory --quiet -C extern/redis-2.8.21 clean

gotest: build-all
	godep go test ./pkg/...

docker:
	docker build --force-rm -t codis-image .
