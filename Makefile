all: build
	@tar -cf deploy.tar bin sample

build: build-proxy build-config build-server

build-proxy:
	go build -o bin/codis-proxy ./cmd/proxy

build-config:
	go build -o bin/codis-config ./cmd/cconfig
	@rm -rf bin/assets && cp -r cmd/cconfig/assets bin/

build-server:
	@mkdir -p bin
	make -j4 -C ext/redis-2.8.13/
	@cp -f ext/redis-2.8.13/src/redis-server bin/codis-server

clean:
	@rm -rf bin
	@rm -f *.rdb *.out *.log *.dump deploy.tar
	@rm -f Dockerfile ext/Dockerfile
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

distclean: clean
	@make --no-print-directory --quiet -C ext/redis-2.8.13 clean

gotest:
	go test ./... -race
