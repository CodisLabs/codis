.DEFAULT_GOAL := build-all

export GO15VENDOREXPERIMENT=1

build-all: codis-server codis-dashboard codis-proxy codis-admin codis-ha codis-fe

codis-deps:
	@mkdir -p bin config && bash version
	@make --no-print-directory -C vendor/github.com/spinlock/jemalloc-go/

codis-dashboard: codis-deps
	go build -i -o bin/codis-dashboard ./cmd/dashboard
	@./bin/codis-dashboard --default-config > config/dashboard.toml

codis-proxy: codis-deps
	go build -i -o bin/codis-proxy ./cmd/proxy
	@./bin/codis-proxy --default-config > config/proxy.toml

codis-admin: codis-deps
	go build -i -o bin/codis-admin ./cmd/admin

codis-ha: codis-deps
	go build -i -o bin/codis-ha ./cmd/ha

codis-fe: codis-deps
	go build -i -o bin/codis-fe ./cmd/fe
	@rm -rf bin/assets; cp -rf cmd/fe/assets bin/

codis-server:
	@mkdir -p bin
	make -j4 -C extern/redis-2.8.21/
	@rm -f bin/codis-server*
	@cp -f extern/redis-2.8.21/src/redis-server bin/codis-server-2.8.21
	make -j4 -C extern/redis-3.2.4/
	@cp -f extern/redis-3.2.4/src/redis-server bin/codis-server
	@cp -f extern/redis-3.2.4/src/redis-benchmark bin/
	@cp -f extern/redis-3.2.4/src/redis-cli bin/

clean:
	@rm -rf bin
	@rm -rf scripts/tmp

distclean: clean
	@make --no-print-directory --quiet -C extern/redis-2.8.21 distclean
	@make --no-print-directory --quiet -C extern/redis-3.2.4  distclean
	@make --no-print-directory --quiet -C vendor/github.com/spinlock/jemalloc-go/ distclean

gotest: codis-deps
	go test ./cmd/... ./pkg/...

gobench: codis-deps
	go test -gcflags -l -bench=. -v ./pkg/...

docker:
	docker build --force-rm -t codis-image .

demo:
	pushd example && make
