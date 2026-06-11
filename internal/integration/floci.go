package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultImage = "floci/floci:latest"
	flociPort    = "4566/tcp"
)

type flociContainer struct {
	container testcontainers.Container
	endpoint  string
}

func startFloci(ctx context.Context, image string) (flociContainer, error) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{flociPort},
			WaitingFor: wait.ForHTTP("/_floci/init").
				WithPort(flociPort).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return flociContainer{}, fmt.Errorf("start Floci container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		terminateFloci(container)
		return flociContainer{}, fmt.Errorf("resolve Floci container host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, flociPort)
	if err != nil {
		terminateFloci(container)
		return flociContainer{}, fmt.Errorf("resolve Floci container port: %w", err)
	}

	return flociContainer{
		container: container,
		endpoint:  "http://" + host + ":" + mappedPort.Port(),
	}, nil
}

func terminateFloci(container testcontainers.Container) {
	if container == nil {
		return
	}

	terminateCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := container.Terminate(terminateCtx, testcontainers.StopTimeout(20*time.Second))
	if err != nil {
		reportCleanupError("terminate Floci container", err)
	}
}
