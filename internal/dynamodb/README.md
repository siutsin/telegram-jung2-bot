# `internal/dynamodb`

## Purpose

This package adapts the internal storage contracts to DynamoDB.

It:

- builds DynamoDB request shapes
- encodes and decodes DynamoDB values
- handles DynamoDB pagination
- implements the concrete chat, message, and scale-up adapters

It does not own chat, message, or schedule rules.

## Dependencies

This package depends on:

- `internal` (`bot`)
- AWS DynamoDB SDK

## Flow

### Message flow

```mermaid
flowchart TD
    row[Message or query input] --> client[message adapter]
    client --> request[DynamoDB request]
    request --> dynamo[DynamoDB]
    dynamo --> decode[Decoded message rows]
```

- the message adapter saves message rows and queries stored message history.
- Query methods keep following `LastEvaluatedKey` until DynamoDB is done.

### Chat flow

```mermaid
flowchart TD
    row[Stored DynamoDB item] --> decode[decodeChat]
    decode --> parse[chat.ParseRow or chat.FromScheduleRow]
    parse --> setting[ChatSetting]
```

- the chat adapter `Get` path uses the strict `chat.ParseRow` path.
- schedule-facing scans use the permissive `chat.FromScheduleRow` path.

### Scale-up flow

```mermaid
flowchart TD
    scaleUp[scale-up adapter] --> describe[DescribeTable]
    describe --> target[choose read capacity]
    target --> update[UpdateTable]
```

- the scale-up adapter reads current throughput, chooses the target read capacity, then updates the table.
- some known DynamoDB scale-up errors are ignored to match the deployed behaviour.

## Scope

This package owns:

- DynamoDB client adapters
- DynamoDB request encoding
- DynamoDB item decoding
- pagination loops

## Validation

Calls fail when:

- a DynamoDB SDK call fails
- a stored chat row is malformed on the strict read path
- a stored message row cannot be decoded

## Fallbacks

These do not fail:

- known ignorable scale-up errors
