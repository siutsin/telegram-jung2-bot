# telegram-jung2-bot

Telegram group chat statistics bot. Tracks message counts, produces rankings,
and schedules off-work reports.

Go HTTP webhook, SQS worker, Telegram, and DynamoDB. Kubernetes CronJobs call
the internal `/onOffFromWork` and `/onScaleUp` routes. Code is in `cmd/` and
`internal/`. Buck2 builds and tests.

## Where is the JavaScript version?

This project started in 2016, more than a decade ago (!), as my technical
playground. It was a way to learn Node.js and the Telegram Bot API, and
to put a meme bot in my Telegram groups. For whatever reason it
spread, and thousands of groups were using it at that time.

Over time it became a lab for experiments: Heroku, PM2, Serverless Framework,
AWS ECS, Kubernetes, and more that I cannot even remember. The funny thing
is that I have not used this bot myself for a very long time.

The JavaScript ecosystem has constant supply-chain issues, and the number of
dependencies and their transitive dependencies is scary. I rewrote the
service in Go so that it would need almost no maintenance, whilst keeping it
running until no one is using it.

## Prerequisites

- [Buck2](https://buck2.build/docs/getting_started/)
- Go 1.27+
- Docker, for the optional Floci AWS integration check

## Commands

| Target               | What it does                                               |
|----------------------|------------------------------------------------------------|
| `make install-buck2` | Install or upgrade Buck2                                   |
| `make build`         | Build the service. Does not vendor.                        |
| `make ci`            | `vendor`, then `coverage` (runs `test`, which runs `lint`) |
| `make test`          | Lint, then fast race tests. Skips `slow`.                  |
| `make test-only`     | Tests without lint. CI runs this on arm64.                 |
| `make coverage`      | Fail unless `internal/` coverage is 100%                   |
| `make integration`   | Floci AWS tests. See `internal/integration/README.md`.     |
| `make lint`          | Go, shell, and Markdown checks                             |
| `make lint-fix`      | Apply supported lint fixes                                 |
| `make mock`          | Regenerate `internal/mock/`                                |
| `make vendor`        | Refresh vendor and third-party `BUCK` files                |
| `make clean`         | Remove Buck2 outputs                                       |
