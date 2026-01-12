.PHONY: build test

build:
	buck2 build //rust:core
	buck2 build //rust:cbindgen_headers

test:
	buck2 test //...
