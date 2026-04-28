.PHONY: build test vendor clean

build: vendor
	buck2 build //go/...

test: vendor
	buck2 test //go/...

vendor:
	cd go && go mod vendor
	cd go && go run ./cmd/buckify

clean:
	buck2 clean
