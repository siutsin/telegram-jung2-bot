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
	defaultRegion                   = "eu-west-1"
	integrationTestsEnv             = "INTEGRATION_TESTS"
	maxParallelIntegrationResources = 3
)

type integrationRuntime struct {
	ctx           context.Context
	clients       awsClients
	endpoint      string
	containerName string
	cleanup       func()
}

var (
	sharedRuntime            *integrationRuntime
	integrationTestsGate     bool
	integrationResourceSlots = make(chan struct{}, maxParallelIntegrationResources)
)

// bootstrapIntegrationRuntime creates the shared emulator clients before parallel tests start.
func bootstrapIntegrationRuntime() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	endpoint := os.Getenv("FLOCI_ENDPOINT")
	image := getenvDefault("FLOCI_IMAGE", defaultImage)
	containerName := getenvDefault("FLOCI_CONTAINER_NAME", defaultFlociContainerName)
	region := getenvDefault("AWS_REGION", defaultRegion)

	var flociCleanup func()
	if endpoint == "" {
		floci, err := startFloci(ctx, image, containerName)
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
		ctx:           ctx,
		clients:       clients,
		endpoint:      endpoint,
		containerName: containerName,
		cleanup: func() {
			if flociCleanup != nil {
				flociCleanup()
			}
			cancel()
		},
	}

	return nil
}

// teardownIntegrationRuntime releases the shared runtime after all tests have stopped using it.
func teardownIntegrationRuntime() {
	if sharedRuntime != nil && sharedRuntime.cleanup != nil {
		sharedRuntime.cleanup()
		sharedRuntime = nil
	}
}

// startIntegrationTest bounds emulator load while preserving isolated AWS resources per test.
func startIntegrationTest(t *testing.T) (context.Context, awsClients, testResources) {
	t.Helper()

	fmt.Fprintf(os.Stderr, "=== RUN   %s\n", t.Name())

	ctx, clients := requireIntegrationRuntime(t)
	acquireIntegrationResourceSlot(t)

	resources, resourceCleanup, err := provisionResources(ctx, clients)
	require.NoError(t, err, "provision test resources")
	t.Cleanup(resourceCleanup)

	return ctx, clients, resources
}

// acquireIntegrationResourceSlot prevents parallel tests from exhausting the Floci container.
func acquireIntegrationResourceSlot(t *testing.T) {
	t.Helper()

	select {
	case integrationResourceSlots <- struct{}{}:
		t.Cleanup(func() { <-integrationResourceSlots })
	case <-t.Context().Done():
		t.Fatalf("acquire integration resource slot: %v", t.Context().Err())
	}
}

// requireIntegrationRuntime skips opt-in tests unless their shared Floci runtime is ready.
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

// integrationEndpoint reports the shared endpoint for diagnostics without exposing runtime state.
func integrationEndpoint() string {
	if sharedRuntime == nil {
		return ""
	}

	return sharedRuntime.endpoint
}

// integrationContainerName reports the shared container for failure diagnostics.
func integrationContainerName() string {
	if sharedRuntime == nil {
		return ""
	}

	return sharedRuntime.containerName
}

// reportCleanupError preserves the primary test result while surfacing cleanup failures.
func reportCleanupError(action string, err error) {
	fmt.Fprintf(os.Stderr, "cleanup %s: %v\n", action, err)
}

// getenvDefault keeps local integration overrides optional and repeatable.
func getenvDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}

	return fallback
}
