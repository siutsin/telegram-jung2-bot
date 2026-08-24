# Internal packages

`internal` is the service's flat package for small, closely related code.
Its Go package name is `bot`.

| File             | Responsibility                                      |
|------------------|-----------------------------------------------------|
| `app.go`         | Starts and stops the HTTP servers and queue worker. |
| `chat.go`        | Defines chat records and update requests.           |
| `command.go`     | Parses Telegram commands into queue actions.        |
| `config.go`      | Loads and validates startup settings.               |
| `message.go`     | Defines stored messages and date formats.           |
| `schedule.go`    | Calculates schedules and admin setting changes.     |
| `service.go`     | Runs the bot actions used by the worker.            |
| `statistics.go`  | Renders Telegram reports.                           |
| `telegram.go`    | Provides Telegram types and API client calls.       |
| `workday.go`     | Defines the stored workday bitmask.                 |
| `worker.go`      | Polls and dispatches queue actions.                 |

The package uses no AWS SDK types. `queue` is its only service adapter
dependency.

## Larger packages

These packages stay separate because each has several production files and a
clear adapter boundary.

- `internal/dynamodb` maps the domain contracts to DynamoDB.
- `internal/httpserver` owns HTTP routes, webhook handling, and readiness.
- `internal/queue` maps queue contracts to SQS.
- `internal/integration` tests the service against an isolated Floci stack.

## Important contracts

- `dateCreated` keeps its existing UTC+8 storage format.
- Message TTL remains seven days.
- Workday bits remain `Sun=1` through `Sat=64`.
- Telegram reports are UTF-8 safe and limited to 3800 characters.
- `app.Run` marks readiness false before it drains and stops both servers.
