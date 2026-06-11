# `internal/integration`

Opt-in Floci integration tests for the real AWS SDK v2 DynamoDB and SQS
adapters, production HTTP handlers, and service dispatch. Not part of `make
test` or `make coverage` (Buck `slow` label).

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

Buck runs one `go_test` target; Go runs eight top-level tests. Failures name the
failing `TestFloci*` (and subtests where used) in Buck stderr/stdout.

- `TestFlociDynamoDB` — chat/message CRUD, due-chat scan, `ListEnabled`
- `TestFlociSQS` — all command and schedule queue actions, attribute casing
- `TestFlociHTTPHealth` — `/health`
- `TestFlociHTTPWebhook` — group webhooks, multi-command order, invalid admin cmd
- `TestFlociHTTPStage` — stage ping, webhook, scheduler, scale-up routes
- `TestFlociWorkerService` — SQS poll → `service.Service` → recorded replies
- `TestFlociServiceOnOffFromWork` — `OnOffFromWork` fan-out to `offFromWork`
- `TestFlociServiceAdminSettings` — `DisableAllJung`, `EnableAllJung`,
  `SetOffWorkTime`

## Scope

Adapter smoke tests against local Floci — not full production parity.

**In scope:** real SDK clients, temporary tables/queue, production builders and
handlers, queue encode/receive/decode/delete, `httptest` HTTP routing.

**Out of scope:** real Telegram API, EventBridge, full `worker.Run` loop,
multi-page DynamoDB scans, IAM/throttling, JS-vs-Go fixtures, mixed Lambda/SQS
casing beyond the explicit decode contract cases.

SQS checks round-trip Go-encoded messages through Go decode. That catches
adapter and emulator mistakes but not shared encode/decode bugs.

## Files

- `integration_test.go` — `TestMain`, gate, top-level `TestFloci*`
- `setup.go` — shared runtime bootstrap, per-test resources
- `floci.go` — Testcontainers Floci start/stop
- `aws.go` — AWS clients, table/queue provision
- `checks.go` — DynamoDB and SQS assertions
- `http.go`, `webhook.go`, `stage.go` — HTTP route checks
- `worker.go`, `service.go`, `settings.go` — service-layer flows
- `dispatch.go`, `helpers.go` — handler mapping and shared helpers
