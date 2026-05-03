// Package dynamodb owns SDK-free DynamoDB request shapes and pagination helpers.
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/message"
)

const (
	ChatPartitionKey    = "chatId"
	MessagePartitionKey = "chatId"
	MessageSortKey      = "dateCreated"

	ChatTitleAttribute      = "chatTitle"
	CountTimestampAttribute = "countTimestamp"
	DateCreatedAttribute    = "dateCreated"
	EnableAllJungAttribute  = "enableAllJung"
	FirstNameAttribute      = "firstName"
	LastNameAttribute       = "lastName"
	MessageCountAttribute   = "messageCount"
	MessagePerUserAttribute = "messagePerUser"
	OffTimeAttribute        = "offTime"
	TTLAttribute            = "ttl"
	UserCountAttribute      = "userCount"
	UserIDAttribute         = "userId"
	UsernameAttribute       = "username"
	WorkdayAttribute        = "workday"

	defaultOffTime = "1000"
)

var defaultWeekdays = map[string]struct{}{
	"MON": {},
	"TUE": {},
	"WED": {},
	"THU": {},
	"FRI": {},
}

type Request struct {
	TableName                 string
	Key                       map[string]any
	KeyConditionExpression    string
	FilterExpression          string
	ScanIndexForward          bool
	ExclusiveStartKey         map[string]any
	UpdateExpression          string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]any
	ProvisionedThroughput     Throughput
}

type Throughput struct {
	ReadCapacityUnits  int64
	WriteCapacityUnits int64
}

type Page[T any] struct {
	Items            []T
	LastEvaluatedKey map[string]any
}

type FetchPage[T any] func(ctx context.Context, exclusiveStartKey map[string]any) (Page[T], error)

// CollectPages accumulates paginated DynamoDB results.
func CollectPages[T any](ctx context.Context, fetch FetchPage[T]) ([]T, error) {
	var rows []T
	var startKey map[string]any

	for {
		err := ctx.Err()
		if err != nil {
			return nil, fmt.Errorf("collect dynamodb pages: %w", err)
		}

		page, err := fetch(ctx, startKey)
		if err != nil {
			return nil, fmt.Errorf("collect dynamodb pages: %w", err)
		}
		rows = append(rows, page.Items...)
		if len(page.LastEvaluatedKey) == 0 {
			return rows, nil
		}
		startKey = page.LastEvaluatedKey
	}
}

// QueryMessagesRequest builds a message query request.
func QueryMessagesRequest(tableName string, chatID int64, now time.Time, days int, startKey map[string]any) Request {
	return Request{
		TableName:              tableName,
		KeyConditionExpression: "chatId = :chat_id AND dateCreated > :date_created",
		ScanIndexForward:       false,
		ExclusiveStartKey:      startKey,
		ExpressionAttributeValues: map[string]any{
			":chat_id":      chatID,
			":date_created": message.FormatDateCreated(now.AddDate(0, 0, -days)),
		},
	}
}

// QueryChatStatsRequest builds a chat statistics query request.
func QueryChatStatsRequest(tableName string, chatID int64) Request {
	return Request{
		TableName:              tableName,
		KeyConditionExpression: "chatId = :chat_id",
		ExpressionAttributeValues: map[string]any{
			":chat_id": chatID,
		},
	}
}

// ScanDueChatsRequest builds a due-chat scan request.
func ScanDueChatsRequest(tableName string, offTime string, weekday string, startKey map[string]any) Request {
	names := map[string]string{
		"#ot": OffTimeAttribute,
	}
	filterExpression := "#ot = :ot"
	if isDefaultOffWorkScan(offTime, weekday) {
		filterExpression += " Or (attribute_not_exists(#ot) And attribute_not_exists(#wd))"
		names["#wd"] = WorkdayAttribute
	}

	return Request{
		TableName:                tableName,
		FilterExpression:         filterExpression,
		ExclusiveStartKey:        startKey,
		ExpressionAttributeNames: names,
		ExpressionAttributeValues: map[string]any{
			":ot": offTime,
		},
	}
}

// BuildChatCountUpdate builds a chat statistics update request.
func BuildChatCountUpdate(tableName string, chatID int64, userCount int, messageCount int, now time.Time) Request {
	return Request{
		TableName:        tableName,
		Key:              map[string]any{ChatPartitionKey: chatID},
		UpdateExpression: "SET #uc = :uc, #mc = :mc, #mpu = :mpu, #ct = :ct",
		ExpressionAttributeNames: map[string]string{
			"#uc":  UserCountAttribute,
			"#mc":  MessageCountAttribute,
			"#mpu": MessagePerUserAttribute,
			"#ct":  CountTimestampAttribute,
		},
		ExpressionAttributeValues: map[string]any{
			":uc":  userCount,
			":mc":  messageCount,
			":mpu": float64(messageCount) / float64(userCount),
			":ct":  message.FormatDateCreated(now),
		},
	}
}

// BuildScaleUpRequest builds a table throughput update request.
func BuildScaleUpRequest(tableName string, currentRead int64, currentWrite int64, desiredReadRaw string) Request {
	desiredRead := currentRead
	parsed, err := parsePositiveInt64(desiredReadRaw)
	if err == nil {
		desiredRead = parsed
	}

	return Request{
		TableName: tableName,
		ProvisionedThroughput: Throughput{
			ReadCapacityUnits:  desiredRead,
			WriteCapacityUnits: currentWrite,
		},
	}
}

// IsIgnoredScaleUpError reports whether a scale-up error is ignorable.
func IsIgnoredScaleUpError(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "Subscriber limit exceeded") ||
		strings.Contains(message, "The provisioned throughput for the table will not change") ||
		strings.Contains(message, "Attempt to change a resource which is still in use")
}

// SanitisedLogFields returns request fields safe to log.
func SanitisedLogFields(request Request) map[string]any {
	fields := map[string]any{
		"tableName": request.TableName,
	}
	if len(request.Key) > 0 {
		fields["key"] = request.Key
	}
	if request.KeyConditionExpression != "" {
		fields["keyConditionExpression"] = request.KeyConditionExpression
	}
	if request.FilterExpression != "" {
		fields["filterExpression"] = request.FilterExpression
	}
	if len(request.ExclusiveStartKey) > 0 {
		fields["hasExclusiveStartKey"] = true
	}

	return fields
}

// isDefaultOffWorkScan reports whether a scan matches the contract defaults.
func isDefaultOffWorkScan(offTime string, weekday string) bool {
	if offTime != defaultOffTime {
		return false
	}
	_, ok := defaultWeekdays[weekday]
	return ok
}

// parsePositiveInt64 parses a positive integer string.
func parsePositiveInt64(raw string) (int64, error) {
	var result int64
	for _, digit := range raw {
		if digit < '0' || digit > '9' {
			return 0, errors.New("not a positive integer")
		}
		result = result*10 + int64(digit-'0')
	}
	if result == 0 {
		return 0, errors.New("not a positive integer")
	}

	return result, nil
}
