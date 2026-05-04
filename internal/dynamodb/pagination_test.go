package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectPages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		run     func(t *testing.T) ([]string, error)
		want    []string
		wantErr error
		errText []string
	}{
		{
			name: "multiple pages",
			run: func(t *testing.T) ([]string, error) {
				pageIndex := 0
				return collectPages(context.Background(), func(ctx context.Context, startKey map[string]any) (page[string], error) {
					require.NoError(t, ctx.Err())
					pageIndex++
					switch pageIndex {
					case 1:
						assert.Nil(t, startKey)
						return page[string]{
							Items:            []string{"first"},
							LastEvaluatedKey: map[string]any{"chatId": int64(1)},
						}, nil
					case 2:
						assert.Equal(t, map[string]any{"chatId": int64(1)}, startKey)
						return page[string]{Items: []string{"second"}}, nil
					default:
						t.Fatalf("unexpected page %d", pageIndex)
						return page[string]{}, nil
					}
				})
			},
			want: []string{"first", "second"},
		},
		{
			name: "empty result",
			run: func(t *testing.T) ([]string, error) {
				return collectPages(context.Background(), func(ctx context.Context, startKey map[string]any) (page[string], error) {
					assert.Nil(t, startKey)
					return page[string]{}, nil
				})
			},
			want: nil,
		},
		{
			name: "fetch error",
			run: func(t *testing.T) ([]string, error) {
				return collectPages(context.Background(), func(ctx context.Context, startKey map[string]any) (page[string], error) {
					return page[string]{}, errors.New("boom")
				})
			},
			errText: []string{"collect dynamodb pages", "boom"},
		},
		{
			name: "context error",
			run: func(t *testing.T) ([]string, error) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return collectPages(ctx, func(ctx context.Context, startKey map[string]any) (page[string], error) {
					t.Fatal("fetch should not run after context cancellation")
					return page[string]{}, nil
				})
			},
			wantErr: context.Canceled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, err := test.run(t)
			if test.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, test.wantErr)
				return
			}
			if len(test.errText) > 0 {
				require.Error(t, err)
				for _, text := range test.errText {
					assert.Contains(t, err.Error(), text)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, rows)
		})
	}
}
