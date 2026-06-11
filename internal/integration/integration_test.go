package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	defaultRegion = "eu-west-1"
	slowTestsEnv  = "SLOW_TESTS"
)

func TestFlociAWSAdapters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Floci integration in short mode")
	}
	if os.Getenv(slowTestsEnv) != "1" {
		t.Skipf("set %s=1 to run Floci integration", slowTestsEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	endpoint := os.Getenv("FLOCI_ENDPOINT")
	image := getenvDefault("FLOCI_IMAGE", defaultImage)
	region := getenvDefault("AWS_REGION", defaultRegion)

	if endpoint == "" {
		floci, err := startFloci(ctx, image)
		require.NoError(t, err, "start Floci")
		t.Cleanup(func() {
			terminateFloci(floci.container)
		})
		endpoint = floci.endpoint
	}

	clients, err := newAWSClients(ctx, endpoint, region)
	require.NoError(t, err, "create AWS clients")

	resources, cleanup, err := provisionResources(ctx, clients)
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	require.NoError(t, err, "provision local AWS resources")

	runDynamoDBIntegration(t, ctx, clients.dynamo, resources)
	runSQSIntegration(t, ctx, clients.sqs, resources)
	runWebhookIntegration(t, ctx, clients.dynamo, clients.sqs, resources)

	t.Logf("Floci integration passed using %s", endpoint)
}

func reportCleanupError(action string, err error) {
	fmt.Fprintf(os.Stderr, "cleanup %s: %v\n", action, err)
}

func getenvDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}

	return fallback
}
