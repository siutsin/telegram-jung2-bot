package dynamodb

import (
	"context"
	"fmt"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

type dueChatScanRequest struct {
	tableName                 string
	filterExpression          string
	expressionAttributeNames  map[string]string
	expressionAttributeValues map[string]any
}

// Get loads one stored chat row.
func (client ChatClient) Get(ctx context.Context, tableName string, chatID int64) (chat.ChatSetting, bool, error) {
	output, err := client.dynamo.GetItem(ctx, &awsdynamodb.GetItemInput{
		TableName: awscore.String(tableName),
		Key:       encodeDynamoValues(map[string]any{"chatId": chatID}),
	})
	if err != nil {
		return chat.ChatSetting{}, false, fmt.Errorf("get DynamoDB chat row: %w", err)
	}
	if len(output.Item) == 0 {
		return chat.ChatSetting{}, false, nil
	}

	settings, err := chat.ParseRow(decodeChat(output.Item))
	if err != nil {
		return chat.ChatSetting{}, false, fmt.Errorf("parse DynamoDB chat row: %w", err)
	}

	return settings, true, nil
}

// Update stores a chat setting update expression.
func (client ChatClient) Update(ctx context.Context, update chat.UpdateExpression) error {
	return updateContractUpdate(ctx, client.dynamo, update.TableName, update.Key, update.UpdateExpression, update.ExpressionAttributeNames, update.ExpressionAttributeValues)
}

// Save stores a chat record row.
func (client ChatClient) Save(ctx context.Context, tableName string, settings chat.ChatSetting) error {
	metadataUpdate := chat.BuildMetadataUpdate(tableName, settings)
	return updateContractUpdate(ctx, client.dynamo, metadataUpdate.TableName, metadataUpdate.Key, metadataUpdate.UpdateExpression, metadataUpdate.ExpressionAttributeNames, metadataUpdate.ExpressionAttributeValues)
}

// ListEnabled scans stored chat rows for scheduling.
func (client ChatClient) ListEnabled(ctx context.Context, tableName string) ([]chat.ChatSetting, error) {
	return collectPages(ctx, func(pageCtx context.Context, startKey map[string]any) (page[chat.ChatSetting], error) {
		output, err := client.dynamo.Scan(pageCtx, &awsdynamodb.ScanInput{
			TableName:         awscore.String(tableName),
			ExclusiveStartKey: encodeDynamoValues(startKey),
		})
		if err != nil {
			return page[chat.ChatSetting]{}, fmt.Errorf("scan DynamoDB chat rows: %w", err)
		}

		rows := make([]chat.ChatSetting, 0, len(output.Items))
		for _, item := range output.Items {
			rows = append(rows, chat.FromScheduleRow(decodeChat(item)))
		}
		return page[chat.ChatSetting]{
			Items:            rows,
			LastEvaluatedKey: decodeLastEvaluatedKey(output.LastEvaluatedKey),
		}, nil
	})
}

// SaveStatistics stores the computed group counts in the chat row.
func (client ChatClient) SaveStatistics(ctx context.Context, tableName string, chatID int64, userCount int, messageCount int, now time.Time) error {
	return updateItem(ctx, client.dynamo, buildChatCountUpdate(tableName, chatID, userCount, messageCount, now))
}

// DueChatIDs scans and filters due chats for one scheduled window.
func (client ChatClient) DueChatIDs(ctx context.Context, tableName string, timestamp time.Time) ([]int64, error) {
	window := schedule.WindowFromTime(timestamp)
	scanRequest := scanDueChatsRequest(tableName, window.OffTime, window.Weekday)
	return collectPages(ctx, func(pageCtx context.Context, startKey map[string]any) (page[int64], error) {
		output, err := client.dynamo.Scan(pageCtx, &awsdynamodb.ScanInput{
			TableName:                 awscore.String(scanRequest.tableName),
			ExclusiveStartKey:         encodeDynamoValues(startKey),
			FilterExpression:          awscore.String(scanRequest.filterExpression),
			ExpressionAttributeNames:  scanRequest.expressionAttributeNames,
			ExpressionAttributeValues: encodeDynamoValues(scanRequest.expressionAttributeValues),
		})
		if err != nil {
			return page[int64]{}, fmt.Errorf("scan due chats: %w", err)
		}

		chatIDs := make([]int64, 0, len(output.Items))
		for _, item := range output.Items {
			row := decodeChat(item)
			if dueScanRowMatches(row, window.Weekday) {
				if row.ChatID == 0 {
					return page[int64]{}, fmt.Errorf("due chat row missing chatId")
				}
				chatIDs = append(chatIDs, row.ChatID)
			}
		}
		return page[int64]{
			Items:            chatIDs,
			LastEvaluatedKey: decodeLastEvaluatedKey(output.LastEvaluatedKey),
		}, nil
	})
}

// dueScanRowMatches applies the deployed post-scan weekday filter.
// For example, a row with MON|TUE matches day "MON" but not "SUN".
func dueScanRowMatches(row chat.Row, day string) bool {
	if row.Workday == nil {
		return row.OffTime == ""
	}

	return workday.MatchesDay(day, workday.Workdays(*row.Workday&int(workday.AllDays)))
}

// scanDueChatsRequest builds a due-chat scan request.
// For example, offTime "1000" on "MON" also includes rows where offTime and
// workday are both missing.
func scanDueChatsRequest(tableName string, offTime string, weekday string) dueChatScanRequest {
	names := map[string]string{
		"#ot": "offTime",
	}
	filterExpression := "#ot = :ot"
	if isDefaultOffWorkScan(offTime, weekday) {
		filterExpression += " Or (attribute_not_exists(#ot) And attribute_not_exists(#wd))"
		names["#wd"] = "workday"
	}

	return dueChatScanRequest{
		tableName:                tableName,
		filterExpression:         filterExpression,
		expressionAttributeNames: names,
		expressionAttributeValues: map[string]any{
			":ot": offTime,
		},
	}
}

// messagePerUser computes the stored average without dividing by zero.
// For example, userCount 5 and messageCount 20 becomes 4.
func messagePerUser(userCount int, messageCount int) float64 {
	if userCount == 0 {
		return 0
	}

	return float64(messageCount) / float64(userCount)
}

// buildChatCountUpdate builds a chat statistics update request.
// For example, userCount 5 and messageCount 20 stores messagePerUser as 4.
func buildChatCountUpdate(tableName string, chatID int64, userCount int, messageCount int, now time.Time) itemUpdateRequest {
	return itemUpdateRequest{
		tableName:        tableName,
		key:              map[string]any{"chatId": chatID},
		updateExpression: "SET #uc = :uc, #mc = :mc, #mpu = :mpu, #ct = :ct",
		expressionAttributeNames: map[string]string{
			"#uc":  "userCount",
			"#mc":  "messageCount",
			"#mpu": "messagePerUser",
			"#ct":  "countTimestamp",
		},
		expressionAttributeValues: map[string]any{
			":uc":  userCount,
			":mc":  messageCount,
			":mpu": messagePerUser(userCount, messageCount),
			":ct":  message.FormatDateCreated(now),
		},
	}
}

// isDefaultOffWorkScan reports whether a scan matches the contract defaults.
// For example, offTime "1000" and weekday "MON" matches the default scan.
func isDefaultOffWorkScan(offTime string, weekday string) bool {
	if offTime != "1000" {
		return false
	}
	return workday.MatchesDay(weekday, workday.Weekdays)
}
