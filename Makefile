.PHONY: build coverage test integration ci vendor lint lint-fix mock install-buck2 clean

TEST_TARGETS := //...
TEST_MODIFIERS := -m toolchains//:race
SLOW_TEST_TARGETS := //internal/integration:floci_integration_test

build:
	buck2 build //...

test: lint
	buck2 test --exclude slow $(TEST_MODIFIERS) $(TEST_TARGETS)

coverage: test
	COVERAGE_TEST_TARGETS="$$(buck2 uquery "kind('go_test', $(TEST_TARGETS))" | sort | grep -v '^root//internal/integration:floci_integration_test$$' | tr '\n' ' ')" \
	COVERAGE_TEST_MODIFIERS="$(TEST_MODIFIERS) -m prelude//go/constraints:coverage_mode[atomic]" \
	./hack/test-coverage.sh

integration:
	buck2 test $(TEST_MODIFIERS) $(SLOW_TEST_TARGETS) -- --env INTEGRATION_TESTS=1

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
	rm -f internal/mock/*_mock.go internal/mock/httpserver/*_mock.go
	go generate ./internal/app ./internal/dynamodb ./internal/httpserver ./internal/queue

install-buck2:
	./hack/install-buck2.sh

clean:
	buck2 clean
