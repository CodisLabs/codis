.DEFAULT_GOAL := build-all

export GO15VENDOREXPERIMENT=1
REDIS_VER="5.0"

build-all: codis-server codis-dashboard codis-proxy codis-admin codis-ha codis-fe clean-gotest

deps:
	@mkdir -p bin config && bash version

codis-deps: deps
	@make --no-print-directory -C vendor/github.com/spinlock/jemalloc-go/

codis-dashboard: codis-deps
	go build -i -o bin/codis-dashboard ./cmd/dashboard
	@./bin/codis-dashboard --default-config > config/dashboard.toml

codis-proxy: codis-deps
	go build -i -tags "cgo_jemalloc" -o bin/codis-proxy ./cmd/proxy
	@./bin/codis-proxy --default-config > config/proxy.toml

codis-admin: codis-deps
	go build -i -o bin/codis-admin ./cmd/admin

codis-ha: codis-deps
	go build -i -o bin/codis-ha ./cmd/ha

codis-fe: codis-deps
	go build -i -o bin/codis-fe ./cmd/fe
	@rm -rf bin/assets; cp -rf cmd/fe/assets bin/

codis-server: deps
	@rm -f bin/codis-server*
	make -j4 -C extern/redis-$(REDIS_VER)/
	@cp -f extern/redis-$(REDIS_VER)/src/redis-server  bin/codis-server
	@cp -f extern/redis-$(REDIS_VER)/src/redis-benchmark bin/
	@cp -f extern/redis-$(REDIS_VER)/src/redis-cli bin/
	@cp -f extern/redis-$(REDIS_VER)/src/redis-sentinel bin/
	@cp -f extern/redis-$(REDIS_VER)/redis.conf config/
	@sed -e "s/^sentinel/# sentinel/g" extern/redis-$(REDIS_VER)/sentinel.conf > config/sentinel.conf

clean-gotest:
	@rm -rf ./pkg/topom/gotest.tmp

clean: clean-gotest
	@rm -rf bin
	@rm -rf config
	@rm -rf scripts/tmp

distclean: clean
	@make --no-print-directory --quiet -C extern/redis-$(REDIS_VER) distclean
	@make --no-print-directory --quiet -C vendor/github.com/spinlock/jemalloc-go/ distclean

gotest: codis-deps
	go test ./cmd/... ./pkg/...

gobench: codis-deps
	go test -gcflags -l -bench=. -v ./pkg/...

docker:
	docker build --force-rm -t codis-image .

demo:
	pushd example && make
