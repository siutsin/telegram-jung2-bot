package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	gomock "go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	mock "github.com/siutsin/telegram-jung2-bot/internal/mock"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

func TestChatClientGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		output      *awsdynamodb.GetItemOutput
		getErr      error
		wantFound   bool
		want        chat.ChatSetting
		wantErrText string
	}{
		{name: "missing", output: &awsdynamodb.GetItemOutput{}, wantFound: false},
		{
			name: "success",
			output: &awsdynamodb.GetItemOutput{Item: map[string]ddbtypes.AttributeValue{
				"chatId":        &ddbtypes.AttributeValueMemberN{Value: "123"},
				"dateCreated":   &ddbtypes.AttributeValueMemberS{Value: "2026-05-02T20:30:00+08:00"},
				"enableAllJung": &ddbtypes.AttributeValueMemberBOOL{Value: true},
			}},
			wantFound: true,
			want: chat.ChatSetting{
				ChatID:        123,
				DateCreated:   time.Date(2026, 5, 2, 20, 30, 0, 0, time.FixedZone("", 8*60*60)),
				EnableAllJung: true,
			},
		},
		{name: "sdk error", getErr: errors.New("boom"), wantErrText: "get DynamoDB chat row: boom"},
		{
			name: "parse error",
			output: &awsdynamodb.GetItemOutput{Item: map[string]ddbtypes.AttributeValue{
				"dateCreated": &ddbtypes.AttributeValueMemberS{Value: "bad"},
			}},
			wantErrText: "parse DynamoDB chat row",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)
			dynamoClient.EXPECT().
				GetItem(gomock.Any(), gomock.Any()).
				Return(test.output, test.getErr)

			settings, found, err := NewChatClient(dynamoClient).Get(context.Background(), "chats", 123)

			if test.wantErrText != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantFound, found)
			assert.Equal(t, test.want, settings)
		})
	}
}

func TestChatClientUpdateAndSaveStatistics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(context.Context, ChatClient) error
	}{
		{
			name: "update",
			run: func(ctx context.Context, client ChatClient) error {
				return client.Update(ctx, chat.BuildAllJungUpdate("chats", 123, false))
			},
		},
		{
			name: "save",
			run: func(ctx context.Context, client ChatClient) error {
				return client.Save(ctx, "chats", chat.ChatSetting{
					ChatID:      123,
					ChatTitle:   "Ops",
					DateCreated: time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC),
					TTL:         456,
				})
			},
		},
		{
			name: "save statistics",
			run: func(ctx context.Context, client ChatClient) error {
				return client.SaveStatistics(ctx, "chats", 123, 2, 5, time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)
			dynamoClient.EXPECT().
				UpdateItem(gomock.Any(), gomock.Any()).
				Return(&awsdynamodb.UpdateItemOutput{}, nil)

			require.NoError(t, test.run(context.Background(), NewChatClient(dynamoClient)))
		})
	}
}

func TestChatClientListEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		scanErr     error
		want        []chat.ChatSetting
		wantErrText string
	}{
		{
			name: "success",
			want: []chat.ChatSetting{{ChatID: 123, EnableAllJung: true}},
		},
		{name: "scan error", scanErr: errors.New("boom"), wantErrText: "scan DynamoDB chat rows: boom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)
			dynamoClient.EXPECT().
				Scan(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, input *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error) {
					assert.Equal(t, "chats", awscore.ToString(input.TableName))
					if test.scanErr != nil {
						return nil, test.scanErr
					}
					return &awsdynamodb.ScanOutput{Items: []map[string]ddbtypes.AttributeValue{
						{"chatId": &ddbtypes.AttributeValueMemberN{Value: "123"}},
					}}, nil
				})

			rows, err := NewChatClient(dynamoClient).ListEnabled(context.Background(), "chats")

			if test.wantErrText != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, rows)
		})
	}
}

func TestChatClientDueChatIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		items       []map[string]ddbtypes.AttributeValue
		scanErr     error
		want        []int64
		wantErrText string
	}{
		{name: "success", items: dueChatItems(), want: []int64{123}},
		{name: "scan error", scanErr: errors.New("boom"), wantErrText: "scan due chats: boom"},
		{
			name: "matching row without chat id",
			items: []map[string]ddbtypes.AttributeValue{
				{
					"offTime": &ddbtypes.AttributeValueMemberS{Value: "1000"},
					"workday": &ddbtypes.AttributeValueMemberN{Value: "62"},
				},
			},
			wantErrText: "due chat row missing chatId",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)
			dynamoClient.EXPECT().
				Scan(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, input *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error) {
					assert.Equal(t, "chats", awscore.ToString(input.TableName))
					if test.scanErr != nil {
						return nil, test.scanErr
					}
					return &awsdynamodb.ScanOutput{Items: test.items}, nil
				})

			chatIDs, err := NewChatClient(dynamoClient).DueChatIDs(context.Background(), "chats", time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC))

			if test.wantErrText != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrText)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, chatIDs)
		})
	}
}

func dueChatItems() []map[string]ddbtypes.AttributeValue {
	return []map[string]ddbtypes.AttributeValue{
		{
			"chatId":  &ddbtypes.AttributeValueMemberN{Value: "123"},
			"offTime": &ddbtypes.AttributeValueMemberS{Value: "1000"},
			"workday": &ddbtypes.AttributeValueMemberN{Value: "62"},
		},
		{
			"chatId":  &ddbtypes.AttributeValueMemberN{Value: "456"},
			"offTime": &ddbtypes.AttributeValueMemberS{Value: "1000"},
			"workday": &ddbtypes.AttributeValueMemberN{Value: "64"},
		},
	}
}

func TestDueScanRowMatches(t *testing.T) {
	t.Parallel()

	monWithUnknownBits := workday.Mon | 128
	tuesday := workday.Tue
	tests := []struct {
		name string
		row  chat.Row
		day  string
		want bool
	}{
		{name: "missing defaults with empty off time", row: chat.Row{}, day: "MON", want: true},
		{name: "missing rejects non-empty off time", row: chat.Row{OffTime: "1800"}, day: "MON", want: false},
		{name: "workday match masks unknown bits", row: chat.Row{Workday: &monWithUnknownBits}, day: "MON", want: true},
		{name: "workday miss", row: chat.Row{Workday: &tuesday}, day: "MON", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, dueScanRowMatches(test.row, test.day))
		})
	}
}

func TestScanDueChatsRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		offTime    string
		weekday    string
		wantFilter string
		wantNames  map[string]string
		wantValues map[string]any
	}{
		{
			name:       "custom off time",
			offTime:    "1800",
			weekday:    "MON",
			wantFilter: "#ot = :ot",
			wantNames:  map[string]string{"#ot": "offTime"},
			wantValues: map[string]any{":ot": "1800"},
		},
		{
			name:       "default weekday includes fallback rows",
			offTime:    "1000",
			weekday:    "FRI",
			wantFilter: "#ot = :ot Or (attribute_not_exists(#ot) And attribute_not_exists(#wd))",
			wantNames: map[string]string{
				"#ot": "offTime",
				"#wd": "workday",
			},
			wantValues: map[string]any{":ot": "1000"},
		},
		{
			name:       "default weekend skips fallback rows",
			offTime:    "1000",
			weekday:    "SAT",
			wantFilter: "#ot = :ot",
			wantNames:  map[string]string{"#ot": "offTime"},
			wantValues: map[string]any{":ot": "1000"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scanRequest := scanDueChatsRequest("chats", test.offTime, test.weekday)

			assert.Equal(t, "chats", scanRequest.tableName)
			assert.Equal(t, test.wantFilter, scanRequest.filterExpression)
			assert.Equal(t, test.wantNames, scanRequest.expressionAttributeNames)
			assert.Equal(t, test.wantValues, scanRequest.expressionAttributeValues)
		})
	}
}

func TestBuildChatCountUpdate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 30, 0, 0, time.UTC)
	updateRequest := buildChatCountUpdate("chats", 123, 2, 5, now)

	assert.Equal(t, itemUpdateRequest{
		tableName:                 "chats",
		key:                       map[string]any{"chatId": int64(123)},
		updateExpression:          "SET #uc = :uc, #mc = :mc, #mpu = :mpu, #ct = :ct",
		expressionAttributeNames:  map[string]string{"#uc": "userCount", "#mc": "messageCount", "#mpu": "messagePerUser", "#ct": "countTimestamp"},
		expressionAttributeValues: map[string]any{":uc": 2, ":mc": 5, ":mpu": 2.5, ":ct": "2026-05-02T20:30:00+08:00"},
	}, updateRequest)
}
