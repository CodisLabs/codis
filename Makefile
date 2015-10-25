all: build

build: build-version build-server build-proxy build-admin build-dashboard

build-version:
	@bash genver.sh

build-proxy:
	go build -o bin/codis-proxy ./cmd/proxy

build-admin:
	go build -o bin/codis-admin ./cmd/admin

build-dashboard:
	go build -o bin/codis-dashboard ./cmd/dashboard

# build-config:
# 	go build -o bin/codis-config ./cmd/cconfig
# 	@rm -rf bin/assets && cp -r cmd/cconfig/assets bin/

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
	go test ./pkg/... ./cmd/...
