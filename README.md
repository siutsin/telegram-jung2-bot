# telegram-jung2-bot

Telegram group chat statistics bot. Tracks message counts, produces rankings, and schedules off-work reports.

## Architecture

- Go owns the HTTP webhook, SQS polling, Telegram HTTP client, DynamoDB access, command routing, statistics, settings, and report formatting.
- EventBridge Scheduler enqueues scheduled actions into SQS.
- Migration-only reference material is temporary and must be removed before the
  project is considered a standalone Go service.
- The service executable lives under `cmd/`; private Go packages live under `internal/`.
- Startup wiring lives in `cmd/main.go`; production adapters are split across
  focused internal packages instead of a single runtime package.
- Buck2 targets control build visibility.
- Buck2 builds and tests the service. Vendoring is refreshed explicitly.

## Layout

```text
.
├── cmd/
│   ├── BUCK
│   └── main.go
├── internal/
│   ├── app/
│   ├── chat/
│   ├── command/
│   ├── config/
│   ├── dynamodb/
│   ├── httpserver/
│   ├── message/
│   ├── queue/
│   ├── schedule/
│   ├── service/
│   ├── statistics/
│   ├── telegram/
│   ├── worker/
│   └── workday/
└── vendor/
```

## Prerequisites

- [Buck2](https://buck2.build/docs/getting_started/)
- Go 1.26+

## Commands

```bash
make install-buck2
```

Installs or upgrades Buck2 from the latest pre-built release.

```bash
make build
```

Builds the Go service with Buck2. This does not refresh vendoring.

```bash
make ci
```

Runs the full CI gate in order: `make vendor`, then `make coverage`. Since
`make coverage` depends on `make test`, and `make test` depends on `make lint`,
the effective sequence is vendoring, lint, race-enabled tests, then coverage
collection.

```bash
make test
```

Runs Buck2 tests with the race detector enabled. This does not refresh
vendoring. `make lint` runs first.

```bash
make coverage
```

Runs the Buck-built Go coverage check and fails unless the packages included by
`hack/test-coverage.sh` have 100% statement coverage. It reuses the same Buck
test target set and race mode as `make test`.

```bash
make lint
```

Builds the repo's custom golangci-lint binary, then runs Go, shell, spelling,
and Markdown lint checks.

```bash
make lint-fix
```

Applies supported lint fixes.

```bash
make mock
```

Removes old generated mocks, then regenerates centralised GoMock code under
`internal/mock/` via `go generate`.

```bash
make vendor
```

Refreshes Go vendoring and generated vendor `BUCK` files. Run this after
dependency changes.

```bash
make clean
```

Cleans Buck2 build outputs.
