.PHONY: all install test vet

all: test vet

install:
	go install

test:
	go test

vet:
	go vet
	golint .
