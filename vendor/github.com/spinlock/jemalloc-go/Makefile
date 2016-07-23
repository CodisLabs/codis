.DEFAULT_GOAL = all

all:
	@test -f jemalloc/Makefile || make config --quiet

config:
	cd jemalloc && ./autogen.sh --with-jemalloc-prefix="je_"
	@make -f help.mk --quiet relink

clean distclean:
	@test -f jemalloc/Makefile && make -C jemalloc --quiet distclean || true
	@make -f help.mk --quiet rmlink

install: all
	go install ./
