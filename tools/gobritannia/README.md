# `tools/gobritannia`

## Purpose

This package provides a Go analyser that reports configured US English terms in
Go definitions and comments. The term list is deliberately generic so the
package can move to its own repository later.

It covers spelling differences and vocabulary differences from
`tools/gobritannia/terms.go`. It intentionally does not inspect external
selector names such as Go API types, so code can still refer to upstream
identifiers with US spellings.

Use `allow` for terms that are valid US English in normal prose but should stay
unchanged in a specific project or domain. Each allow entry has a `term` and an
optional `comment` setting. `comment` defaults to `true`, meaning the term is
allowed in both code identifiers and comments. Set `comment: false` to allow
the term in code identifiers only, while still reporting it in comments.

For example, `cookie` maps to `biscuit` in everyday vocabulary, but HTTP APIs
normally use `cookie`, so a web project can allow it without weakening the rest
of the dictionary.

## Usage

Run the standalone checker with:

```sh
go run ./cmd/gobritannia ./...
```

Allow project-specific terms in standalone mode with:

```sh
go run ./cmd/gobritannia -allow=cookie,cookies ./...
```

The standalone flag is shorthand for `comment: true`.

## golangci-lint

The repo builds a custom golangci-lint binary with `gobritannia` compiled in:

```sh
make lint-bin
```

`make lint` and `make lint-fix` run through `./bin/golangci-lint-custom`.
`gobritannia` is enabled in `.golangci.yaml`.

Allow project-specific terms in golangci-lint with:

```yaml
linters:
  settings:
    custom:
      gobritannia:
        settings:
          allow:
            - term: cookie
            - term: color
              comment: false
```
