package dynamodb

import (
	"fmt"
	"strconv"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
)

// encodeDynamoValues converts loose contract values into DynamoDB attributes.
// For example, map[string]any{":chatId": int64(42)} becomes an N attribute with
// value "42".
func encodeDynamoValues(values map[string]any) map[string]ddbtypes.AttributeValue {
	if len(values) == 0 {
		return nil
	}

	encoded := make(map[string]ddbtypes.AttributeValue, len(values))
	for name, value := range values {
		encoded[name] = encodeDynamoValue(value)
	}

	return encoded
}

// encodeDynamoValue converts one loose contract value into a DynamoDB attribute.
// For example, int64(42) becomes AttributeValueMemberN{"42"}.
func encodeDynamoValue(value any) ddbtypes.AttributeValue {
	switch typed := value.(type) {
	case bool:
		return &ddbtypes.AttributeValueMemberBOOL{Value: typed}
	case float64:
		return &ddbtypes.AttributeValueMemberN{Value: strconv.FormatFloat(typed, 'f', -1, 64)}
	case int:
		return &ddbtypes.AttributeValueMemberN{Value: strconv.Itoa(typed)}
	case int64:
		return &ddbtypes.AttributeValueMemberN{Value: strconv.FormatInt(typed, 10)}
	case string:
		return &ddbtypes.AttributeValueMemberS{Value: typed}
	default:
		return &ddbtypes.AttributeValueMemberS{Value: fmt.Sprint(value)}
	}
}

// decodeMessage converts one DynamoDB item into a stored message row.
// For example, an item with chatId and dateCreated becomes message.Message with
// parsed DateCreated.
func decodeMessage(item map[string]ddbtypes.AttributeValue) (message.Message, error) {
	timestamp, err := message.ParseDateCreated(stringAttribute(item, "dateCreated"))
	if err != nil {
		return message.Message{}, err
	}

	return message.Message{
		ChatID:      int64Attribute(item, "chatId"),
		ChatTitle:   stringAttribute(item, "chatTitle"),
		DateCreated: timestamp,
		FirstName:   stringAttribute(item, "firstName"),
		LastName:    stringAttribute(item, "lastName"),
		TTL:         int64Attribute(item, "ttl"),
		UserID:      int64Attribute(item, "userId"),
		Username:    stringAttribute(item, "username"),
	}, nil
}

// decodeChat converts one DynamoDB item into a chat row.
// For example, an item with offTime and workday becomes chat.Row with those
// fields populated.
func decodeChat(item map[string]ddbtypes.AttributeValue) chat.Row {
	return chat.Row{
		ChatID:        int64Attribute(item, "chatId"),
		ChatTitle:     stringAttribute(item, "chatTitle"),
		DateCreated:   stringAttribute(item, "dateCreated"),
		TTL:           int64Attribute(item, "ttl"),
		EnableAllJung: boolAttribute(item, "enableAllJung"),
		OffTime:       stringAttribute(item, "offTime"),
		Workday:       intAttribute(item, "workday"),
	}
}

// stringAttribute returns a string attribute when present.
// For example, an S attribute "Ops" returns "Ops".
func stringAttribute(item map[string]ddbtypes.AttributeValue, key string) string {
	attribute, ok := item[key]
	if !ok {
		return ""
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberS)
	if !ok {
		return ""
	}

	return value.Value
}

// int64Attribute returns an int64 attribute when present.
// For example, an N attribute "42" returns 42.
func int64Attribute(item map[string]ddbtypes.AttributeValue, key string) int64 {
	attribute, ok := item[key]
	if !ok {
		return 0
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberN)
	if !ok {
		return 0
	}

	parsed, err := strconv.ParseInt(value.Value, 10, 64)
	if err != nil {
		return 0
	}

	return parsed
}

// intAttribute returns an int attribute when present.
// For example, an N attribute "6" returns *int(6).
func intAttribute(item map[string]ddbtypes.AttributeValue, key string) *int {
	attribute, ok := item[key]
	if !ok {
		return nil
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberN)
	if !ok {
		return nil
	}

	parsed, err := strconv.Atoi(value.Value)
	if err != nil {
		return nil
	}

	return &parsed
}

// boolAttribute returns a bool attribute when present.
// For example, a BOOL attribute true returns *bool(true).
func boolAttribute(item map[string]ddbtypes.AttributeValue, key string) *bool {
	attribute, ok := item[key]
	if !ok {
		return nil
	}

	value, ok := attribute.(*ddbtypes.AttributeValueMemberBOOL)
	if !ok {
		return nil
	}

	parsed := value.Value
	return &parsed
}

// decodeLastEvaluatedKey converts DynamoDB pagination state into loose values.
// For example, {"chatId": N("42")} becomes {"chatId": int64(42)}.
func decodeLastEvaluatedKey(item map[string]ddbtypes.AttributeValue) map[string]any {
	if len(item) == 0 {
		return nil
	}

	decoded := make(map[string]any, len(item))
	for key, value := range item {
		switch typed := value.(type) {
		case *ddbtypes.AttributeValueMemberBOOL:
			decoded[key] = typed.Value
		case *ddbtypes.AttributeValueMemberN:
			parsed, err := strconv.ParseInt(typed.Value, 10, 64)
			if err == nil {
				decoded[key] = parsed
				continue
			}
			decoded[key] = typed.Value
		case *ddbtypes.AttributeValueMemberS:
			decoded[key] = typed.Value
		}
	}

	return decoded
}
