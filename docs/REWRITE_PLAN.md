# Telegram Jung2 Bot - Go Implementation Plan

## Goal

telegram-jung2-bot is now a Go service. The rewrite should be delivered as
small, testable packages that can be worked on in parallel without changing
existing DynamoDB data, SQS action names, Telegram commands, or generated bot
text.

Compatibility comes before optimisation. A module is not complete until its
tests prove the relevant compatibility rules.

## Working Model

- Keep package APIs narrow and domain-focused.
- Put pure domain logic behind interfaces before wiring AWS, Telegram, or HTTP.
- Prefer table-driven unit tests for domain rules and fake clients for adapters.
- Use historical fixtures only to create Go parity tests; do not add new runtime
  behaviour outside Go.
- Keep `go/core` temporary. New work should move into `go/internal/...` packages.
- Buck2 remains the build and test entry point until explicitly changed.

## Package Map

```text
go/
├── main.go
├── cmd/
│   └── buckify/
├── internal/
│   ├── app/          # composition root, lifecycle, graceful shutdown
│   ├── config/       # environment parsing and validation
│   ├── httpserver/   # HTTP routes, webhook endpoint, health checks
│   ├── telegram/     # Telegram update models and HTTP client
│   ├── queue/        # SQS polling, enqueueing, message deletion
│   ├── schedule/     # EventBridge Scheduler integration
│   ├── dynamodb/     # AWS client setup and pagination helpers
│   ├── chat/         # chat settings domain and repository
│   ├── message/      # message domain and repository
│   ├── command/      # command parsing, validation, action routing
│   ├── statistics/   # ranking, report models, text rendering
│   ├── workday/      # bitmask parsing and off-work matching
│   └── testutil/     # fake clients, fixture loaders, clock helpers
└── core/             # temporary scaffold to delete after module migration
```

## Parallel Work Rules

- Each module owns its models, interfaces, implementation, and tests.
- Shared types should move upward only when two implemented modules need them.
- Adapter packages may depend on domain packages; domain packages must not depend
  on adapters.
- Parallel branches should avoid editing `main.go` and `internal/app` until their
  module contracts and tests are merged.
- Cross-module integration should happen through explicit interfaces and small
  orchestration tests.

## Module Contracts

### `internal/config`

**Owns**

- Required environment variables and default values.
- Table names, queue URLs, Telegram token, AWS region, logging level, local
  endpoint overrides, and server addresses.

**Public API**

- `Load(env map[string]string) (Config, error)`.
- Validation helpers for table names, URLs, timeouts, and numeric limits.

**Tests**

- Table-driven tests for missing, invalid, and defaulted variables.
- Tests must assert that invalid table names fail before any AWS client is built.
- Tests must cover local development endpoint overrides.

### `internal/workday`

**Owns**

- The fixed workday bitmask values:
  `Sun=1`, `Mon=2`, `Tue=4`, `Wed=8`, `Thu=16`, `Fri=32`, `Sat=64`.
- Parsing stored workday values and checking whether a date is a workday.

**Public API**

- `Parse(mask int) (Workdays, error)`.
- `Contains(workdays Workdays, date time.Time) bool`.

**Tests**

- Table-driven tests for each day, weekday combinations, empty masks, and invalid
  values.
- Golden tests for `MON,TUE,WED,THU,FRI == 62` behaviour.

### `internal/telegram`

**Owns**

- Telegram update, chat, user, and message models used by the service.
- Telegram HTTP client for sending messages and reading administrators.
- Telegram response truncation at 3800 characters.

**Public API**

- `ParseUpdate([]byte) (Update, error)`.
- `Client.SendMessage(ctx, chatID, text) error`.
- `Client.GetChatAdministrators(ctx, chatID) ([]Administrator, error)`.

**Tests**

- Fixture tests for supported update shapes and optional Telegram fields.
- `httptest` client tests for request URL, method, payload, and error handling.
- Unicode truncation tests that prove output remains valid UTF-8.

### `internal/command`

**Owns**

- Supported Telegram commands and aliases.
- Command validation, admin requirement metadata, and conversion to queue actions.

**Public API**

- `Parse(text string) (Command, bool)`.
- `ActionFor(command Command, chat ChatContext) (queue.Action, error)`.

**Tests**

- Table-driven tests for every command, alias, unknown command, casing, and
  leading/trailing whitespace case.
- Tests for admin-only commands and invalid arguments such as bad off-work time.
- Fixture parity tests for command-to-action names and payload shape.

### `internal/message`

**Owns**

- Stored message model, TTL calculation, and message repository.
- Query windows for statistics.

**Public API**

- `Repository.Save(ctx, Message) error`.
- `Repository.QueryByChat(ctx, chatID, since, until) ([]Message, error)`.

**Tests**

- Unit tests for TTL and timestamp formatting.
- Repository tests with a fake DynamoDB API that verifies key names and
  attributes.
- Pagination tests that require reading until `LastEvaluatedKey` is empty.

### `internal/chat`

**Owns**

- Chat settings model and repository.
- Defaults for missing settings.

**Public API**

- `Repository.Get(ctx, chatID) (Settings, error)`.
- `Repository.Save(ctx, Settings) error`.
- `Repository.ListEnabled(ctx) ([]Settings, error)`.

**Tests**

- Repository tests for `CHATID_TABLE` key and attribute names.
- Tests for missing rows, partial rows, malformed rows, and default values.
- Pagination tests for scans and list operations.

### `internal/statistics`

**Owns**

- Ranking calculations for top chatters, silent users, and all participants.
- Report rendering and ordering rules.

**Public API**

- `TopTen(messages []message.Message) Report`.
- `TopDiver(messages []message.Message, participants []telegram.User) Report`.
- `AllJung(messages []message.Message) Report`.
- `Render(report Report) string`.

**Tests**

- Table-driven ranking tests covering ties, zero-message users, missing names,
  and large chats.
- Golden fixture tests for exact rendered text.
- Truncation tests that prove final rendered messages stay within 3800
  characters.

### `internal/queue`

**Owns**

- SQS action model, long polling, enqueueing, decoding, and deletion.
- Compatibility with both message attribute casings:
  `messageAttributes.action.stringValue` and
  `messageAttributes.action.StringValue`.

**Public API**

- `Action` model with stable action names and payloads.
- `Consumer.Poll(ctx, handler Handler) error`.
- `Producer.Enqueue(ctx, Action) error`.
- `DecodeMessage(raw Message) (Action, error)`.

**Tests**

- Decoder tests for both attribute casings and malformed messages.
- Producer tests for stable action names, attributes, and payload JSON.
- Consumer tests that delete only after successful handling and preserve failed
  messages for retry.

### `internal/schedule`

**Owns**

- EventBridge Scheduler integration for off-work reports.
- Schedule identity, idempotent upserts, and skip reasons.

**Public API**

- `Service.SyncChat(ctx, chat.Settings) error`.
- `Service.HandleDueReport(ctx, time.Time) error`.

**Tests**

- Fake scheduler tests for create, update, disable, and idempotent behaviour.
- Tests for timezone conversion, workday matching, and skipped schedules.
- Action fixture tests for scheduled SQS messages.

### `internal/httpserver`

**Owns**

- HTTP routing, webhook endpoint, health checks, request validation, and response
  status codes.

**Public API**

- `New(ServerDeps) http.Handler`.
- Route handlers should depend on interfaces, not concrete AWS clients.

**Tests**

- `httptest` tests for webhook success, malformed JSON, unsupported updates,
  health checks, and dependency failures.
- Tests must prove unsupported Telegram updates are ignored safely.

### `internal/app`

**Owns**

- Application wiring, dependency construction, worker lifecycle, and graceful
  shutdown.

**Public API**

- `Run(ctx, Config) error`.

**Tests**

- Startup tests with fake dependencies.
- Shutdown tests that prove HTTP and queue workers stop on context cancellation.
- Error propagation tests for failed configuration or dependency construction.

### `internal/dynamodb`

**Owns**

- Shared DynamoDB client construction and pagination helpers.
- Common error classification and sanitised logging fields.

**Public API**

- Small interfaces implemented by AWS SDK clients and fakes.
- Pagination helpers used by `chat` and `message` repositories.

**Tests**

- Helper tests for pagination, empty pages, and propagated errors.
- Tests that errors do not expose unnecessary Telegram message text.

### `internal/testutil`

**Owns**

- Fixture loading, fake AWS/Telegram clients, deterministic clocks, and helper
  assertions shared by tests.

**Tests**

- Keep helpers small enough to be obvious; test only non-trivial helper logic.

## Integration Slices

Use these slices to merge independently completed modules without waiting for
the whole rewrite.

1. **Webhook intake**
   - Modules: `telegram`, `command`, `message`, `queue`, `httpserver`.
   - Tests: webhook fixture to saved message or queued action with fake
     repositories and fake queue producer.

2. **Command execution**
   - Modules: `queue`, `command`, `chat`, `message`, `statistics`, `telegram`.
   - Tests: SQS action fixture to exact Telegram response text.

3. **Settings management**
   - Modules: `command`, `chat`, `telegram`, `schedule`.
   - Tests: admin command fixture to settings update and schedule sync.

4. **Scheduled reports**
   - Modules: `schedule`, `queue`, `chat`, `message`, `statistics`, `telegram`.
   - Tests: due scheduled action to exact report or documented skip reason.

5. **Application wiring**
   - Modules: `config`, `app`, `httpserver`, `queue`, all adapters.
   - Tests: fake full app starts, handles cancellation, and returns dependency
     errors clearly.

## Test Strategy

- Every package should have unit tests before integration wiring.
- Each compatibility rule should have at least one named test.
- Use golden files only for stable rendered bot text and external payload
  fixtures.
- Prefer fake interfaces over live AWS, Telegram, or Docker dependencies in unit
  tests.
- Add integration tests only after the relevant package contracts are stable.
- `go test ./...` and `make test` must pass before replacing production
  behaviour.

## Compatibility Checklist

- [ ] Same command names and aliases.
- [ ] Same SQS action names and payload shape.
- [ ] Same DynamoDB table names and attribute names.
- [ ] Same `workday` bitmask values.
- [ ] Same off-work default time behaviour.
- [ ] Same admin permission checks.
- [ ] Same Telegram response text and truncation limit.
- [ ] Same handling for optional Telegram fields.
- [ ] Same behaviour for missing or malformed chat settings.

## Quality Gates

- Package APIs are documented by tests before downstream packages depend on
  them.
- Domain packages avoid direct AWS, Telegram, HTTP, or environment access.
- Adapters use contexts, bounded timeouts, and structured `log/slog` logging.
- DynamoDB reads handle `LastEvaluatedKey` pagination.
- Logs avoid leaking unnecessary Telegram message text.
- Buck2 targets stay deterministic while Buck2 remains in the toolchain.

## Open Decisions

- Whether Buck2 remains the long-term build entry point or becomes a thin wrapper
  around `go test` and `go build`.
- Whether historical runtime files should be removed once Go fixture parity is
  complete.
