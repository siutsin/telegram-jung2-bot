.PHONY: build coverage test ci vendor lint lint-fix lint-bin mock install-buck2 clean

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

lint: lint-bin
	GOLANGCI_LINT=./bin/golangci-lint-custom ./hack/lint.sh

lint-fix: lint-bin
	GOLANGCI_LINT=./bin/golangci-lint-custom ./hack/lint.sh fix

lint-bin:
	golangci-lint custom

mock:
	rm -f internal/mock/*_mock.go internal/mock/httpserver/*_mock.go
	go generate ./internal/app ./internal/dynamodb ./internal/httpserver ./internal/queue

install-buck2:
	./hack/install-buck2.sh

clean:
	buck2 clean
