# telegram-jung2-bot

Telegram group chat statistics bot. Tracks message counts, produces rankings, and schedules off-work reports.

## Architecture

- Go owns the HTTP webhook, SQS polling, Telegram HTTP client, DynamoDB access, command routing, statistics, settings, and report formatting.
- EventBridge Scheduler enqueues scheduled actions into SQS.
- Migration-only reference material is temporary and must be removed before the
  project is considered a standalone Go service.
- The service executable lives under `cmd/telegram-jung2-bot`; private Go packages live under `internal/`.
- Buck2 targets control build visibility.
- Buck2 builds, tests, and vendors the service.

## Layout

```text
.
├── cmd/
│   └── telegram-jung2-bot/
│       ├── BUCK
│       └── main.go
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

Builds the Go service with Buck2.

```bash
make test
```

Runs Buck2 tests, then runs a Buck-built Go coverage check. Coverage must stay
at 100%.

```bash
make test-coverage
```

Runs the Buck-built Go coverage check and fails unless total statement coverage
is 100%.

```bash
make lint
```

Runs Go and Markdown lint checks.

```bash
make lint-fix
```

Applies supported lint fixes.

```bash
make vendor
```

Refreshes Go vendoring and generated vendor `BUCK` files.

```bash
make check-gobuckify
```

Regenerates Buck targets with the official Buck2 `gobuckify` tool.

```bash
make clean
```

Cleans Buck2 build outputs.
