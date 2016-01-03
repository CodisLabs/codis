.DEFAULT_GOAL := all

GODEP :=

ifndef GODEP
GODEP := $(shell \
	if ! command -v godep &> /dev/null; then \
		go get -u github.com/tools/godep; \
	fi; \
	echo 'godep';)
endif

all: codis-server codis-dashboard codis-proxy codis-admin codis-ha codis-fe

godep:
	@mkdir -p bin && bash version
	@GOPATH=`${GODEP} path` ${GODEP} restore -v 2>&1 | while IFS= read -r line; do echo "  **** $${line}"; done
	@echo

codis-proxy: godep
	${GODEP} go build -i -o bin/codis-proxy ./cmd/proxy

codis-admin: godep
	${GODEP} go build -i -o bin/codis-admin ./cmd/admin

codis-dashboard: godep
	${GODEP} go build -i -o bin/codis-dashboard ./cmd/dashboard

codis-ha: godep
	${GODEP} go build -i -o bin/codis-ha ./cmd/ha

codis-fe: godep
	${GODEP} go build -i -o bin/codis-fe ./cmd/fe
	@rm -rf bin/assets; cp -rf cmd/fe/assets bin/

codis-server:
	@mkdir -p bin
	make -j -C extern/redis-2.8.21/
	@rm -f bin/codis-server
	@cp -f extern/redis-2.8.21/src/redis-server bin/codis-server

clean:
	@rm -rf bin

distclean: clean
	@rm -rf Godeps/_workspace/pkg scripts/tmp
	@make --no-print-directory --quiet -C extern/redis-2.8.21 clean

gotest: godep
	${GODEP} go test ./pkg/...

docker:
	docker build --force-rm -t codis-image .
