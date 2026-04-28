# Agent Instructions

This repository is a Go-only rewrite of telegram-jung2-bot. Keep changes
simple, Go-native, and compatible with existing data and bot behaviour.

## Compatibility Rules

- Preserve DynamoDB table names, key names, attribute names, and value formats.
- Preserve Telegram command names, response text, ordering, and truncation behaviour.
- Preserve SQS action names and support both message attribute casings:
  `messageAttributes.action.stringValue` and
  `messageAttributes.action.StringValue`.
- Treat historical fixtures as compatibility references only; add or update Go
  tests for behaviour you touch.

## Domain Rules

- Never change the workday bitmask values:
  `Sun=1`, `Mon=2`, `Tue=4`, `Wed=8`, `Thu=16`, `Fri=32`, `Sat=64`.
- `MESSAGE_TABLE` uses `chatId` as the partition key and `dateCreated` as the
  sort key; message TTL is 7 days.
- `CHATID_TABLE` uses `chatId` as the partition key and stores `chatTitle`,
  `enableAllJung`, `offTime`, and `workday`.
- Always handle DynamoDB pagination with `LastEvaluatedKey`.
- Truncate generated Telegram reports at 3800 characters for UTF-8 safety.

## Go Guidance

- Use `context.Context` for network, AWS, and shutdown-aware operations.
- Wrap errors with useful context using `%w`.
- Use `log/slog` for structured logs.
- Avoid global mutable state except for startup configuration.

## Build and Test

- `make vendor` refreshes Go vendoring and generated Buck targets.
- `make build` builds the Go service with Buck2.
- `make test` runs Go tests with Buck2.
