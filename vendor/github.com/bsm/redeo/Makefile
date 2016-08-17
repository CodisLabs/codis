default: test

testdeps: deps
	@go get github.com/onsi/ginkgo
	@go get github.com/onsi/gomega

test: testdeps
	@go test ./...

.PHONY: test

bench: testdeps
	@go test --bench=.

.PHONY: bench

deps:
	@go get


