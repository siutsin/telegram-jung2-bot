# Gotchas and Known Issues

## Go Service

### Preserve both SQS message attribute casings

Existing SQS producers may use different casing for message attributes:

- Lambda: `messageAttributes.action.stringValue`
- Legacy ECS: `messageAttributes.action.StringValue`

Keep a small helper for reading either form before dispatching an action.

### Keep DynamoDB pagination explicit

Statistics queries must keep reading pages until `LastEvaluatedKey` is absent.
Missing pagination silently under-counts active chats.

### Preserve existing date formats

Message rows use `dateCreated` strings with the existing UTC+8 offset format.
Go parsers must accept the existing stored format before normalising time
internally.

### Keep Telegram responses below the safety limit

Telegram allows 4096 characters, but this project uses a 3800 character limit to
avoid UTF-8 and formatting edge cases. Apply truncation after rendering the final
message.
