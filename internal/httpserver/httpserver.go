// Package httpserver owns transport-independent webhook handling.
package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/siutsin/telegram-jung2-bot/internal/chat"
	"github.com/siutsin/telegram-jung2-bot/internal/command"
	"github.com/siutsin/telegram-jung2-bot/internal/message"
	"github.com/siutsin/telegram-jung2-bot/internal/queue"
	"github.com/siutsin/telegram-jung2-bot/internal/schedule"
	"github.com/siutsin/telegram-jung2-bot/internal/telegram"
)

type Response struct {
	StatusCode int
	Message    string
}

type Store interface {
	SaveMessage(ctx context.Context, request message.UpdateExpression) error
	SaveChat(ctx context.Context, request chat.UpdateExpression) error
}

type Enqueuer interface {
	Enqueue(ctx context.Context, action queue.Action) error
}

type Messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type ScaleUpper interface {
	ScaleUp(ctx context.Context) error
}

type Dependencies struct {
	MessageTable string
	ChatTable    string
	Store        Store
	Enqueuer     Enqueuer
	Messenger    Messenger
	ScaleUpper   ScaleUpper
	Now          func() time.Time
}

type ServerDeps struct {
	Dependencies
	MaxBodyBytes int64
	Stage        string
}

// New builds the HTTP handler for service routes.
func New(dependencies ServerDeps) http.Handler {
	mux := http.NewServeMux()
	healthHandler := func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeResponse(writer, Health())
	}
	webhookHandler := func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxBodyBytes(dependencies)))
		if err != nil {
			writeResponse(writer, Response{StatusCode: http.StatusBadRequest, Message: "read request body"})
			return
		}
		writeResponse(writer, HandleWebhook(request.Context(), body, dependencies.Dependencies))
	}
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/webhook", webhookHandler)
	if dependencies.Stage != "" {
		stagePrefix := "/jung2bot/" + strings.Trim(dependencies.Stage, "/")
		mux.HandleFunc(stagePrefix+"/ping", func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			writeJSONResponse(writer, http.StatusOK, map[string]string{"health": "ok"})
		})
		mux.HandleFunc(stagePrefix, func(writer http.ResponseWriter, request *http.Request) {
			http.NotFound(writer, request)
		})
		mux.HandleFunc(stagePrefix+"/", func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != stagePrefix+"/" {
				http.NotFound(writer, request)
				return
			}
			if request.Method != http.MethodPost {
				http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxBodyBytes(dependencies)))
			if err != nil {
				writeJSONResponse(writer, http.StatusBadRequest, map[string]any{"statusCode": http.StatusBadRequest, "message": "read request body"})
				return
			}
			response := HandleWebhook(request.Context(), body, dependencies.Dependencies)
			writeStageWebhookResponse(writer, response)
		})
		mux.HandleFunc(stagePrefix+"/onOffFromWork", func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := dependencies.Enqueuer.Enqueue(request.Context(), schedule.BuildOnOffFromWorkAction(request.URL.Query().Get("timeString"))); err != nil {
				http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			writeJSONResponse(writer, http.StatusAccepted, map[string]string{"onOffFromWork": "ok"})
		})
		mux.HandleFunc(stagePrefix+"/onScaleUp", func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if dependencies.ScaleUpper == nil {
				writeJSONResponse(writer, http.StatusServiceUnavailable, map[string]string{"onScaleUp": "failed"})
				return
			}
			if err := dependencies.ScaleUpper.ScaleUp(request.Context()); err != nil {
				writeJSONResponse(writer, http.StatusServiceUnavailable, map[string]string{"onScaleUp": "failed"})
				return
			}
			writeJSONResponse(writer, http.StatusOK, map[string]string{"onScaleUp": "ok"})
		})
	}

	return mux
}

// Health returns the health check response.
func Health() Response {
	return Response{StatusCode: 200, Message: "ok"}
}

// HandleWebhook processes a Telegram webhook payload.
func HandleWebhook(ctx context.Context, payload []byte, dependencies Dependencies) Response {
	update, err := telegram.ParseUpdate(payload)
	if err != nil {
		return Response{StatusCode: 500, Message: "decode Telegram update"}
	}
	if update.Message == nil || !strings.Contains(update.Message.Chat.Type, "group") {
		return Response{StatusCode: 204, Message: "edited_message or non-group"}
	}

	now := currentTime(dependencies)
	storedMessage := message.FromTelegram(*update.Message, now)
	if err := dependencies.Store.SaveMessage(ctx, message.BuildSaveUpdate(dependencies.MessageTable, storedMessage)); err != nil {
		return Response{StatusCode: 500, Message: "save message"}
	}
	storedChat := chat.FromTelegram(*update.Message, now)
	if err := dependencies.Store.SaveChat(ctx, chat.BuildMetadataUpdate(dependencies.ChatTable, storedChat)); err != nil {
		return Response{StatusCode: 500, Message: "save chat"}
	}

	parsedCommands := parseCommands(*update.Message)
	if len(parsedCommands) == 0 {
		return Response{StatusCode: 200}
	}

	for _, parsedCommand := range parsedCommands {
		action, err := command.ActionFor(parsedCommand, command.ChatContext{
			ChatID:    update.Message.Chat.ID,
			ChatTitle: update.Message.Chat.Title,
			UserID:    userID(update.Message.From),
		})
		if err != nil {
			if parsedCommand.Name == command.SetOffFromWorkTimeUTC {
				if dependencies.Messenger == nil {
					return Response{StatusCode: 500, Message: "reply invalid command"}
				}
				if sendErr := dependencies.Messenger.SendMessage(ctx, update.Message.Chat.ID, schedule.InvalidSetOffFromWorkTimeUTCMessage(update.Message.Chat.Title)); sendErr != nil {
					return Response{StatusCode: 500, Message: "reply invalid command"}
				}
			}
			continue
		}
		if err := dependencies.Enqueuer.Enqueue(ctx, action); err != nil {
			return Response{StatusCode: 500, Message: "enqueue command"}
		}
	}

	return Response{StatusCode: 200}
}

// currentTime returns the injected time or time.Now.
func currentTime(dependencies Dependencies) time.Time {
	if dependencies.Now == nil {
		return time.Now()
	}

	return dependencies.Now()
}

// userID returns the Telegram user ID or zero.
func userID(user *telegram.User) int64 {
	if user == nil {
		return 0
	}

	return user.ID
}

// parseCommands extracts supported bot commands from a message.
func parseCommands(message telegram.Message) []command.Command {
	if len(message.Entities) == 0 || message.Entities[0].Type != "bot_command" {
		return nil
	}

	return command.ParseAll(message.Text)
}

// Validate checks required HTTP dependencies.
func Validate(dependencies Dependencies) error {
	if dependencies.MessageTable == "" {
		return fmt.Errorf("message table is required")
	}
	if dependencies.ChatTable == "" {
		return fmt.Errorf("chat table is required")
	}
	if dependencies.Store == nil {
		return fmt.Errorf("store is required")
	}
	if dependencies.Enqueuer == nil {
		return fmt.Errorf("enqueuer is required")
	}
	if dependencies.Messenger == nil {
		return fmt.Errorf("messenger is required")
	}

	return nil
}

// maxBodyBytes returns the configured body size limit.
func maxBodyBytes(dependencies ServerDeps) int64 {
	if dependencies.MaxBodyBytes > 0 {
		return dependencies.MaxBodyBytes
	}

	return 1 << 20
}

// writeResponse writes a plain response body.
func writeResponse(writer http.ResponseWriter, response Response) {
	writer.WriteHeader(response.StatusCode)
	if response.Message != "" {
		_, _ = writer.Write([]byte(response.Message))
	}
}

// writeStageWebhookResponse writes the stage-compatible webhook response.
func writeStageWebhookResponse(writer http.ResponseWriter, response Response) {
	body := map[string]any{"statusCode": response.StatusCode}
	if response.Message != "" && response.StatusCode < http.StatusInternalServerError {
		body["message"] = response.Message
	}
	writeJSONResponse(writer, response.StatusCode, body)
}

// writeJSONResponse writes a JSON response body.
func writeJSONResponse(writer http.ResponseWriter, statusCode int, body any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(body)
}
