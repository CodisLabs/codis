.DEFAULT_GOAL := build-all

export GO15VENDOREXPERIMENT=0

.PHONY: godep

GODEP :=

ifndef GODEP
GODEP := $(shell \
	if which godep 2>&1 >/dev/null; then \
		echo "godep"; \
	else \
		if [ ! -x "${GOPATH}/bin/godep" ]; then \
			go get -u github.com/tools/godep; \
		fi; \
		echo "${GOPATH}/bin/godep"; \
	fi;)
endif

build-all: codis-server codis-dashboard codis-proxy codis-admin codis-ha codis-fe

godep:
	@GOPATH=`${GODEP} path` ${GODEP} restore -v 2>&1 | while IFS= read -r line; do echo "  >>>> $${line}"; done
	@echo

codis-deps:
	@mkdir -p bin && bash version

codis-proxy: codis-deps
	${GODEP} go build -i -o bin/codis-proxy ./cmd/proxy

codis-admin: codis-deps
	${GODEP} go build -i -o bin/codis-admin ./cmd/admin

codis-dashboard: codis-deps
	${GODEP} go build -i -o bin/codis-dashboard ./cmd/dashboard

codis-ha: codis-deps
	${GODEP} go build -i -o bin/codis-ha ./cmd/ha

codis-fe: codis-deps
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

gotest: codis-deps
	${GODEP} go test ./pkg/...

docker:
	docker build --force-rm -t codis-image .
