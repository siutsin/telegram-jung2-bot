package integration

import (
	"context"
	"net/http"
	"testing"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/stretchr/testify/assert"
)

func runHTTPHealthIntegration(
	t *testing.T,
	ctx context.Context,
	sqsClient *awssqs.Client,
	resources testResources,
) {
	t.Helper()

	httpServer := buildIntegrationHTTPServer(t, sqsClient, resources, integrationServerOptions{})
	response := doHTTP(t, ctx, http.MethodGet, httpServer.baseURL+"/health", "")
	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Errorf("close HTTP response body: %v", closeErr)
		}
	}()

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "ok", readResponseBody(t, response))
}
