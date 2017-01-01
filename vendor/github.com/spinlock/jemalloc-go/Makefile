.DEFAULT_GOAL = build

PWD := $(shell pwd)
SRC := jemalloc-4.4.0

build:
	@test -f $(SRC)/Makefile || make config --quiet

install: build
	@go install -x -v ./

config:
	@cd $(SRC) && ./autogen.sh --with-jemalloc-prefix="je_"
	@rm -rf jemalloc VERSION
	@ln -s $(SRC)/include/jemalloc
	@ln -s $(SRC)/VERSION
	@make -f help.mk relink

clean distclean:
	@test -f $(SRC)/Makefile && make -C $(SRC) --quiet distclean || true
	@rm -rf jemalloc VERSION
	@make -f help.mk unlink

relink unlink:
	@make -f help.mk $@

test:
	@go test -v ./
