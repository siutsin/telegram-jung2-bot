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

Buck runs one `go_test` target; Go runs ten top-level tests. Failures name the
failing `TestFloci*` (and subtests where used) in Buck stderr/stdout.

- `TestFlociDynamoDB` — chat/message CRUD, due-chat scan, `ListEnabled`
- `TestFlociDynamoDBPagination` — `ListEnabled` across multi-page DynamoDB scans
- `TestFlociSQS` — all command and schedule queue actions, attribute casing,
  Floci receive round-trip
- `TestFlociHTTPHealth` — `/health`
- `TestFlociHTTPWebhook` — group webhooks, multi-command order, invalid admin cmd
- `TestFlociHTTPStage` — stage ping, webhook, scheduler, scale-up routes
- `TestFlociWorkerRun` — production `worker.Run` poll loop with cancel
- `TestFlociWorkerService` — SQS poll → `service.Service` → recorded replies
- `TestFlociServiceOnOffFromWork` — `OnOffFromWork` fan-out to `offFromWork`
- `TestFlociServiceAdminSettings` — `DisableAllJung`, `EnableAllJung`,
  `SetOffWorkTime`

## Scope

Adapter smoke tests against local Floci — not full production parity.

**In scope:** real SDK clients, temporary tables/queue, production builders and
handlers, queue encode/receive/decode/delete, `httptest` HTTP routing,
`worker.Run`, multi-page `ListEnabled`, Floci SQS receive plus Lambda-style
attribute casing on received payloads.

**Out of scope:** real Telegram API, EventBridge service itself, IAM/throttling,
JS-vs-Go fixtures. SQS checks still round-trip Go-encoded messages through Go
decode, so shared encode/decode bugs need independent fixtures.

## Files

- `integration_test.go` — `TestMain`, gate, top-level `TestFloci*`
- `setup.go` — shared runtime bootstrap, per-test resources
- `floci.go` — Testcontainers Floci start/stop
- `aws.go` — AWS clients, table/queue provision
- `checks.go` — DynamoDB and SQS assertions
- `http.go`, `webhook.go`, `stage.go` — HTTP route checks
- `worker.go`, `service.go`, `settings.go` — service-layer flows
- `dispatch.go`, `helpers.go` — handler mapping and shared helpers
