package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/workday"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestServiceHandleDueReportRequiresChats(t *testing.T) {
	t.Parallel()

	err := (Service{Enqueuer: &fakeEnqueuer{}}).HandleDueReport(context.Background(), time.Time{})

	require.Error(t, err)
	assert.EqualError(t, err, "chat repository is required")
}

func TestServiceHandleDueReportRequiresEnqueuer(t *testing.T) {
	t.Parallel()

	err := (Service{Chats: &fakeChatRepository{}}).HandleDueReport(context.Background(), time.Time{})

	require.Error(t, err)
	assert.EqualError(t, err, "enqueuer is required")
}

func TestServiceHandleDueReportWrapsListError(t *testing.T) {
	t.Parallel()

	err := (Service{Chats: &fakeChatRepository{err: errors.New("boom")}, Enqueuer: &fakeEnqueuer{}}).HandleDueReport(context.Background(), time.Time{})

	require.Error(t, err)
	assert.EqualError(t, err, "list due chats: boom")
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

func TestDueChatIDsSkipsContractDefaultOnWeekend(t *testing.T) {
	t.Parallel()

	chatIDs := DueChatIDs([]chat.Settings{{ChatID: 1}}, time.Date(2022, 3, 5, 10, 0, 0, 0, time.UTC))

	assert.Empty(t, chatIDs)
}

func TestBuildOnOffFromWorkAction(t *testing.T) {
	t.Parallel()

	action := BuildOnOffFromWorkAction("2022-03-04T10:00:00.000Z")

	assert.Equal(t, queue.ActionOnOffFromWork, action.Name)
	assert.Equal(t, queue.BodyOnOffFromWork, action.Body)
	assert.Equal(t, map[string]string{
		"action":     "onOffFromWork",
		"timeString": "2022-03-04T10:00:00.000Z",
	}, action.Attributes)
}

func TestBuildOffFromWorkAction(t *testing.T) {
	t.Parallel()

	action := BuildOffFromWorkAction(123)

	assert.Equal(t, queue.ActionOffFromWork, action.Name)
	assert.Equal(t, queue.BodyOffFromWork, action.Body)
	assert.Equal(t, map[string]string{
		"action": "offFromWork",
		"chatId": "123",
	}, action.Attributes)
}

func TestEnableAllJung(t *testing.T) {
	t.Parallel()

	change := EnableAllJung("chats", 123, "Group", true)

	require.True(t, change.Allowed)
	assert.Contains(t, change.Reply, "Enabled AllJung command")
	assert.Equal(t, "chats", change.Update.TableName)
	assert.Equal(t, map[string]any{"chatId": int64(123)}, change.Update.Key)
	assert.Equal(t, map[string]any{":eaj": true}, change.Update.ExpressionAttributeValues)
}

func TestEnableAllJungRequiresAdmin(t *testing.T) {
	t.Parallel()

	assert.False(t, EnableAllJung("chats", 123, "Group", false).Allowed)
}

func TestDisableAllJung(t *testing.T) {
	t.Parallel()

	change := DisableAllJung("chats", 123, "Group", true)

	require.True(t, change.Allowed)
	assert.Contains(t, change.Reply, "Disabled AllJung command")
	assert.Equal(t, map[string]any{":eaj": false}, change.Update.ExpressionAttributeValues)
}

func TestDisableAllJungRequiresAdmin(t *testing.T) {
	t.Parallel()

	assert.False(t, DisableAllJung("chats", 123, "Group", false).Allowed)
}

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
