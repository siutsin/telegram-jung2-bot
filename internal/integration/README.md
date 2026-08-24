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
- `FLOCI_PORT` — host port for the Apple container fallback below (default
  `4566`).
- `FLOCI_CPUS` — CPUs allocated by the Apple container fallback (default `4`).
- `FLOCI_MEMORY` — memory allocated by the Apple container fallback (default
  `2G`).
- `AWS_REGION` — local AWS SDK region (default `eu-west-1`).

Testcontainers also starts a short-lived `reaper_*` (Ryuk) container to clean
up labelled containers when the test process exits.

### No Docker

`make integration` runs through `hack/integration.sh`. If Docker is not
available and the Apple `container` CLI is, the script starts Floci itself
with `container run`, waits for `/_floci/init`, sets `FLOCI_ENDPOINT` for the
test run, and stops/removes the container afterwards. Testcontainers-go has no
support for Apple's container tool, so this path never invokes it. Set
`FLOCI_ENDPOINT` yourself to skip this fallback and reuse an endpoint you
started another way.

## Layout

`TestMain` starts Floci once (or uses `FLOCI_ENDPOINT`), builds AWS clients, and
stops the container after all tests. Each `TestFloci*` that exercises AWS
provisions its own DynamoDB tables and SQS queue via `startIntegrationTest`,
then deletes them in `t.Cleanup`. Local fixture and lifecycle tests only use
the shared isolated runtime because they do not access AWS resources.
Resource-backed tests run at most three at a time so Floci stays responsive while
independent test cases still execute concurrently.

Buck runs one `go_test` target; Go runs sixteen top-level tests. Failures name
the failing `TestFloci*` (and subtests where used) in Buck stderr/stdout.

- `TestFlociDynamoDB` — chat/message CRUD, due-chat scan
- `TestFlociDynamoDBDueChatPagination` — multi-page `DueChatIDs`
- `TestFlociDynamoDBMessagePagination` — multi-page `QueryByChat`
- `TestFlociSQS` — all command and schedule actions, casing, Floci round-trip
- `TestFlociSQSBatch` — multi-message worker batch through one poll window
- `TestFlociHTTPHealth` — `/health`
- `TestFlociHTTPWebhook` — group webhooks, multi-command order, invalid admin cmd
- `TestFlociHTTPWebhookTelegramClient` — webhook reply via real `telegram.Client` + httptest
- `TestFlociHTTPStage` — stage ping, webhook, scheduler, scale-up routes
- `TestFlociAppRun` — `app.Run` with HTTP server, worker, health, and queue action
- `TestFlociAppRunGracefulShutdownAfterComponentCrash` — each queue worker,
  HTTP server, and metrics server failure makes readiness fail and shuts down
  both servers. It also confirms `/metrics` returns Prometheus runtime data.
  The worker and metrics failures also drain an in-flight HTTP request.
- `TestFlociWorkerRun` — production `worker.Run` poll loop with cancel
- `TestFlociWorkerHandlers` — `worker.Handlers` dispatch to `service.Service`
- `TestFlociWorkerService` — single-poll service dispatch for report actions
- `TestFlociServiceOnOffFromWork` — `OnOffFromWork` fan-out to `offFromWork`
- `TestFlociServiceAdminSettings` — admin settings side effects

## Scope

Adapter and lifecycle smoke tests against local Floci.

**In scope:** real SDK clients, temporary tables/queue, production builders and
handlers, queue encode/receive/decode/delete, `httptest` HTTP routing,
`telegram.Client` against a fake Bot API, `worker.Run`, `worker.Handlers`,
multi-page DynamoDB scans/queries, Floci SQS receive, `app.Run` shutdown with
a live HTTP listener.

**Out of scope (not suitable for this harness):**

- **Real Telegram API** — needs a live bot token, external network, and creates
  flaky CI dependency. Covered locally via `httptest` + `telegram.WithBaseURL`.
- **EventBridge Scheduler service** — not emulated by Floci; the scheduler HTTP
  contract is already exercised by `TestFlociHTTPStage`.
- **AWS IAM and throttling faults** — need injected SDK errors or a fault
  injection proxy, not local happy-path emulation.
- **Captured production traffic replay** — would need curated prod fixtures
  and ongoing maintenance beyond what this harness already covers.
