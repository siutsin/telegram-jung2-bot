# telegram-jung2-bot

Telegram group chat statistics bot. Tracks message counts, produces rankings, and schedules off-work reports.

## Architecture

- Go owns the HTTP webhook, SQS polling, Telegram HTTP client, DynamoDB access, command routing, statistics, settings, and report formatting.
- EventBridge Scheduler enqueues scheduled actions into SQS.
- Historical fixtures may still exist for compatibility reference, but new runtime code belongs under `go/`.

## Prerequisites

- [Buck2](https://buck2.build/docs/getting_started/)
- Go 1.26+

## Build

```
make build
```

Builds the Go service with Buck2.

## Testing

```
make test
```

Runs Go tests through Buck2.
