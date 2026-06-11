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

type integrationEnv struct {
	ctx       context.Context
	clients   awsClients
	resources testResources
	endpoint  string
	cleanup   func()
}

var (
	sharedIntegrationEnv *integrationEnv
	integrationSlowGate  bool
)

func bootstrapIntegration() error {
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

	resources, resourceCleanup, err := provisionResources(ctx, clients)
	if err != nil {
		cancel()
		if flociCleanup != nil {
			flociCleanup()
		}
		return fmt.Errorf("provision local AWS resources: %w", err)
	}

	sharedIntegrationEnv = &integrationEnv{
		ctx:       ctx,
		clients:   clients,
		resources: resources,
		endpoint:  endpoint,
		cleanup: func() {
			if resourceCleanup != nil {
				resourceCleanup()
			}
			if flociCleanup != nil {
				flociCleanup()
			}
			cancel()
		},
	}

	return nil
}

func teardownIntegration() {
	if sharedIntegrationEnv != nil && sharedIntegrationEnv.cleanup != nil {
		sharedIntegrationEnv.cleanup()
		sharedIntegrationEnv = nil
	}
}

func startIntegrationTest(t *testing.T) (context.Context, awsClients, testResources) {
	t.Helper()

	fmt.Fprintf(os.Stderr, "=== RUN   %s\n", t.Name())

	return requireIntegrationEnv(t)
}

func requireIntegrationEnv(t *testing.T) (context.Context, awsClients, testResources) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping Floci integration in short mode")
	}
	if integrationSlowGate {
		t.Skipf("set %s=1 to run Floci integration", slowTestsEnv)
	}
	require.NotNil(t, sharedIntegrationEnv, "integration environment not initialised")

	return sharedIntegrationEnv.ctx, sharedIntegrationEnv.clients, sharedIntegrationEnv.resources
}

func integrationEndpoint() string {
	if sharedIntegrationEnv == nil {
		return ""
	}

	return sharedIntegrationEnv.endpoint
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
