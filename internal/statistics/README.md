# `internal/statistics`

## Purpose

This package renders chat activity reports.

It:

- groups rows by user
- sorts rankings
- builds report header, body, and footer
- truncates final rendered output to the Telegram-safe limit

It does not query message storage or send Telegram messages.

## Dependencies

This package depends on:

- `internal/message`
- `internal/telegram`

## Flow

### Report flow

```mermaid
flowchart TD
    rows[Message rows] --> normalise[NormaliseRows]
    normalise --> render[GenerateReport]
    render --> summary[Summary]
```

- rows are grouped and counted first
- then the report text is rendered and truncated from the normalised rankings

### Render flow

```mermaid
flowchart TD
    normalised[Normalised rows] --> header[BuildHeader]
    normalised --> body[BuildBodyWithLimit]
    normalised --> footer[BuildFooter]
    header --> report[Report text]
    body --> report
    footer --> report
    report --> truncate[Truncate final text]
```

- final report text is truncated after header, body, footer, and prefixes are rendered

## Scope

This package owns:

- report ranking rules
- report text rendering
- report truncation

## Validation

Report generation fails when:

- the input row list is empty

## Fallbacks

These do not fail:

- zero `Now`, which falls back to the current time
- zero `WindowDays`, which falls back to the default 7-day window
