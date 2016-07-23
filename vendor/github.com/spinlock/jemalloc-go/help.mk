.DEFAULT_GOAL = relink

-include jemalloc/Makefile

relink: rmlink
	@for i in $(C_SRCS); do \
		rm -f              je_$$(basename $$i); \
		ln -s jemalloc/$$i je_$$(basename $$i); \
	done

rmlink:
	@rm -f je_*.c
