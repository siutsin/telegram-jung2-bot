package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	awscore "github.com/aws/aws-sdk-go-v2/aws"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appdynamodb "github.com/siutsin/telegram-jung2-bot/internal/dynamodb"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
)

func runStageHTTPIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	provisionedTable, cleanupProvisioned := createProvisionedChatTable(t, ctx, dynamoClient)
	if cleanupProvisioned != nil {
		t.Cleanup(cleanupProvisioned)
	}

	scaleUpper := appdynamodb.NewScaleUpper(dynamoClient, provisionedTable, 2)
	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{
		stage:      integrationStage,
		scaleUpper: scaleUpper,
	})
	stagePrefix := "/jung2bot/" + integrationStage

	t.Run("ping", func(t *testing.T) {
		response := doHTTP(t, ctx, http.MethodGet, httpServer.baseURL+stagePrefix+"/ping", "")
		defer func() {
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Errorf("close HTTP response body: %v", closeErr)
			}
		}()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, `{"health":"ok"}`, readResponseBody(t, response))
	})

	t.Run("webhook", func(t *testing.T) {
		const (
			stageChatID    int64 = 42014
			stageChatTitle       = "Stage Webhook"
		)

		response := doHTTP(
			t,
			ctx,
			http.MethodPost,
			httpServer.baseURL+stagePrefix+"/",
			webhookPlainPayload(stageChatID, stageChatTitle),
		)
		defer func() {
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Errorf("close HTTP response body: %v", closeErr)
			}
		}()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, `{"statusCode":200}`, readResponseBody(t, response))
		assertWebhookChatRow(t, ctx, dynamoClient, resources.chatTable, stageChatID, stageChatTitle)
	})

	t.Run("onOffFromWork", func(t *testing.T) {
		response := doHTTP(
			t,
			ctx,
			http.MethodGet,
			httpServer.baseURL+stagePrefix+"/onOffFromWork?timeString=2026-06-11T18:30:00Z",
			"",
		)
		defer func() {
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Errorf("close HTTP response body: %v", closeErr)
			}
		}()
		assert.Equal(t, http.StatusAccepted, response.StatusCode)
		assert.JSONEq(t, `{"onOffFromWork":"ok"}`, readResponseBody(t, response))

		queueResponse, err := receiveOne(ctx, httpServer.queueClient, httpServer.queueURL)
		require.NoError(t, err, "receive onOffFromWork queue message")

		gotAction := queue.DecodeMessage(queueResponse.Messages[0])
		wantAction := schedule.BuildOnOffFromWorkAction("2026-06-11T18:30:00Z")
		assertAction(t, wantAction, gotAction)

		err = httpServer.queueClient.Delete(ctx, queue.DeleteMessageRequest{
			QueueURL:      httpServer.queueURL,
			ReceiptHandle: queueResponse.Messages[0].ReceiptHandle,
		})
		require.NoError(t, err, "delete onOffFromWork queue message")
	})

	t.Run("onScaleUp", func(t *testing.T) {
		response := doHTTP(t, ctx, http.MethodGet, httpServer.baseURL+stagePrefix+"/onScaleUp", "")
		defer func() {
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Errorf("close HTTP response body: %v", closeErr)
			}
		}()
		assert.Equal(t, http.StatusOK, response.StatusCode)
		assert.JSONEq(t, `{"onScaleUp":"ok"}`, readResponseBody(t, response))
	})

	runStageSchedulerAuthIntegration(t, ctx, dynamoClient, sqsClient, resources, scaleUpper)
}

func runStageSchedulerAuthIntegration(
	t *testing.T,
	ctx context.Context,
	dynamoClient *awsdynamodb.Client,
	sqsClient *awssqs.Client,
	resources testResources,
	scaleUpper appdynamodb.ScaleUpper,
) {
	t.Helper()

	const schedulerSecret = "integration-scheduler-secret"
	httpServer := buildIntegrationHTTPServer(t, dynamoClient, sqsClient, resources, integrationServerOptions{
		stage:                integrationStage,
		scaleUpper:           scaleUpper,
		schedulerSecretToken: schedulerSecret,
	})
	stagePrefix := "/jung2bot/" + integrationStage

	t.Run("schedulerAuthRejectsMissingToken", func(t *testing.T) {
		response := doHTTP(
			t,
			ctx,
			http.MethodGet,
			httpServer.baseURL+stagePrefix+"/onOffFromWork?timeString=2026-06-11T18:30:00Z",
			"",
		)
		defer func() {
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Errorf("close HTTP response body: %v", closeErr)
			}
		}()
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.JSONEq(t, `{"onOffFromWork":"unauthorised"}`, readResponseBody(t, response))
	})

	t.Run("schedulerAuthAcceptsQueryToken", func(t *testing.T) {
		response := doHTTP(
			t,
			ctx,
			http.MethodGet,
			httpServer.baseURL+stagePrefix+"/onOffFromWork?timeString=2026-06-11T18:30:00Z&schedulerToken="+schedulerSecret,
			"",
		)
		defer func() {
			closeErr := response.Body.Close()
			if closeErr != nil {
				t.Errorf("close HTTP response body: %v", closeErr)
			}
		}()
		assert.Equal(t, http.StatusAccepted, response.StatusCode)
		assert.JSONEq(t, `{"onOffFromWork":"ok"}`, readResponseBody(t, response))

		err := httpServer.queueClient.Delete(ctx, queue.DeleteMessageRequest{
			QueueURL:      httpServer.queueURL,
			ReceiptHandle: mustReceiveOnOffFromWorkReceipt(t, ctx, httpServer),
		})
		require.NoError(t, err, "delete authorised onOffFromWork queue message")
	})
}

func mustReceiveOnOffFromWorkReceipt(
	t *testing.T,
	ctx context.Context,
	httpServer integrationHTTPServer,
) string {
	t.Helper()

	queueResponse, err := receiveOne(ctx, httpServer.queueClient, httpServer.queueURL)
	require.NoError(t, err, "receive authorised onOffFromWork queue message")

	gotAction := queue.DecodeMessage(queueResponse.Messages[0])
	wantAction := schedule.BuildOnOffFromWorkAction("2026-06-11T18:30:00Z")
	assertAction(t, wantAction, gotAction)

	return queueResponse.Messages[0].ReceiptHandle
}

func createProvisionedChatTable(t *testing.T, ctx context.Context, client *awsdynamodb.Client) (string, func()) {
	t.Helper()

	tableName := "telegram-jung2-bot-scale-it-" + formatInt(integrationNow.UnixNano())
	_, err := client.CreateTable(ctx, &awsdynamodb.CreateTableInput{
		TableName: awscore.String(tableName),
		AttributeDefinitions: []ddbtypes.AttributeDefinition{
			{AttributeName: awscore.String("chatId"), AttributeType: ddbtypes.ScalarAttributeTypeN},
		},
		KeySchema: []ddbtypes.KeySchemaElement{
			{AttributeName: awscore.String("chatId"), KeyType: ddbtypes.KeyTypeHash},
		},
		ProvisionedThroughput: &ddbtypes.ProvisionedThroughput{
			ReadCapacityUnits:  awscore.Int64(1),
			WriteCapacityUnits: awscore.Int64(1),
		},
	})
	require.NoError(t, err, "create provisioned chat table")
	require.NoError(t, waitForTableActive(ctx, client, tableName), "wait for provisioned chat table")

	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, deleteErr := client.DeleteTable(cleanupCtx, &awsdynamodb.DeleteTableInput{
			TableName: awscore.String(tableName),
		})
		if deleteErr != nil {
			reportCleanupError("delete provisioned chat table", deleteErr)
		}
	}

	return tableName, cleanup
}
