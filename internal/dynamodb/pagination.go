package dynamodb

import (
	"context"
	"fmt"
)

// page is the typed result from one paginated DynamoDB fetch.
type page[T any] struct {
	Items            []T
	LastEvaluatedKey map[string]any
}

// collectPages accumulates paginated DynamoDB results.
// For example, two pages [1,2] and [3] become one slice [1,2,3].
// It stays generic because the package uses the same pagination flow for
// messages, chat settings, and chat IDs.
func collectPages[T any](
	ctx context.Context,
	fetch func(ctx context.Context, exclusiveStartKey map[string]any) (page[T], error),
) ([]T, error) {
	var rows []T
	var startKey map[string]any

	for {
		err := ctx.Err()
		if err != nil {
			return nil, fmt.Errorf("collect dynamodb pages: %w", err)
		}

		fetchedPage, err := fetch(ctx, startKey)
		if err != nil {
			return nil, fmt.Errorf("collect dynamodb pages: %w", err)
		}
		rows = append(rows, fetchedPage.Items...)
		if len(fetchedPage.LastEvaluatedKey) == 0 {
			return rows, nil
		}
		startKey = fetchedPage.LastEvaluatedKey
	}
}
