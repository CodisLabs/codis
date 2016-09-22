.DEFAULT_GOAL := build-all

export GO15VENDOREXPERIMENT=1

build-all: codis-server codis-dashboard codis-proxy codis-admin codis-ha codis-fe

codis-deps:
	@mkdir -p bin && bash version

codis-proxy: codis-deps
	go build -i -o bin/codis-proxy ./cmd/proxy

codis-admin: codis-deps
	go build -i -o bin/codis-admin ./cmd/admin

codis-dashboard: codis-deps
	go build -i -o bin/codis-dashboard ./cmd/dashboard

codis-ha: codis-deps
	go build -i -o bin/codis-ha ./cmd/ha

codis-fe: codis-deps
	go build -i -o bin/codis-fe ./cmd/fe
	@rm -rf bin/assets; cp -rf cmd/fe/assets bin/

codis-server:
	@mkdir -p bin
	make -j -C extern/redis-2.8.21/
	@rm -f bin/codis-server
	@cp -f extern/redis-2.8.21/src/redis-server bin/codis-server

clean:
	@rm -rf bin
	@rm -rf scripts/tmp

distclean: clean
	@make --no-print-directory --quiet -C extern/redis-2.8.21 distclean

gotest: codis-deps
	go test ./pkg/...

docker:
	docker build --force-rm -t codis-image .
