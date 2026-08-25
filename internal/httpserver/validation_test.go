package httpserver

import (
	"testing"

	gomock "go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"

	httpservermock "github.com/siutsin/telegram-jung2-bot/internal/mock/httpserver"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	_, validDependencies := newMockDependencies(t)
	controller := gomock.NewController(t)
	tests := []struct {
		name         string
		dependencies Dependencies
		wantErr      string
	}{
		{
			name:         "valid",
			dependencies: validDependencies,
		},
		{
			name:         "missing enqueuer",
			dependencies: Dependencies{},
			wantErr:      "enqueuer is required",
		},
		{
			name:         "missing messenger",
			dependencies: Dependencies{Enqueuer: httpservermock.NewMockEnqueuer(controller), MessageEnqueuer: httpservermock.NewMockEnqueuer(controller)},
			wantErr:      "messenger is required",
		},
		{
			name:         "missing message enqueuer",
			dependencies: Dependencies{Enqueuer: httpservermock.NewMockEnqueuer(controller)},
			wantErr:      "message enqueuer is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validate(tc.dependencies)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}
