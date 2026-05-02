# Gotchas and Known Issues

## Go Service

### Keep Go code in the executable and private package trees

Do not reintroduce root-level Go package directories. The service executable
lives under `cmd/telegram-jung2-bot`, and private packages live under
`internal/`. Buck2 target visibility should stay explicit even where Go import
visibility also applies.

### Preserve both SQS message attribute casings

Existing SQS producers may use different casing for message attributes:

- Lambda: `messageAttributes.action.stringValue`
- ECS polling: `messageAttributes.action.StringValue`

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
