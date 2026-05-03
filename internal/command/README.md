# `internal/command`

## Purpose

This package parses supported Telegram commands.

It:

- finds supported command names in message text
- keeps contract command casing
- extracts command args
- converts commands into queue actions

It does not send messages or handle queue work.

## Dependencies

This package depends on:

- `internal/queue`
- `internal/workday`

## API

- `ParseAll(text string) []Command`
- `ActionFor(command Command, chat ChatContext) (queue.Action, error)`
- `SetOffFromWorkTimeUTC`

## Scope

This package owns:

- Telegram command parsing
- command arg parsing
- command to action mapping
- set-off-from-work command validation

## Validation

Action creation fails when:

- the command is not supported
- set-off-from-work args are invalid
