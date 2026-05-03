// Package httpserver owns transport-independent webhook handling.
package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

type MessageStore interface {
	Save(ctx context.Context, message message.Message) error
}

type ChatStore interface {
	Save(ctx context.Context, settings chat.Settings) error
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
	Messages   MessageStore
	Chats      ChatStore
	Enqueuer   Enqueuer
	Messenger  Messenger
	ScaleUpper ScaleUpper
	Now        func() time.Time
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
		registerStageRoutes(mux, dependencies)
	}

	return mux
}

// registerStageRoutes wires the contract-compatible stage-prefixed routes.
func registerStageRoutes(mux *http.ServeMux, dependencies ServerDeps) {
	stagePrefix := "/jung2bot/" + strings.Trim(dependencies.Stage, "/")
	registerStagePingRoute(mux, stagePrefix)
	registerStagePrefixRoute(mux, stagePrefix)
	registerStageWebhookRoute(mux, stagePrefix, dependencies)
	registerOnOffFromWorkRoute(mux, stagePrefix, dependencies)
	registerScaleUpRoute(mux, stagePrefix, dependencies)
}

// Health returns the health check response.
func Health() Response {
	return Response{StatusCode: 200, Message: "ok"}
}

// HandleWebhook processes a Telegram webhook payload.
func HandleWebhook(ctx context.Context, payload []byte, dependencies Dependencies) Response {
	telegramMessage, response, ok := parseGroupMessage(payload)
	if !ok {
		return response
	}
	if response, ok := saveWebhookState(ctx, *telegramMessage, currentTime(dependencies), dependencies); !ok {
		return response
	}

	return enqueueWebhookCommands(ctx, *telegramMessage, dependencies)
}

// registerStagePingRoute wires the stage-compatible health route.
func registerStagePingRoute(mux *http.ServeMux, stagePrefix string) {
	mux.HandleFunc(stagePrefix+"/ping", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSONResponse(writer, http.StatusOK, map[string]string{"health": "ok"})
	})
}

// registerStagePrefixRoute keeps the stage root without trailing slash as 404.
func registerStagePrefixRoute(mux *http.ServeMux, stagePrefix string) {
	mux.HandleFunc(stagePrefix, func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	})
}

// registerStageWebhookRoute wires the stage-compatible webhook route.
func registerStageWebhookRoute(mux *http.ServeMux, stagePrefix string, dependencies ServerDeps) {
	mux.HandleFunc(stagePrefix+"/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != stagePrefix+"/" {
			http.NotFound(writer, request)
			return
		}
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := readRequestBody(writer, request, maxBodyBytes(dependencies))
		if err != nil {
			writeJSONResponse(writer, http.StatusBadRequest, map[string]any{"statusCode": http.StatusBadRequest, "message": "read request body"})
			return
		}
		writeStageWebhookResponse(writer, HandleWebhook(request.Context(), body, dependencies.Dependencies))
	})
}

// registerOnOffFromWorkRoute wires the stage-compatible off-work trigger route.
func registerOnOffFromWorkRoute(mux *http.ServeMux, stagePrefix string, dependencies ServerDeps) {
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
}

// registerScaleUpRoute wires the stage-compatible scale-up route.
func registerScaleUpRoute(mux *http.ServeMux, stagePrefix string, dependencies ServerDeps) {
	mux.HandleFunc(stagePrefix+"/onScaleUp", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := scaleUp(request.Context(), dependencies.ScaleUpper); err != nil {
			writeJSONResponse(writer, http.StatusServiceUnavailable, map[string]string{"onScaleUp": "failed"})
			return
		}
		writeJSONResponse(writer, http.StatusOK, map[string]string{"onScaleUp": "ok"})
	})
}

// readRequestBody reads a bounded request body.
func readRequestBody(writer http.ResponseWriter, request *http.Request, bodyLimit int64) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(writer, request.Body, bodyLimit))
}

// scaleUp triggers the optional scale-up dependency.
func scaleUp(ctx context.Context, scaleUpper ScaleUpper) error {
	if scaleUpper == nil {
		return fmt.Errorf("scale upper is required")
	}

	return scaleUpper.ScaleUp(ctx)
}

// parseGroupMessage parses a Telegram webhook and keeps only group messages.
func parseGroupMessage(payload []byte) (*telegram.Message, Response, bool) {
	update, err := telegram.ParseUpdate(payload)
	if err != nil {
		return nil, Response{StatusCode: 500, Message: "decode Telegram update"}, false
	}
	if update.Message == nil || !strings.Contains(update.Message.Chat.Type, "group") {
		return nil, Response{StatusCode: 204, Message: "edited_message or non-group"}, false
	}

	return update.Message, Response{}, true
}

// saveWebhookState persists the message and chat records for a webhook update.
func saveWebhookState(ctx context.Context, telegramMessage telegram.Message, now time.Time, dependencies Dependencies) (Response, bool) {
	if err := saveWebhookMessage(ctx, telegramMessage, now, dependencies); err != nil {
		return Response{StatusCode: 500, Message: "save message"}, false
	}
	if err := saveWebhookChat(ctx, telegramMessage, now, dependencies); err != nil {
		return Response{StatusCode: 500, Message: "save chat"}, false
	}

	return Response{}, true
}

// saveWebhookMessage persists a Telegram message row.
func saveWebhookMessage(ctx context.Context, telegramMessage telegram.Message, now time.Time, dependencies Dependencies) error {
	storedMessage := message.FromTelegram(telegramMessage, now)
	return dependencies.Messages.Save(ctx, storedMessage)
}

// saveWebhookChat persists Telegram chat metadata.
func saveWebhookChat(ctx context.Context, telegramMessage telegram.Message, now time.Time, dependencies Dependencies) error {
	storedChat := chat.FromTelegram(telegramMessage, now)
	return dependencies.Chats.Save(ctx, storedChat)
}

// enqueueWebhookCommands converts and enqueues supported Telegram commands.
func enqueueWebhookCommands(ctx context.Context, telegramMessage telegram.Message, dependencies Dependencies) Response {
	for _, parsedCommand := range parseCommands(telegramMessage) {
		response, ok := enqueueWebhookCommand(ctx, telegramMessage, parsedCommand, dependencies)
		if !ok {
			return response
		}
	}

	return Response{StatusCode: 200}
}

// enqueueWebhookCommand converts one parsed command into queue work.
func enqueueWebhookCommand(ctx context.Context, telegramMessage telegram.Message, parsedCommand command.Command, dependencies Dependencies) (Response, bool) {
	action, err := command.ActionFor(parsedCommand, command.ChatContext{
		ChatID:    telegramMessage.Chat.ID,
		ChatTitle: telegramMessage.Chat.Title,
		UserID:    userID(telegramMessage.From),
	})
	if err == nil {
		if enqueueErr := dependencies.Enqueuer.Enqueue(ctx, action); enqueueErr != nil {
			return Response{StatusCode: 500, Message: "enqueue command"}, false
		}
		return Response{}, true
	}
	if shouldIgnoreCommandError(parsedCommand) {
		return Response{}, true
	}
	if sendErr := sendInvalidSetOffReply(ctx, telegramMessage, dependencies); sendErr != nil {
		return Response{StatusCode: 500, Message: "reply invalid command"}, false
	}

	return Response{}, true
}

// shouldIgnoreCommandError reports whether a command error should be skipped.
func shouldIgnoreCommandError(parsedCommand command.Command) bool {
	return parsedCommand.Name != command.SetOffFromWorkTimeUTC
}

// sendInvalidSetOffReply sends the contract reply for invalid off-work input.
func sendInvalidSetOffReply(ctx context.Context, telegramMessage telegram.Message, dependencies Dependencies) error {
	if dependencies.Messenger == nil {
		return fmt.Errorf("messenger is required")
	}

	return dependencies.Messenger.SendMessage(
		ctx,
		telegramMessage.Chat.ID,
		schedule.InvalidSetOffFromWorkTimeUTCMessage(telegramMessage.Chat.Title),
	)
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
	if dependencies.Messages == nil {
		return fmt.Errorf("message store is required")
	}
	if dependencies.Chats == nil {
		return fmt.Errorf("chat store is required")
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
	if response.Message != "" && allowsResponseBody(response.StatusCode) {
		if _, err := writer.Write([]byte(response.Message)); err != nil {
			logResponseWriteError("write plain response", response.StatusCode, err)
		}
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
	if err := json.NewEncoder(writer).Encode(body); err != nil {
		logResponseWriteError("encode JSON response", statusCode, err)
	}
}

// allowsResponseBody reports whether an HTTP status permits a response body.
func allowsResponseBody(statusCode int) bool {
	return statusCode != http.StatusNoContent && statusCode != http.StatusNotModified
}

// logResponseWriteError records response write failures after headers are in flight.
func logResponseWriteError(operation string, statusCode int, err error) {
	slog.Error(operation, "status_code", statusCode, "err", err)
}
