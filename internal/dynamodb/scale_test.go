package dynamodb

import (
	"context"
	"errors"
	"testing"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	gomock "go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mock "github.com/siutsin/telegram-jung2-bot/internal/mock"
)

func TestScaleUpperScaleUp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		describeOutput       *awsdynamodb.DescribeTableOutput
		describeErr          error
		desiredRead          int
		updateTableErr       error
		wantErrText          string
		wantUpdateTableInput *awsdynamodb.UpdateTableInput
	}{
		{
			name:                 "success updates target throughput",
			describeOutput:       describedTable(),
			desiredRead:          10,
			wantUpdateTableInput: updateTableInput(10),
		},
		{
			name:                 "ignored scale up error",
			describeOutput:       describedTable(),
			desiredRead:          10,
			updateTableErr:       errors.New("Subscriber limit exceeded"),
			wantUpdateTableInput: updateTableInput(10),
		},
		{
			name:                 "update error",
			describeOutput:       describedTable(),
			desiredRead:          10,
			updateTableErr:       errors.New("boom"),
			wantErrText:          "update DynamoDB table: boom",
			wantUpdateTableInput: updateTableInput(10),
		},
		{
			name:                 "zero desired read keeps current throughput",
			describeOutput:       describedTable(),
			wantUpdateTableInput: updateTableInput(1),
		},
		{
			name:        "describe error",
			describeErr: errors.New("boom"),
			wantErrText: "describe DynamoDB table: boom",
		},
		{
			name: "missing table description",
			describeOutput: &awsdynamodb.DescribeTableOutput{
				Table: nil,
			},
			wantErrText: "missing provisioned throughput",
		},
		{
			name: "missing provisioned throughput",
			describeOutput: &awsdynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{},
			},
			wantErrText: "missing provisioned throughput",
		},
		{
			name: "missing read capacity units",
			describeOutput: &awsdynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{
					ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
						WriteCapacityUnits: awscore.Int64(3),
					},
				},
			},
			wantErrText: "missing capacity units",
		},
		{
			name: "missing write capacity units",
			describeOutput: &awsdynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{
					ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
						ReadCapacityUnits: awscore.Int64(1),
					},
				},
			},
			wantErrText: "missing capacity units",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			dynamoClient := mock.NewMockDynamoRequester(controller)

			dynamoClient.EXPECT().
				DescribeTable(gomock.Any(), gomock.Any()).
				Return(test.describeOutput, test.describeErr)

			if test.wantUpdateTableInput != nil {
				dynamoClient.EXPECT().
					UpdateTable(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, input *awsdynamodb.UpdateTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateTableOutput, error) {
						assert.Equal(t, test.wantUpdateTableInput, input)
						if test.updateTableErr != nil {
							return nil, test.updateTableErr
						}
						return &awsdynamodb.UpdateTableOutput{}, nil
					})
			}

			err := NewScaleUpper(dynamoClient, "messages", test.desiredRead).ScaleUp(context.Background())

			if test.wantErrText != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrText)
				return
			}

			require.NoError(t, err)
		})
	}
}

func describedTable() *awsdynamodb.DescribeTableOutput {
	return &awsdynamodb.DescribeTableOutput{
		Table: &ddbtypes.TableDescription{
			ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
				ReadCapacityUnits:  awscore.Int64(1),
				WriteCapacityUnits: awscore.Int64(3),
			},
		},
	}
}

func updateTableInput(readCapacity int64) *awsdynamodb.UpdateTableInput {
	return &awsdynamodb.UpdateTableInput{
		TableName: awscore.String("messages"),
		ProvisionedThroughput: &ddbtypes.ProvisionedThroughput{
			ReadCapacityUnits:  awscore.Int64(readCapacity),
			WriteCapacityUnits: awscore.Int64(3),
		},
	}
}

func TestIsIgnoredScaleUpError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "subscriber limit", err: errors.New("Subscriber limit exceeded: daily limit"), want: true},
		{name: "no throughput change", err: errors.New("The provisioned throughput for the table will not change blah"), want: true},
		{name: "resource still in use", err: errors.New("Attempt to change a resource which is still in use blah"), want: true},
		{name: "other error", err: errors.New("Some other errors"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isIgnoredScaleUpError(test.err))
		})
	}
}
