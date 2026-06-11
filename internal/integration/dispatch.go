package integration

import (
	"context"
	"fmt"

	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/service"
	"github.com/siutsin/telegram-jung2-bot/internal/worker"
)

func dispatchAllServiceActions(ctx context.Context, svc service.Service, action queue.Action) error {
	switch action.Name {
	case queue.ActionJungHelp:
		return svc.JungHelp(ctx, actionChatID(action), action.Attributes["chatTitle"])
	case queue.ActionTopTen:
		return svc.TopTen(ctx, actionChatID(action))
	case queue.ActionTopDiver:
		return svc.TopDiver(ctx, actionChatID(action))
	case queue.ActionAllJung:
		return svc.AllJung(ctx, actionChatID(action))
	case queue.ActionOffFromWork:
		return svc.OffFromWork(ctx, actionChatID(action))
	case queue.ActionEnableAllJung:
		return svc.EnableAllJung(
			ctx,
			actionChatID(action),
			action.Attributes["chatTitle"],
			actionUserID(action),
		)
	case queue.ActionDisableAllJung:
		return svc.DisableAllJung(
			ctx,
			actionChatID(action),
			action.Attributes["chatTitle"],
			actionUserID(action),
		)
	case queue.ActionSetOffWorkTime:
		return svc.SetOffWorkTime(ctx, worker.SetOffInput{
			ChatID:    actionChatID(action),
			ChatTitle: action.Attributes["chatTitle"],
			UserID:    actionUserID(action),
			OffTime:   action.Attributes["offTime"],
			Workday:   action.Attributes["workday"],
		})
	case queue.ActionOnOffFromWork:
		return svc.OnOffFromWork(ctx, action.Attributes["timeString"])
	default:
		return fmt.Errorf("unexpected action %q", action.Name)
	}
}

func actionUserID(action queue.Action) int64 {
	return actionChatIDFromAttribute(action.Attributes["userId"])
}
