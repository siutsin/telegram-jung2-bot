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
	defaultRegion       = "eu-west-1"
	integrationTestsEnv = "INTEGRATION_TESTS"
)

type integrationRuntime struct {
	ctx      context.Context
	clients  awsClients
	endpoint string
	cleanup  func()
}

var (
	sharedRuntime        *integrationRuntime
	integrationTestsGate bool
)

func bootstrapIntegrationRuntime() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	endpoint := os.Getenv("FLOCI_ENDPOINT")
	image := getenvDefault("FLOCI_IMAGE", defaultImage)
	region := getenvDefault("AWS_REGION", defaultRegion)

	var flociCleanup func()
	if endpoint == "" {
		floci, err := startFloci(ctx, image)
		if err != nil {
			cancel()
			return fmt.Errorf("start Floci: %w", err)
		}
		endpoint = floci.endpoint
		flociCleanup = func() {
			terminateFloci(floci.container)
		}
	}

	clients, err := newAWSClients(ctx, endpoint, region)
	if err != nil {
		cancel()
		if flociCleanup != nil {
			flociCleanup()
		}
		return fmt.Errorf("create AWS clients: %w", err)
	}

	sharedRuntime = &integrationRuntime{
		ctx:      ctx,
		clients:  clients,
		endpoint: endpoint,
		cleanup: func() {
			if flociCleanup != nil {
				flociCleanup()
			}
			cancel()
		},
	}

	return nil
}

func teardownIntegrationRuntime() {
	if sharedRuntime != nil && sharedRuntime.cleanup != nil {
		sharedRuntime.cleanup()
		sharedRuntime = nil
	}
}

func startIntegrationTest(t *testing.T) (context.Context, awsClients, testResources) {
	t.Helper()

	fmt.Fprintf(os.Stderr, "=== RUN   %s\n", t.Name())

	ctx, clients := requireIntegrationRuntime(t)

	resources, resourceCleanup, err := provisionResources(ctx, clients)
	require.NoError(t, err, "provision test resources")
	t.Cleanup(resourceCleanup)

	return ctx, clients, resources
}

func requireIntegrationRuntime(t *testing.T) (context.Context, awsClients) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping Floci integration in short mode")
	}
	if integrationTestsGate {
		t.Skipf("set %s=1 to run Floci integration", integrationTestsEnv)
	}
	require.NotNil(t, sharedRuntime, "integration runtime not initialised")

	return sharedRuntime.ctx, sharedRuntime.clients
}

func integrationEndpoint() string {
	if sharedRuntime == nil {
		return ""
	}

	return sharedRuntime.endpoint
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
