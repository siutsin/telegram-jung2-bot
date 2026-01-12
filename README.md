# telegram-jung2-bot

Telegram group chat statistics bot. Tracks message counts, produces rankings, and schedules off-work reports.

## Architecture

- Go handles HTTP, SQS polling, and Telegram HTTP calls.
- Rust owns all business logic and executes DynamoDB calls via the AWS Rust SDK.
- EventBridge Scheduler enqueues scheduled actions into SQS.

## Build and Run

Build the Rust core library and headers:

```
make build
```

## Testing

Run all tests:

```
make test
```
