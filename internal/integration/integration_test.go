package integration

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv(integrationTestsEnv) != "1" {
		integrationTestsGate = true
		os.Exit(m.Run())
	}

	err := bootstrapIntegrationRuntime()
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration runtime bootstrap failed: %v\n", err)
		os.Exit(1)
	}
	defer teardownIntegrationRuntime()
	fmt.Fprintf(os.Stderr, "Floci integration using %s (container %s)\n", integrationEndpoint(), integrationContainerName())

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
