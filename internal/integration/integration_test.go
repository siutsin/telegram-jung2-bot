package integration

import (
	"fmt"
	"os"
	"testing"
)

// TestMain shares one Floci runtime so integration tests avoid repeated container startup.
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
	fmt.Fprintf(os.Stderr, "Floci integration using %s (container %s)\n", integrationEndpoint(), integrationContainerName())

	code := m.Run()
	teardownIntegrationRuntime()
	os.Exit(code)
}

// TestFlociDynamoDB protects real DynamoDB CRUD behaviour from emulator and SDK drift.
func TestFlociDynamoDB(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runDynamoDBIntegration(t, ctx, clients.dynamo, resources)
}

// TestFlociDynamoDBDueChatPagination keeps due-chat scans correct across result pages.
func TestFlociDynamoDBDueChatPagination(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runDynamoDBDueChatPaginationIntegration(t, ctx, clients.dynamo, resources)
}

// TestFlociDynamoDBMessagePagination keeps message queries correct across result pages.
func TestFlociDynamoDBMessagePagination(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runDynamoDBMessageQueryPaginationIntegration(t, ctx, clients.dynamo, resources)
}

// TestFlociSQS protects supported queue action shapes and attribute casing.
func TestFlociSQS(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runSQSIntegration(t, ctx, clients.sqs, resources)
}

// TestFlociSQSBatch ensures one worker poll handles every message in a batch.
func TestFlociSQSBatch(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runSQSBatchIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociHTTPHealth keeps the health contract available through the production handler.
func TestFlociHTTPHealth(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runHTTPHealthIntegration(t, ctx, clients.sqs, resources)
}

// TestFlociHTTPWebhook protects webhook routing and persistence against integration regressions.
func TestFlociHTTPWebhook(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runWebhookIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociHTTPWebhookTelegramClient keeps webhook replies compatible with the Telegram client.
func TestFlociHTTPWebhookTelegramClient(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runWebhookTelegramClientIntegration(t, ctx, clients.sqs, resources)
}

// TestFlociHTTPStage protects stage-specific routing and scheduler authentication.
func TestFlociHTTPStage(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runStageHTTPIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociAppRun verifies production components cooperate through the app lifecycle.
func TestFlociAppRun(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runAppRunIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociWorkerRun keeps the production polling loop working with real SQS traffic.
func TestFlociWorkerRun(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runWorkerRunIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociWorkerHandlers protects queue-to-service dispatch for administrative actions.
func TestFlociWorkerHandlers(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runWorkerHandlersIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociWorkerService verifies report actions reach the service through one poll loop.
func TestFlociWorkerService(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runWorkerServiceIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociServiceOnOffFromWork protects scheduled off-from-work fan-out behaviour.
func TestFlociServiceOnOffFromWork(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runServiceOnOffFromWorkIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}

// TestFlociServiceAdminSettings preserves administrative setting side effects.
func TestFlociServiceAdminSettings(t *testing.T) {
	t.Parallel()

	ctx, clients, resources := startIntegrationTest(t)
	runServiceAdminSettingsIntegration(t, ctx, clients.dynamo, clients.sqs, resources)
}
