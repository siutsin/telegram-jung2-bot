package httpserver

import (
	"testing"

	gomock "go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"

	mock "github.com/siutsin/telegram-jung2-bot/internal/mock"
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
			name:    "missing message store",
			wantErr: "message store is required",
		},
		{
			name:         "missing chat store",
			dependencies: Dependencies{Messages: mock.NewMockMessageSaver(controller)},
			wantErr:      "chat store is required",
		},
		{
			name:         "missing enqueuer",
			dependencies: Dependencies{Messages: mock.NewMockMessageSaver(controller), Chats: mock.NewMockChatSaver(controller)},
			wantErr:      "enqueuer is required",
		},
		{
			name:         "missing messenger",
			dependencies: Dependencies{Messages: mock.NewMockMessageSaver(controller), Chats: mock.NewMockChatSaver(controller), Enqueuer: mock.NewMockEnqueuer(controller)},
			wantErr:      "messenger is required",
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
