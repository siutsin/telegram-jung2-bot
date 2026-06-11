# `internal/integration`

Opt-in Floci integration tests for the real AWS SDK v2 DynamoDB and SQS
adapters, production HTTP handlers, worker dispatch, and app lifecycle. Not part
of `make test` or `make coverage` (Buck `slow` label).

## Run

```bash
make integration
```

Requires `INTEGRATION_TESTS=1` (set by the Makefile). Without it, tests skip.

Optional environment variables:

- `FLOCI_ENDPOINT` — reuse an existing Floci-compatible endpoint instead of
  starting Docker.
- `FLOCI_CONTAINER_NAME` — Docker name for the Floci container (default
  `telegram-jung2-bot-it-floci`).
- `FLOCI_IMAGE` — image override (default `floci/floci:latest`).
- `AWS_REGION` — local AWS SDK region (default `eu-west-1`).

Testcontainers also starts a short-lived `reaper_*` (Ryuk) container to clean
up labelled containers when the test process exits.

## Layout

`TestMain` starts Floci once (or uses `FLOCI_ENDPOINT`), builds AWS clients, and
stops the container after all tests. Each `TestFloci*` provisions its own
DynamoDB tables and SQS queue via `startIntegrationTest`, then deletes them in
`t.Cleanup`.

Buck runs one `go_test` target; Go runs seventeen top-level tests. Failures name
the failing `TestFloci*` (and subtests where used) in Buck stderr/stdout.

- `TestFlociDynamoDB` — chat/message CRUD, due-chat scan, `ListEnabled`
- `TestFlociDynamoDBPagination` — multi-page `ListEnabled`
- `TestFlociDynamoDBDueChatPagination` — multi-page `DueChatIDs`
- `TestFlociDynamoDBMessagePagination` — multi-page `QueryByChat`
- `TestFlociLegacySQSFixtures` — legacy JS Lambda/ECS queue payload decode
- `TestFlociSQS` — all command and schedule actions, casing, Floci round-trip
- `TestFlociSQSBatch` — multi-message worker batch through one poll window
- `TestFlociHTTPHealth` — `/health`
- `TestFlociHTTPWebhook` — group webhooks, multi-command order, invalid admin cmd
- `TestFlociHTTPWebhookTelegramClient` — webhook reply via real `telegram.Client` + httptest
- `TestFlociHTTPStage` — stage ping, webhook, scheduler, scale-up routes
- `TestFlociAppRun` — `app.Run` with HTTP server, worker, health, and queue action
- `TestFlociWorkerRun` — production `worker.Run` poll loop with cancel
- `TestFlociWorkerHandlers` — `worker.Handlers` dispatch to `service.Service`
- `TestFlociWorkerService` — single-poll service dispatch for report actions
- `TestFlociServiceOnOffFromWork` — `OnOffFromWork` fan-out to `offFromWork`
- `TestFlociServiceAdminSettings` — admin settings side effects

## Scope

Adapter and lifecycle smoke tests against local Floci.

**In scope:** real SDK clients, temporary tables/queue, production builders and
handlers, queue encode/receive/decode/delete, `httptest` HTTP routing,
`telegram.Client` against a fake Bot API, legacy JS queue fixtures,
`worker.Run`, `worker.Handlers`, multi-page DynamoDB scans/queries, Floci SQS
receive, `app.Run` shutdown with a live HTTP listener.

**Out of scope (not suitable for this harness):**

- **Real Telegram API** — needs a live bot token, external network, and creates
  flaky CI dependency. Covered locally via `httptest` + `telegram.WithBaseURL`.
- **EventBridge Scheduler service** — not emulated by Floci; the scheduler HTTP
  contract is already exercised by `TestFlociHTTPStage`.
- **AWS IAM and throttling faults** — need injected SDK errors or a fault
  injection proxy, not local happy-path emulation.
- **Captured production traffic replay** — would need curated prod fixtures and
  ongoing maintenance beyond the legacy JS contract shapes already checked in
  `TestFlociLegacySQSFixtures`.

## Cutover cleanup

`TestFlociLegacySQSFixtures` and `legacyfixtures.go` are temporary JS-to-Go
parity checks. Remove them after production cutover when the old Node bot is
retired, the queue no longer carries legacy-shaped messages, and only Go
producers enqueue work.

## Files

- `integration_test.go` — `TestMain`, gate, top-level `TestFloci*`
- `setup.go` — shared runtime bootstrap, per-test resources
- `floci.go` — Testcontainers Floci start/stop
- `aws.go` — AWS clients, table/queue provision
- `checks.go` — DynamoDB and SQS assertions
- `pagination.go` — multi-page DynamoDB scan/query checks
- `legacyfixtures.go` — legacy JS queue payload fixtures
- `telegram.go` — httptest Telegram Bot API harness
- `app.go` — `app.Run` lifecycle smoke test
- `http.go`, `webhook.go`, `stage.go` — HTTP route checks
- `worker.go`, `service.go`, `settings.go` — service and worker flows
- `dispatch.go`, `helpers.go` — handler mapping and shared helpers
