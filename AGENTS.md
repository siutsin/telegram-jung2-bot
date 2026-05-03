# Agent Instructions

This repository is the Go service for `telegram-jung2-bot`. Keep the root
clean, Go-native, and contract-first. Migration-only reference material may be
used to verify behaviour until production adapter parity is complete.

## Working Rules

- Work on independent modules first; defer app wiring until package contracts
  and parity tests are stable.
- Do not create a `go/` subdirectory. The service executable lives under
  `cmd/`; private Go packages live under `internal/`.
- Keep Buck target visibility explicit even though the Go `internal` import
  rule also prevents imports from outside this module tree.
- Do not add shared `testutil`, shared mocks, or abstraction packages until
  repeated code proves they are needed.
- Keep domain packages free of AWS SDK clients, Telegram HTTP clients, HTTP
  server code, environment readers, and app wiring.
- Use package-local interfaces, fakes, or generated mocks. Use
  `go.uber.org/mock` for generated mocks when hand-written fakes become noisy.

## Contract Rules

- Preserve DynamoDB table names, key names, attribute names, and value formats.
- Preserve Telegram command names, aliases, response text, ordering, and
  truncation behaviour.
- Preserve SQS action names and support both message attribute casings:
  `messageAttributes.action.stringValue` and
  `messageAttributes.action.StringValue`.
- Preserve existing `dateCreated` parsing for the stored UTC+8 offset format
  before normalising time internally.
- Treat migration fixtures as contract references only; add or update Go tests
  for behaviour you touch.
- Every implementation change should identify which contract test case
  or fixture it replicates.

## Domain Rules

- Never change the workday bitmask values:
  `Sun=1`, `Mon=2`, `Tue=4`, `Wed=8`, `Thu=16`, `Fri=32`, `Sat=64`.
- `MESSAGE_TABLE` uses `chatId` as the partition key and `dateCreated` as the
  sort key; message TTL is 7 days.
- `CHATID_TABLE` uses `chatId` as the partition key and stores `chatTitle`,
  `enableAllJung`, `offTime`, and `workday`.
- Always handle DynamoDB pagination with `LastEvaluatedKey`.
- Truncate generated Telegram reports at 3800 characters after rendering final
  text, and keep truncation UTF-8 safe.
- Avoid logging unnecessary Telegram message text.

## Go Guidance

- Use `context.Context` for network, AWS, Telegram, and shutdown-aware
  operations.
- Wrap errors with useful context using `%w`.
- Use `log/slog` for structured logs.
- Avoid global mutable state except for startup configuration.
- Keep identifiers unexported unless they are needed outside their own
  package.
- Keep package APIs small and stable before depending on them from other
  packages.
- When a function transforms, normalises, validates, or derives data, add a
  short docstring example that shows the input and output shape.

## Build, Test, And Lint

- `make vendor` refreshes Go vendoring and generated Buck targets.
- `make build` builds the Go service with Buck2.
- `make ci` runs the full CI gate in order: `vendor`, then `make coverage`
  (which runs `make test`, and therefore `make lint`, before collecting
  coverage).
- Use `make test` for Buck2 test execution and `make coverage` for the Buck-built
  coverage gate.
- Do not invoke native `go test` directly for validation.
- `make test` runs `make lint` first.
- `make test` runs Buck2 tests with the race detector enabled.
- `make coverage` runs only the Buck-built atomic Go coverage gate; coverage
  must remain 100% for `internal/` packages, and `cmd/` entrypoints are
  excluded from the coverage gate.
- `make coverage` reuses the same Buck test target set and race mode as
  `make test`; test selection lives in the `Makefile`, not the coverage script.
- `make build` and `make test` do not refresh vendoring; run `make vendor`
  explicitly after dependency changes.
- `make lint` runs `gofmt` checks, `go vet`, `golangci-lint`, `shellcheck`,
  `typos`, and `markdownlint-cli2`.
- `make lint-fix` applies supported formatting/lint fixes.
- `make install-buck2` installs or updates Buck2 for the local platform.
- Use Buck2's official `prelude//go/tools/gobuckify:gobuckify` target for Go
  vendor BUCK generation through `make vendor`; do not reintroduce a custom
  generator.
