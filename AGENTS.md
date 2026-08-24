# Agent Instructions

Go service for `telegram-jung2-bot`. The executable lives in `cmd/`. Private
packages live in `internal/`. Do not create a `go/` directory. Preserve
production contracts with focused Go tests.

## Commands

Do not run native `go test`.

| Task             | Command                                |
|------------------|----------------------------------------|
| Local validation | `make test`                            |
| Coverage         | `make coverage`                        |
| Mocks            | `make mock`                            |
| Dependencies     | `make vendor`                          |
| CI               | `make ci`                              |
| Local image      | `container build`, else `docker build` |

`make test` is the local gate. It runs lint, then Buck2 tests with the race
detector. `make test-only` skips lint. CI uses that after one lint job.

`make coverage` must stay at 100% for `internal/`. `cmd/` is excluded.

Run `make vendor` after dependency changes. It uses
`prelude//go/tools/gobuckify:gobuckify`. Do not add a custom generator.

Local images: use Apple Container first when it is available. Otherwise use
Docker.

Create worktrees under `/tmp`, not inside this repo. Nested worktrees,
including `.scratchpad/worktrees/`, make Buck2 use the wrong root. A new
worktree has no `vendor/`. Symlink it from the main checkout before
`make test`.

## Layout

- Keep domain packages free of AWS SDK clients, Telegram HTTP clients, HTTP
  servers, environment readers, and app wiring.
- Keep Buck target visibility explicit.
- Do not add shared `testutil`, shared mocks, or abstraction packages until
  reuse is proven.
- After you change a package-local interface, run `make mock`. Generated
  GoMock lives under `internal/mock/` via `go.uber.org/mock`. Use a
  hand-written fake only while the generated mock is noisier.

## Contracts

Preserve DynamoDB names and `dateCreated` UTC+8 parsing. Preserve Telegram
command names, aliases, response text, and ordering. Preserve SQS action
names. Name the contract test that the change preserves.

| Item          | Rule                                                                           |
|---------------|--------------------------------------------------------------------------------|
| Workday bits  | `Sun=1`, `Mon=2`, `Tue=4`, `Wed=8`, `Thu=16`, `Fri=32`, `Sat=64`               |
| MESSAGE_TABLE | Partition `chatId`, sort `dateCreated`, TTL 7 days                             |
| CHATID_TABLE  | Partition `chatId`. Stores `chatTitle`, `enableAllJung`, `offTime`, `workday`  |
| Pagination    | Follow `LastEvaluatedKey`                                                      |
| Reports       | Truncate final rendered reports at 3800 characters. Keep truncation UTF-8 safe |
| SQS action    | Support `messageAttributes.action.stringValue` and `StringValue`               |
| Logs          | Do not log Telegram message text                                               |

## Go

When a function transforms, normalises, validates, or derives data, add a
short docstring example of the input and output shape.

## Tests

If a package uses goroutines, channels, or shared mutable state, the tests
must exercise that concurrency. Do not add concurrency tests for parsers or
formatters.
