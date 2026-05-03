package dynamodb

import (
	"context"
	"errors"
	"testing"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

func TestScaleUpperIgnoresKnownErrors(t *testing.T) {
	t.Parallel()

	err := (ScaleUpper{
		Dynamo: &fakeAPI{
			describeOutput: &awsdynamodb.DescribeTableOutput{
				Table: &ddbtypes.TableDescription{
					ProvisionedThroughput: &ddbtypes.ProvisionedThroughputDescription{
						ReadCapacityUnits:  awscore.Int64(1),
						WriteCapacityUnits: awscore.Int64(3),
					},
				},
			},
			updateTableErr: errors.New("Subscriber limit exceeded"),
		},
		DesiredRead: 10,
		TableName:   "messages",
	}).ScaleUp(context.Background())

	require.NoError(t, err)
}

type fakeAPI struct {
	describeOutput *awsdynamodb.DescribeTableOutput
	updateTableErr error
}

func (api *fakeAPI) DescribeTable(ctx context.Context, params *awsdynamodb.DescribeTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.DescribeTableOutput, error) {
	if api.describeOutput != nil {
		return api.describeOutput, nil
	}
	return &awsdynamodb.DescribeTableOutput{}, nil
}

func (api *fakeAPI) GetItem(ctx context.Context, params *awsdynamodb.GetItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.GetItemOutput, error) {
	return &awsdynamodb.GetItemOutput{}, nil
}

func (api *fakeAPI) Query(ctx context.Context, params *awsdynamodb.QueryInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.QueryOutput, error) {
	return &awsdynamodb.QueryOutput{}, nil
}

func (api *fakeAPI) Scan(ctx context.Context, params *awsdynamodb.ScanInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.ScanOutput, error) {
	return &awsdynamodb.ScanOutput{}, nil
}

func (api *fakeAPI) UpdateItem(ctx context.Context, params *awsdynamodb.UpdateItemInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateItemOutput, error) {
	return &awsdynamodb.UpdateItemOutput{}, nil
}

func (api *fakeAPI) UpdateTable(ctx context.Context, params *awsdynamodb.UpdateTableInput, optFns ...func(*awsdynamodb.Options)) (*awsdynamodb.UpdateTableOutput, error) {
	if api.updateTableErr != nil {
		return nil, api.updateTableErr
	}
	return &awsdynamodb.UpdateTableOutput{}, nil
}
