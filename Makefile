.PHONY: build coverage test ci vendor lint lint-fix mock install-buck2 clean

TEST_TARGETS := //...
TEST_MODIFIERS := -m toolchains//:race

build:
	buck2 build //...

test: lint
	buck2 test $(TEST_MODIFIERS) $(TEST_TARGETS)

coverage: test
	COVERAGE_TEST_TARGETS="$$(buck2 uquery "kind('go_test', $(TEST_TARGETS))" | sort | tr '\n' ' ')" \
	COVERAGE_TEST_MODIFIERS="$(TEST_MODIFIERS) -m prelude//go/constraints:coverage_mode[atomic]" \
	./hack/test-coverage.sh

ci: vendor
	$(MAKE) coverage

vendor: clean
	go mod vendor
	buck2 run prelude//go/tools/gobuckify:gobuckify -- .

lint:
	./hack/lint.sh

lint-fix:
	./hack/lint.sh fix

mock:
	rm -f internal/mock/*_mock.go
	go generate ./internal/app ./internal/dynamodb ./internal/httpserver

install-buck2:
	./hack/install-buck2.sh

clean:
	buck2 clean
