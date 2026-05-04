package dynamodb

import (
	"context"
	"fmt"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type itemUpdateRequest struct {
	TableName                 string
	Key                       map[string]any
	UpdateExpression          string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]any
}

// updateItem applies a contract update expression in DynamoDB.
// For example, a request with Key{"chatId": 42} becomes one UpdateItem call
// with DynamoDB-encoded key and values.
func updateItem(ctx context.Context, dynamoClient dynamoRequester, updateRequest itemUpdateRequest) error {
	_, err := dynamoClient.UpdateItem(ctx, &awsdynamodb.UpdateItemInput{
		TableName:                 awscore.String(updateRequest.TableName),
		Key:                       encodeDynamoValues(updateRequest.Key),
		UpdateExpression:          awscore.String(updateRequest.UpdateExpression),
		ExpressionAttributeNames:  updateRequest.ExpressionAttributeNames,
		ExpressionAttributeValues: encodeDynamoValues(updateRequest.ExpressionAttributeValues),
	})
	if err != nil {
		return fmt.Errorf("update DynamoDB item: %w", err)
	}

	return nil
}

// updateContractUpdate applies one SDK-free contract update shape in DynamoDB.
// For example, a contract update with Key{"chatId": 42} becomes one UpdateItem
// call with DynamoDB-encoded key and values.
// It stays as a tiny adapter because message and chat packages return SDK-free
// update shapes, while this package owns the final DynamoDB call.
func updateContractUpdate(
	ctx context.Context,
	dynamoClient dynamoRequester,
	tableName string,
	key map[string]any,
	updateExpression string,
	expressionAttributeNames map[string]string,
	expressionAttributeValues map[string]any,
) error {
	return updateItem(ctx, dynamoClient, itemUpdateRequest{
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		Key:                       key,
		TableName:                 tableName,
		UpdateExpression:          updateExpression,
	})
}
