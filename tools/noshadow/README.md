# `tools/noshadow`

## Purpose

This package provides a Go analyser that reports declarations shadowing names
from outer scopes.

It catches:

- short declarations
- nested `var`, `const`, `type`, and `func` declarations
- range variables
- type-switch variables
- predeclared identifier shadowing

## Usage

Run the standalone checker with:

```sh
go run ./cmd/noshadow ./...
```

## golangci-lint

The repo builds a custom golangci-lint binary with `noshadow` compiled in:

```sh
make lint-bin
```

`make lint` and `make lint-fix` run through `./bin/golangci-lint-custom`.
`noshadow` is enabled in `.golangci.yaml` with:

```yaml
linters:
  enable:
    - noshadow
  settings:
    custom:
      noshadow:
        type: module
        description: Reports declarations that shadow names from an outer scope.
        settings:
          testT: true
```

By default, `noshadow` reports all shadow declarations. Set `ctx`, `err`,
`found`, `ok`, or `testT` to `true` to allow that name.
