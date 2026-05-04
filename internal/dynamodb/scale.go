package dynamodb

import (
	"context"
	"fmt"
	"strings"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ScaleUp raises DynamoDB read capacity to the configured target.
func (service ScaleUpper) ScaleUp(ctx context.Context) error {
	output, err := service.dynamo.DescribeTable(ctx, &awsdynamodb.DescribeTableInput{
		TableName: awscore.String(service.tableName),
	})
	if err != nil {
		return fmt.Errorf("describe DynamoDB table: %w", err)
	}
	if output.Table == nil || output.Table.ProvisionedThroughput == nil {
		return fmt.Errorf("describe DynamoDB table: missing provisioned throughput")
	}

	throughput := output.Table.ProvisionedThroughput
	if throughput.ReadCapacityUnits == nil || throughput.WriteCapacityUnits == nil {
		return fmt.Errorf("describe DynamoDB table: missing capacity units")
	}
	readCapacity := awscore.ToInt64(throughput.ReadCapacityUnits)
	if service.desiredRead > 0 {
		readCapacity = int64(service.desiredRead)
	}

	_, err = service.dynamo.UpdateTable(ctx, &awsdynamodb.UpdateTableInput{
		TableName: awscore.String(service.tableName),
		ProvisionedThroughput: &ddbtypes.ProvisionedThroughput{
			ReadCapacityUnits:  awscore.Int64(readCapacity),
			WriteCapacityUnits: throughput.WriteCapacityUnits,
		},
	})
	if err != nil {
		if isIgnoredScaleUpError(err) {
			return nil
		}
		return fmt.Errorf("update DynamoDB table: %w", err)
	}

	return nil
}

// isIgnoredScaleUpError reports whether a scale-up error is ignorable.
func isIgnoredScaleUpError(err error) bool {
	if err == nil {
		return false
	}

	errorText := err.Error()
	return strings.Contains(errorText, "Subscriber limit exceeded") ||
		strings.Contains(errorText, "The provisioned throughput for the table will not change") ||
		strings.Contains(errorText, "Attempt to change a resource which is still in use")
}
