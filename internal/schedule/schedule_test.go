package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
)

func TestServiceSyncChat(t *testing.T) {
	t.Parallel()

	scheduler := &fakeScheduler{}
	settings := chat.Settings{ChatID: 123}

	err := (Service{Scheduler: scheduler}).SyncChat(context.Background(), settings)

	require.NoError(t, err)
	assert.Equal(t, []chat.Settings{settings}, scheduler.synced)
}

func TestServiceSyncChatRequiresScheduler(t *testing.T) {
	t.Parallel()

	err := (Service{}).SyncChat(context.Background(), chat.Settings{})

	require.Error(t, err)
	assert.EqualError(t, err, "scheduler is required")
}

func TestServiceSyncChatWrapsSchedulerError(t *testing.T) {
	t.Parallel()

	err := (Service{Scheduler: &fakeScheduler{err: errors.New("boom")}}).SyncChat(context.Background(), chat.Settings{})

	require.Error(t, err)
	assert.EqualError(t, err, "sync chat schedule: boom")
}

func TestServiceHandleDueReportEnqueuesDueChats(t *testing.T) {
	t.Parallel()

	enqueuer := &fakeEnqueuer{}
	rows := []chat.Settings{
		{ChatID: 1},
		{ChatID: 2, OffTime: "1000", HasOffTime: true, Workday: workday.Workdays(workday.Fri), HasWorkday: true},
		{ChatID: 3, OffTime: "1800", HasOffTime: true, Workday: workday.Workdays(workday.Fri), HasWorkday: true},
	}

	err := (Service{Chats: &fakeChatRepository{rows: rows}, Enqueuer: enqueuer}).HandleDueReport(context.Background(), time.Date(2022, 3, 4, 10, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	assert.Equal(t, []queue.Action{BuildOffFromWorkAction(1), BuildOffFromWorkAction(2)}, enqueuer.actions)
}

// These cases keep due-report failures pinned to the right validation or
// repository boundary instead of silently skipping scheduled reports.
func TestServiceHandleDueReportSetupErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service Service
		wantErr string
	}{
		{name: "missing chats", service: Service{Enqueuer: &fakeEnqueuer{}}, wantErr: "chat repository is required"},
		{name: "missing enqueuer", service: Service{Chats: &fakeChatRepository{}}, wantErr: "enqueuer is required"},
		{name: "list error", service: Service{Chats: &fakeChatRepository{err: errors.New("boom")}, Enqueuer: &fakeEnqueuer{}}, wantErr: "list due chats: boom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.service.HandleDueReport(context.Background(), time.Time{})

			require.Error(t, err)
			assert.EqualError(t, err, test.wantErr)
		})
	}
}

func TestServiceHandleDueReportWrapsEnqueueError(t *testing.T) {
	t.Parallel()

	err := (Service{
		Chats:    &fakeChatRepository{rows: []chat.Settings{{ChatID: 1}}},
		Enqueuer: &fakeEnqueuer{err: errors.New("boom")},
	}).HandleDueReport(context.Background(), time.Date(2022, 3, 4, 10, 0, 0, 0, time.UTC))

	require.Error(t, err)
	assert.EqualError(t, err, "enqueue due off-work report: boom")
}

func TestWindowFromTime(t *testing.T) {
	t.Parallel()

	window := WindowFromTime(time.Date(2022, 3, 4, 10, 0, 0, 0, time.UTC))

	assert.Equal(t, Window{OffTime: "1000", Weekday: "FRI"}, window)
}

func TestWindowFromTimePreservesInputOffset(t *testing.T) {
	t.Parallel()

	window := WindowFromTime(time.Date(2022, 3, 4, 18, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)))

	assert.Equal(t, Window{OffTime: "1800", Weekday: "FRI"}, window)
}

func TestDueChatIDs(t *testing.T) {
	t.Parallel()

	rows := []chat.Settings{
		{ChatID: 1},
		{ChatID: 2, OffTime: "1000", HasOffTime: true, Workday: workday.Workdays(workday.Fri), HasWorkday: true},
		{ChatID: 3, OffTime: "1000", HasOffTime: true, Workday: workday.Workdays(workday.Mon), HasWorkday: true},
		{ChatID: 4, OffTime: "1800", HasOffTime: true, Workday: workday.Workdays(workday.Fri), HasWorkday: true},
	}

	chatIDs := DueChatIDs(rows, time.Date(2022, 3, 4, 10, 0, 0, 0, time.UTC))

	assert.Equal(t, []int64{1, 2}, chatIDs)
}

// Keep both action builders together because they pin the exact queue contract
// shape the runtime depends on.
func TestBuildActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		build     func() queue.Action
		wantName  string
		wantBody  string
		wantAttrs map[string]string
	}{
		{
			name:     "on off from work",
			build:    func() queue.Action { return BuildOnOffFromWorkAction("2022-03-04T10:00:00.000Z") },
			wantName: queue.ActionOnOffFromWork,
			wantBody: queue.BodyOnOffFromWork,
			wantAttrs: map[string]string{
				"action":     "onOffFromWork",
				"timeString": "2022-03-04T10:00:00.000Z",
			},
		},
		{
			name:     "off from work",
			build:    func() queue.Action { return BuildOffFromWorkAction(123) },
			wantName: queue.ActionOffFromWork,
			wantBody: queue.BodyOffFromWork,
			wantAttrs: map[string]string{
				"action": "offFromWork",
				"chatId": "123",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := test.build()

			assert.Equal(t, test.wantName, action.Name)
			assert.Equal(t, test.wantBody, action.Body)
			assert.Equal(t, test.wantAttrs, action.Attributes)
		})
	}
}

// Admin-only toggles share the same contract shape except for reply text and
// boolean payload, so one table keeps the assertions aligned.
func TestAllJungSettingChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		build     func(isAdmin bool) SettingChange
		wantReply string
		wantAttrs map[string]any
	}{
		{
			name:      "enable",
			build:     func(isAdmin bool) SettingChange { return EnableAllJung("chats", 123, "Group", isAdmin) },
			wantReply: "Enabled AllJung command",
			wantAttrs: map[string]any{":eaj": true},
		},
		{
			name:      "disable",
			build:     func(isAdmin bool) SettingChange { return DisableAllJung("chats", 123, "Group", isAdmin) },
			wantReply: "Disabled AllJung command",
			wantAttrs: map[string]any{":eaj": false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			change := test.build(true)

			require.True(t, change.Allowed)
			assert.Contains(t, change.Reply, test.wantReply)
			assert.Equal(t, "chats", change.Update.TableName)
			assert.Equal(t, map[string]any{"chatId": int64(123)}, change.Update.Key)
			assert.Equal(t, test.wantAttrs, change.Update.ExpressionAttributeValues)
			assert.False(t, test.build(false).Allowed)
		})
	}
}

// This test protects the contract that default rows implicitly fire only on
// weekdays, which is easy to break during schedule refactors.
func TestDueChatIDsSkipsContractDefaultOnWeekend(t *testing.T) {
	t.Parallel()

	chatIDs := DueChatIDs([]chat.Settings{{ChatID: 1}}, time.Date(2022, 3, 5, 10, 0, 0, 0, time.UTC))

	assert.Empty(t, chatIDs)
}

// This keeps the admin update payload stable because callers depend on the
// exact workday bitmask and update-expression shape.
func TestSetOffFromWorkTimeUTC(t *testing.T) {
	t.Parallel()

	change, err := SetOffFromWorkTimeUTC("chats", 123, "Group", true, "1800", "MON,TUE")

	require.NoError(t, err)
	require.True(t, change.Allowed)
	assert.Contains(t, change.Reply, "Updated setOffFromWorkTime in UTC: 1800 MON,TUE")
	assert.Equal(t, "SET #ot = :ot, #wd = :wd", change.Update.UpdateExpression)
	assert.Equal(t, map[string]any{
		":ot": "1800",
		":wd": int(workday.Mon | workday.Tue),
	}, change.Update.ExpressionAttributeValues)
}

func TestSetOffFromWorkTimeUTCRequiresAdmin(t *testing.T) {
	t.Parallel()

	change, err := SetOffFromWorkTimeUTC("chats", 123, "Group", false, "1800", "MON")

	require.NoError(t, err)
	assert.False(t, change.Allowed)
}

func TestSetOffFromWorkTimeUTCRejectsBadWorkday(t *testing.T) {
	t.Parallel()

	_, err := SetOffFromWorkTimeUTC("chats", 123, "Group", true, "1800", "NOPE")

	require.Error(t, err)
}

func TestInvalidSetOffFromWorkTimeUTCMessage(t *testing.T) {
	t.Parallel()

	message := InvalidSetOffFromWorkTimeUTCMessage("Group")

	assert.Contains(t, message, "圍爐區: Group")
	assert.Contains(t, message, "Error: Invalid format for setOffFromWorkTimeUTC")
	assert.Contains(t, message, "/setOffFromWorkTimeUTC 1800 MON,TUE,WED,THU,FRI")
}

func TestWeekdayToken(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "SUN", weekdayToken(time.Sunday))
	assert.Equal(t, "MON", weekdayToken(time.Monday))
	assert.Equal(t, "TUE", weekdayToken(time.Tuesday))
	assert.Equal(t, "WED", weekdayToken(time.Wednesday))
	assert.Equal(t, "THU", weekdayToken(time.Thursday))
	assert.Equal(t, "FRI", weekdayToken(time.Friday))
	assert.Equal(t, "SAT", weekdayToken(time.Saturday))
	assert.Empty(t, weekdayToken(time.Weekday(99)))
}

type fakeScheduler struct {
	synced []chat.Settings
	err    error
}

func (scheduler *fakeScheduler) Sync(ctx context.Context, settings chat.Settings) error {
	scheduler.synced = append(scheduler.synced, settings)
	return scheduler.err
}

type fakeChatRepository struct {
	rows []chat.Settings
	err  error
}

func (repository *fakeChatRepository) ListEnabled(ctx context.Context) ([]chat.Settings, error) {
	return repository.rows, repository.err
}

type fakeEnqueuer struct {
	actions []queue.Action
	err     error
}

func (enqueuer *fakeEnqueuer) Enqueue(ctx context.Context, action queue.Action) error {
	enqueuer.actions = append(enqueuer.actions, action)
	return enqueuer.err
}
