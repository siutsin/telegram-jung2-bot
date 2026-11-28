# telegram-jung2-bot

[![Latest Dependencies](https://github.com/siutsin/telegram-jung2-bot/actions/workflows/latest-dependencies.yaml/badge.svg)](https://github.com/siutsin/telegram-jung2-bot/actions/workflows/latest-dependencies.yaml)
[![govulncheck](https://github.com/siutsin/telegram-jung2-bot/actions/workflows/govulncheck.yaml/badge.svg)](https://github.com/siutsin/telegram-jung2-bot/actions/workflows/govulncheck.yaml)

Add the bot to your group at [@jung2_bot](https://bit.ly/github-jung2bot)

**冗員**[jung2jyun4] Excess personnel in Cantonese

This bot is created for counting the number of messages per participant in a
chat group.

## Usage

| command     | info                                                                                                                      |
|-------------|---------------------------------------------------------------------------------------------------------------------------|
| `/allJung`  | Show the percentage of all participants for the past seven days                                                           |
| `/jungHelp` | Show help message                                                                                                         |
| `/topDiver` | Show the percentage of top ten divers for the past seven days (Requires at least one message from the user to be counted) |
| `/topTen`   | Show the percentage of top ten participants for the past seven days                                                       |

### Admin Only

| command                  | info                                                                                                                                                                                         |
|--------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `/disableAllJung`        | Disable `/allJung` command                                                                                                                                                                   |
| `/enableAllJung`         | Enable `/allJung` command                                                                                                                                                                    |
| `/setOffFromWorkTimeUTC` | Set offFromWork time in UTC. Format: `/setOffFromWorkTimeUTC {{ 0000-2345, 15 minutes interval }} {{ MON,TUE,WED,THU,FRI,SAT,SUN }}`. E.g. `/setOffFromWorkTimeUTC 1800 MON,TUE,WED,THU,FRI` |

## Architecture

The bot counts group messages, ranks speakers, and sends off-work reports.
[Telegram](https://core.telegram.org/bots) and the scheduler hit HTTP. HTTP
writes [DynamoDB](https://aws.amazon.com/dynamodb/) and enqueues
[SQS](https://aws.amazon.com/sqs/). The worker consumes SQS and calls DynamoDB
and Telegram.

| Process | Address | Role                                                                                |
|---------|---------|-------------------------------------------------------------------------------------|
| HTTP    | `:3000` | [Webhook](https://core.telegram.org/bots/webhooks), `/health`, and scheduler routes |
| Metrics | `:9090` | `/metrics`                                                                          |
| Worker  |         | SQS actions                                                                         |

Code lives in `cmd/` and `internal/`. [Buck2](https://buck2.build/) builds and
tests.

### Metrics

[Prometheus](https://prometheus.io/) `/metrics` includes Go and process
collectors plus:

| Metric                                                   | Meaning                                 |
|----------------------------------------------------------|-----------------------------------------|
| `telegram_jung2_bot_dependency_request_duration_seconds` | Outbound call duration                  |
| `telegram_jung2_bot_dependency_requests_total`           | DynamoDB, SQS, and Telegram calls       |
| `telegram_jung2_bot_http_request_duration_seconds`       | HTTP duration                           |
| `telegram_jung2_bot_http_requests_in_flight`             | In-flight HTTP requests                 |
| `telegram_jung2_bot_http_requests_total`                 | HTTP requests                           |
| `telegram_jung2_bot_off_work_reports_enqueued_total`     | Scheduled report enqueue results        |
| `telegram_jung2_bot_ready`                               | Readiness                               |
| `telegram_jung2_bot_scale_up_total`                      | DynamoDB scale-up results               |
| `telegram_jung2_bot_webhook_commands_enqueued_total`     | Commands queued                         |
| `telegram_jung2_bot_webhook_updates_total`               | Webhook outcomes                        |
| `telegram_jung2_bot_worker_action_duration_seconds`      | Queue action duration, pickup to finish |
| `telegram_jung2_bot_worker_actions_total`                | Queue actions                           |
| `telegram_jung2_bot_worker_queue_wait_duration_seconds`  | Queue lag, enqueue to pickup            |

HTTP metrics use fixed method, status, and route labels.

## Where is the JavaScript version?

This project started in [2016](https://github.com/siutsin/telegram-jung2-bot/pull/1),
more than a decade ago (!), as my technical
playground. It was a way to learn [Node.js](https://nodejs.org/) and the
[Telegram Bot API](https://core.telegram.org/bots/api), and to put a meme bot
in my Telegram groups. For whatever reason it spread, and thousands of groups
were using it at that time.

Over time it became a lab for experiments:
[Heroku](https://www.heroku.com/), [PM2](https://pm2.keymetrics.io/),
[Serverless Framework](https://www.serverless.com/),
[AWS ECS](https://aws.amazon.com/ecs/), [Kubernetes](https://kubernetes.io/),
and more that I cannot even remember. The funny thing is that I have not used
this bot myself for a very long time.

The JavaScript ecosystem has constant supply-chain issues, and the number of
dependencies and their transitive dependencies is scary. I rewrote the
service in [Go](https://go.dev/) so that it would need almost no maintenance,
whilst keeping it running until no one is using it.

## Prerequisites

- Buck2
- Go 1.27+
- [Apple Container](https://github.com/apple/container) for local images, or
  [Docker](https://docs.docker.com/) if it is unavailable. Either covers the
  optional [Floci](https://floci.io/) check

## Commands

| Target               | What it does                                               |
|----------------------|------------------------------------------------------------|
| `make build`         | Build the service. Does not vendor.                        |
| `make ci`            | `vendor`, then `coverage` (runs `test`, which runs `lint`) |
| `make clean`         | Remove Buck2 outputs                                       |
| `make coverage`      | Fail unless `internal/` coverage is 100%                   |
| `make install-buck2` | Install or upgrade Buck2                                   |
| `make integration`   | Floci AWS tests. See `internal/integration/README.md`.     |
| `make lint`          | Go, shell, and Markdown checks                             |
| `make lint-fix`      | Apply supported lint fixes                                 |
| `make mock`          | Regenerate `internal/mock/`                                |
| `make test`          | Lint, then fast race tests. Skips `slow`.                  |
| `make test-only`     | Tests without lint. CI runs this on arm64.                 |
| `make vendor`        | Refresh vendor and third-party `BUCK` files                |
