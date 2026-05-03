# `internal/chat`

## Purpose

This package owns stored chat settings and chat update shapes.

It:

- loads chat settings
- saves chat metadata
- builds chat update requests
- applies stored chat defaults
- filters chats for scheduled reports

It does not talk to DynamoDB directly.

## Dependencies

This package depends on:

- `internal/message`
- `internal/telegram`
- `internal/workday`

## API

- `FromTelegram(input telegram.Message, now time.Time) Settings`
- `FromRow(row Row) (Settings, error)`
- `FromScheduleRow(row Row) Settings`
- `BuildMetadataUpdate(tableName string, settings Settings) UpdateExpression`
- `BuildAllJungUpdate(tableName string, chatID int64, enabled bool) UpdateExpression`
- `BuildOffWorkUpdate(tableName string, chatID int64, offTime string, workdays workday.Workdays) UpdateExpression`
- `FilterDue(rows []Settings, offTime string, day string) []Settings`

## Scope

This package owns:

- chat settings models
- chat repository wrapper
- chat update request shapes
- chat defaulting
- due-chat filtering

## Validation

Chat loading fails when:

- stored `dateCreated` is invalid
- stored `workday` is invalid in full row parsing

## Fallbacks

These do not fail:

- missing stored rows, which default to enabled chat settings
- malformed scheduled `workday` bits in `ListEnabled`, which are masked to known weekday bits
