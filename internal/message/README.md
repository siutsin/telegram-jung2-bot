# `internal/message`

## Purpose

This package owns persisted Telegram message models and contract helpers.

It:

- converts Telegram messages into stored rows
- formats and parses `dateCreated`
- computes message TTL
- builds message update shapes

It does not talk to DynamoDB directly.

## Dependencies

This package depends on:

- `internal/telegram`

## Flow

### Store flow

```mermaid
flowchart TD
    telegram[Telegram message] --> fromTelegram[FromTelegram]
    fromTelegram --> stored[Message]
    stored --> build[BuildSaveUpdate]
    build --> update[DynamoDB update shape]
```

- `FromTelegram` builds the stored message row
- `BuildSaveUpdate` turns that row into the DynamoDB update shape
- `BuildSaveUpdate` preserves the legacy assignment order with `ttl` last

### Date flow

```mermaid
flowchart TD
    time[time.Time] --> format[FormatDateCreated]
    format --> raw[Stored dateCreated]
    raw --> parse[ParseDateCreated]
    parse --> time2[time.Time]
```

- stored `dateCreated` keeps the contract UTC+8 format

## Scope

This package owns:

- message models
- `dateCreated` formatting
- TTL calculation
- message update shapes

## Validation

Parsing fails when:

- stored `dateCreated` is malformed
