.PHONY: build test test-coverage vendor lint lint-fix install-buck2 check-gobuckify clean

build: vendor
	buck2 build //...

test: vendor
	buck2 test -m 'toolchains//:race' //...
	./hack/test-coverage.sh

test-coverage:
	./hack/test-coverage.sh

vendor:
	buck2 clean
	go mod vendor
	buck2 run prelude//go/tools/gobuckify:gobuckify -- .

lint:
	./hack/lint.sh

lint-fix:
	./hack/lint.sh fix

install-buck2:
	./hack/install-buck2.sh

check-gobuckify:
	buck2 run prelude//go/tools/gobuckify:gobuckify -- .

clean:
	buck2 clean
