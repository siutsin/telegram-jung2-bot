package integration

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv(slowTestsEnv) != "1" {
		integrationSlowGate = true
		os.Exit(m.Run())
	}

	err := bootstrapIntegration()
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration bootstrap failed: %v\n", err)
		os.Exit(1)
	}
	defer teardownIntegration()
	fmt.Fprintf(os.Stderr, "Floci integration using %s\n", integrationEndpoint())

	os.Exit(m.Run())
}

func TestFlociDynamoDB(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runDynamoDBIntegration(t, ctx, clients.dynamo, resources)
}

func TestFlociSQS(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runSQSIntegration(t, ctx, clients.sqs, resources)
}

func TestFlociHTTPHealth(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runHTTPHealthIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

func TestFlociHTTPWebhook(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runWebhookIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

func TestFlociHTTPStage(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runStageHTTPIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

func TestFlociWorkerService(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runWorkerServiceIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

func TestFlociServiceOnOffFromWork(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runServiceOnOffFromWorkIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

func TestFlociServiceAdminSettings(t *testing.T) {
	ctx, clients, resources := startIntegrationTest(t)
	runServiceAdminSettingsIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}
