# telegram-jung2-bot

Telegram group chat statistics bot. Tracks message counts, produces rankings, and schedules off-work reports.

## Architecture

- Go owns the HTTP webhook, SQS polling, Telegram HTTP client, DynamoDB access, command routing, statistics, settings, and report formatting.
- EventBridge Scheduler enqueues scheduled actions into SQS.
- Historical production contracts are preserved by focused Go tests.
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
│   ├── integration/
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
- Docker, for the optional Floci-backed AWS integration check.

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

Runs the fast Buck2 test set with the race detector enabled. This excludes Buck
test targets labelled `slow`, does not refresh vendoring, and runs `make lint`
first.

```bash
make coverage
```

Runs the Buck-built Go coverage check and fails unless the packages included by
`hack/test-coverage.sh` have 100% statement coverage. It reuses the same fast
Buck test target set and race mode as `make test`.

```bash
make integration
```

Starts a temporary [Floci](https://github.com/floci-io/floci) container through
Testcontainers-Go, creates local DynamoDB tables and an SQS queue, then runs the
slow Buck `go_test` target against the real AWS SDK adapters with
`INTEGRATION_TESTS=1`. It round-trips chat/message DynamoDB rows and every queue action
shape: all Telegram command actions plus the scheduled `onOffFromWork` and
`offFromWork` actions. Set `FLOCI_ENDPOINT` to use an already-running
Floci-compatible endpoint instead of launching Docker.

The integration target is labelled `slow`, so `make test` and `make coverage`
do not start Docker. The test also skips unless `INTEGRATION_TESTS=1`; `make
integration` passes that environment variable through Buck.

Supported integration environment variables:

| Variable               | Purpose                                                            |
|------------------------|--------------------------------------------------------------------|
| `FLOCI_ENDPOINT`       | Reuse an existing Floci-compatible AWS endpoint.                   |
| `FLOCI_CONTAINER_NAME` | Override the default `telegram-jung2-bot-it-floci` container name. |
| `FLOCI_IMAGE`          | Override the default `floci/floci:latest` container image.         |
| `AWS_REGION`           | Override the local AWS SDK region, defaulting to `eu-west-1`.      |

```bash
make lint
```

Runs Go, shell, spelling, and Markdown lint checks.

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
dependency changes. `gobuckify` generates only vendored third-party Buck
targets in this repository; first-party `BUCK` dependencies under `cmd/` and
`internal/` are maintained manually when Go imports change.

```bash
make clean
```

Cleans Buck2 build outputs.
