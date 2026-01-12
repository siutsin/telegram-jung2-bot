.PHONY: build test vendor clean

build: vendor
	buck2 build //...

test: vendor
	buck2 test //...

vendor:
	go work vendor
	go run ./tools/buckify

clean:
	buck2 clean
