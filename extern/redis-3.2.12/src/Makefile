# Redis Makefile
# Copyright (C) 2009 Salvatore Sanfilippo <antirez at gmail dot com>
# This file is released under the BSD license, see the COPYING file
#
# The Makefile composes the final FINAL_CFLAGS and FINAL_LDFLAGS using
# what is needed for Redis plus the standard CFLAGS and LDFLAGS passed.
# However when building the dependencies (Jemalloc, Lua, Hiredis, ...)
# CFLAGS and LDFLAGS are propagated to the dependencies, so to pass
# flags only to be used when compiling / linking Redis itself REDIS_CFLAGS
# and REDIS_LDFLAGS are used instead (this is the case of 'make gcov').
#
# Dependencies are stored in the Makefile.dep file. To rebuild this file
# Just use 'make dep', but this is only needed by developers.

release_hdr := $(shell sh -c './mkreleasehdr.sh')
uname_S := $(shell sh -c 'uname -s 2>/dev/null || echo not')
OPTIMIZATION?=-O2
DEPENDENCY_TARGETS=hiredis linenoise lua geohash-int

# Default settings
STD=-std=c99 -pedantic -DREDIS_STATIC=''
WARN=-Wall -W
OPT=$(OPTIMIZATION)

PREFIX?=/usr/local
INSTALL_BIN=$(PREFIX)/bin
INSTALL=install

# Default allocator
ifeq ($(uname_S),Linux)
	MALLOC=jemalloc
else
	MALLOC=libc
endif

# Backwards compatibility for selecting an allocator
ifeq ($(USE_TCMALLOC),yes)
	MALLOC=tcmalloc
endif

ifeq ($(USE_TCMALLOC_MINIMAL),yes)
	MALLOC=tcmalloc_minimal
endif

ifeq ($(USE_JEMALLOC),yes)
	MALLOC=jemalloc
endif

ifeq ($(USE_JEMALLOC),no)
	MALLOC=libc
endif

# Override default settings if possible
-include .make-settings

FINAL_CFLAGS=$(STD) $(WARN) $(OPT) $(DEBUG) $(CFLAGS) $(REDIS_CFLAGS) -I../deps/geohash-int
FINAL_LDFLAGS=$(LDFLAGS) $(REDIS_LDFLAGS) $(DEBUG)
FINAL_LIBS=-lm
DEBUG=-g -ggdb

ifeq ($(uname_S),SunOS)
	# SunOS
	INSTALL=cp -pf
	FINAL_CFLAGS+= -D__EXTENSIONS__ -D_XPG6
	FINAL_LIBS+= -ldl -lnsl -lsocket -lresolv -lpthread -lrt
else
ifeq ($(uname_S),Darwin)
	# Darwin
	FINAL_LIBS+= -ldl
else
ifeq ($(uname_S),AIX)
        # AIX
        FINAL_LDFLAGS+= -Wl,-bexpall
        FINAL_LIBS+=-ldl -pthread -lcrypt -lbsd
else
ifeq ($(uname_S),OpenBSD)
	# OpenBSD
	FINAL_LIBS+= -lpthread
else
ifeq ($(uname_S),FreeBSD)
	# FreeBSD
	FINAL_LIBS+= -lpthread
else
	# All the other OSes (notably Linux)
	FINAL_LDFLAGS+= -rdynamic
	FINAL_LIBS+=-ldl -pthread
endif
endif
endif
endif
endif
# Include paths to dependencies
FINAL_CFLAGS+= -I../deps/hiredis -I../deps/linenoise -I../deps/lua/src

ifeq ($(MALLOC),tcmalloc)
	FINAL_CFLAGS+= -DUSE_TCMALLOC
	FINAL_LIBS+= -ltcmalloc
endif

ifeq ($(MALLOC),tcmalloc_minimal)
	FINAL_CFLAGS+= -DUSE_TCMALLOC
	FINAL_LIBS+= -ltcmalloc_minimal
endif

ifeq ($(MALLOC),jemalloc)
	DEPENDENCY_TARGETS+= jemalloc
	FINAL_CFLAGS+= -DUSE_JEMALLOC -I../deps/jemalloc/include
	FINAL_LIBS+= ../deps/jemalloc/lib/libjemalloc.a
endif

REDIS_CC=$(QUIET_CC)$(CC) $(FINAL_CFLAGS)
REDIS_LD=$(QUIET_LINK)$(CC) $(FINAL_LDFLAGS)
REDIS_INSTALL=$(QUIET_INSTALL)$(INSTALL)

CCCOLOR="\033[34m"
LINKCOLOR="\033[34;1m"
SRCCOLOR="\033[33m"
BINCOLOR="\033[37;1m"
MAKECOLOR="\033[32;1m"
ENDCOLOR="\033[0m"

ifndef V
QUIET_CC = @printf '    %b %b\n' $(CCCOLOR)CC$(ENDCOLOR) $(SRCCOLOR)$@$(ENDCOLOR) 1>&2;
QUIET_LINK = @printf '    %b %b\n' $(LINKCOLOR)LINK$(ENDCOLOR) $(BINCOLOR)$@$(ENDCOLOR) 1>&2;
QUIET_INSTALL = @printf '    %b %b\n' $(LINKCOLOR)INSTALL$(ENDCOLOR) $(BINCOLOR)$@$(ENDCOLOR) 1>&2;
endif

REDIS_SERVER_NAME=redis-server
REDIS_SENTINEL_NAME=redis-sentinel
REDIS_SERVER_OBJ=adlist.o quicklist.o ae.o anet.o dict.o server.o sds.o zmalloc.o lzf_c.o lzf_d.o pqsort.o zipmap.o sha1.o ziplist.o release.o networking.o util.o object.o db.o replication.o rdb.o t_string.o t_list.o t_set.o t_zset.o t_hash.o config.o aof.o pubsub.o multi.o debug.o sort.o intset.o syncio.o cluster.o crc16.o endianconv.o slowlog.o scripting.o bio.o rio.o rand.o memtest.o crc64.o bitops.o sentinel.o notify.o setproctitle.o blocked.o hyperloglog.o latency.o sparkline.o redis-check-rdb.o geo.o
REDIS_GEOHASH_OBJ=../deps/geohash-int/geohash.o ../deps/geohash-int/geohash_helper.o
REDIS_CLI_NAME=redis-cli
REDIS_CLI_OBJ=anet.o adlist.o redis-cli.o zmalloc.o release.o anet.o ae.o crc64.o
REDIS_BENCHMARK_NAME=redis-benchmark
REDIS_BENCHMARK_OBJ=ae.o anet.o redis-benchmark.o adlist.o zmalloc.o redis-benchmark.o
REDIS_CHECK_RDB_NAME=redis-check-rdb
REDIS_CHECK_AOF_NAME=redis-check-aof
REDIS_CHECK_AOF_OBJ=redis-check-aof.o

all: $(REDIS_SERVER_NAME) $(REDIS_SENTINEL_NAME) $(REDIS_CLI_NAME) $(REDIS_BENCHMARK_NAME) $(REDIS_CHECK_RDB_NAME) $(REDIS_CHECK_AOF_NAME)
	@echo ""
	@echo "Hint: It's a good idea to run 'make test' ;)"
	@echo ""

.PHONY: all

# Deps (use make dep to generate this)
include Makefile.dep

dep:
	$(REDIS_CC) -MM *.c > Makefile.dep

.PHONY: dep

persist-settings: distclean
	echo STD=$(STD) >> .make-settings
	echo WARN=$(WARN) >> .make-settings
	echo OPT=$(OPT) >> .make-settings
	echo MALLOC=$(MALLOC) >> .make-settings
	echo CFLAGS=$(CFLAGS) >> .make-settings
	echo LDFLAGS=$(LDFLAGS) >> .make-settings
	echo REDIS_CFLAGS=$(REDIS_CFLAGS) >> .make-settings
	echo REDIS_LDFLAGS=$(REDIS_LDFLAGS) >> .make-settings
	echo PREV_FINAL_CFLAGS=$(FINAL_CFLAGS) >> .make-settings
	echo PREV_FINAL_LDFLAGS=$(FINAL_LDFLAGS) >> .make-settings
	-(cd ../deps && $(MAKE) $(DEPENDENCY_TARGETS))

.PHONY: persist-settings

# Prerequisites target
.make-prerequisites:
	@touch $@

# Clean everything, persist settings and build dependencies if anything changed
ifneq ($(strip $(PREV_FINAL_CFLAGS)), $(strip $(FINAL_CFLAGS)))
.make-prerequisites: persist-settings
endif

ifneq ($(strip $(PREV_FINAL_LDFLAGS)), $(strip $(FINAL_LDFLAGS)))
.make-prerequisites: persist-settings
endif

# redis-server
$(REDIS_SERVER_NAME): $(REDIS_SERVER_OBJ)
	$(REDIS_LD) -o $@ $^ ../deps/hiredis/libhiredis.a ../deps/lua/src/liblua.a $(REDIS_GEOHASH_OBJ) $(FINAL_LIBS)

# redis-sentinel
$(REDIS_SENTINEL_NAME): $(REDIS_SERVER_NAME)
	$(REDIS_INSTALL) $(REDIS_SERVER_NAME) $(REDIS_SENTINEL_NAME)

# redis-check-rdb
$(REDIS_CHECK_RDB_NAME): $(REDIS_SERVER_NAME)
	$(REDIS_INSTALL) $(REDIS_SERVER_NAME) $(REDIS_CHECK_RDB_NAME)

# redis-cli
$(REDIS_CLI_NAME): $(REDIS_CLI_OBJ)
	$(REDIS_LD) -o $@ $^ ../deps/hiredis/libhiredis.a ../deps/linenoise/linenoise.o $(FINAL_LIBS)

# redis-benchmark
$(REDIS_BENCHMARK_NAME): $(REDIS_BENCHMARK_OBJ)
	$(REDIS_LD) -o $@ $^ ../deps/hiredis/libhiredis.a $(FINAL_LIBS)

# redis-check-aof
$(REDIS_CHECK_AOF_NAME): $(REDIS_CHECK_AOF_OBJ)
	$(REDIS_LD) -o $@ $^ $(FINAL_LIBS)

# Because the jemalloc.h header is generated as a part of the jemalloc build,
# building it should complete before building any other object. Instead of
# depending on a single artifact, build all dependencies first.
%.o: %.c .make-prerequisites
	$(REDIS_CC) -c $<

clean:
	rm -rf $(REDIS_SERVER_NAME) $(REDIS_SENTINEL_NAME) $(REDIS_CLI_NAME) $(REDIS_BENCHMARK_NAME) $(REDIS_CHECK_RDB_NAME) $(REDIS_CHECK_AOF_NAME) *.o *.gcda *.gcno *.gcov redis.info lcov-html

.PHONY: clean

distclean: clean
	-(cd ../deps && $(MAKE) distclean)
	-(rm -f .make-*)

.PHONY: distclean

test: $(REDIS_SERVER_NAME) $(REDIS_CHECK_AOF_NAME)
	@(cd ..; ./runtest)

test-sentinel: $(REDIS_SENTINEL_NAME)
	@(cd ..; ./runtest-sentinel)

check: test

lcov:
	$(MAKE) gcov
	@(set -e; cd ..; ./runtest --clients 1)
	@geninfo -o redis.info .
	@genhtml --legend -o lcov-html redis.info

test-sds: sds.c sds.h
	$(REDIS_CC) sds.c zmalloc.c -DSDS_TEST_MAIN -o /tmp/sds_test
	/tmp/sds_test

.PHONY: lcov

bench: $(REDIS_BENCHMARK_NAME)
	./$(REDIS_BENCHMARK_NAME)

32bit:
	@echo ""
	@echo "WARNING: if it fails under Linux you probably need to install libc6-dev-i386"
	@echo ""
	$(MAKE) CFLAGS="-m32" LDFLAGS="-m32"

gcov:
	$(MAKE) REDIS_CFLAGS="-fprofile-arcs -ftest-coverage -DCOVERAGE_TEST" REDIS_LDFLAGS="-fprofile-arcs -ftest-coverage"

noopt:
	$(MAKE) OPTIMIZATION="-O0"

valgrind:
	$(MAKE) OPTIMIZATION="-O0" MALLOC="libc"

src/help.h:
	@../utils/generate-command-help.rb > help.h

install: all
	@mkdir -p $(INSTALL_BIN)
	$(REDIS_INSTALL) $(REDIS_SERVER_NAME) $(INSTALL_BIN)
	$(REDIS_INSTALL) $(REDIS_BENCHMARK_NAME) $(INSTALL_BIN)
	$(REDIS_INSTALL) $(REDIS_CLI_NAME) $(INSTALL_BIN)
	$(REDIS_INSTALL) $(REDIS_CHECK_RDB_NAME) $(INSTALL_BIN)
	$(REDIS_INSTALL) $(REDIS_CHECK_AOF_NAME) $(INSTALL_BIN)
	@ln -sf $(REDIS_SERVER_NAME) $(INSTALL_BIN)/$(REDIS_SENTINEL_NAME)
