# `internal/config`

## Purpose

This package loads startup config from environment variables.

It:

- reads env vars
- applies defaults
- validates required values
- returns `Config`

It does not build clients or start the app.

## Dependencies

This package depends on `github.com/caarlos0/env/v11`.

## Flow

### Environment loading flow

```mermaid
flowchart TD
    environ[Process env or env map] --> parse[parseRawConfig]
    parse --> raw[Raw config]
    raw --> build[configFromRaw]
    build --> built[Built config]
    built --> validate[validateConfig]
    validate --> config[Config]
```

- `LoadEnviron` converts process-style env entries and calls `Load`.
- `parseRawConfig` reads env values into the raw config shape.
- `configFromRaw` applies defaults and parses timeout overrides.
- `validateConfig` checks table names and URLs before startup continues.

## Scope

This package owns:

- env parsing
- startup defaults
- startup validation
- typed startup config

## Validation

Startup fails when:

- a required setting is missing
- a table name is invalid
- a required URL is invalid
- an optional URL is set but invalid
- a timeout override is not a positive integer

## Server settings

`SERVER_ADDRESS` configures the webhook server. It defaults to `127.0.0.1:3000`
or `0.0.0.0:3000` when `DOCKER` is true.

`METRICS_SERVER_ADDRESS` configures the Prometheus server. It defaults to
`127.0.0.1:9090` or `0.0.0.0:9090` when `DOCKER` is true.

`HTTP_TIMEOUT_SECONDS`, `SHUTDOWN_TIMEOUT_SECONDS`, and
`READINESS_DRAIN_SECONDS` are positive integer durations. Their defaults are
10 seconds, 10 seconds, and 5 seconds respectively.

## Fallbacks

These do not fail startup:

- malformed process-style env entries passed to `LoadEnviron`
- invalid `SCALE_UP_READ_CAPACITY`, which falls back to `0`
