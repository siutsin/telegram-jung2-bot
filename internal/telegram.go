package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ReportLimit is the project safety limit below Telegram's 4096 character cap.
const ReportLimit = 3800

const defaultAPIBaseURL = "https://api.telegram.org"

// maxErrorBodyBytes bounds how much of a Telegram error body is read, so
// a broken or hostile response cannot grow memory without limit.
const maxErrorBodyBytes = 4096

const frozenDescription = "FROZEN_METHOD_INVALID"

// Client calls the Telegram Bot API.
type Client struct {
	baseURL    string
	botToken   string
	httpClient *http.Client
	metrics    dependencyObserver
}

type SendMessageOptions struct {
	DisableWebPagePreview bool
	ParseMode             string
}

// ClientOption customises a Telegram client.
type ClientOption func(*Client)

//go:generate sh -c "GOFLAGS=-mod=mod go run go.uber.org/mock/mockgen -source=telegram.go -destination=mock/telegram_mock.go -package=mock -mock_names dependencyObserver=MockTelegramDependencyObserver"

// dependencyObserver records a completed Telegram API request.
type dependencyObserver interface {
	ObserveDependency(dependency string, operation string, duration time.Duration, err error)
}

// WithBaseURL overrides the Telegram API base URL for tests or local proxies.
// For example, "https://api.example.com/" becomes baseURL
// "https://api.example.com".
func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) {
		client.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

// WithDependencyObserver records Telegram API request metrics.
func WithDependencyObserver(metrics dependencyObserver) ClientOption {
	return func(client *Client) {
		client.metrics = metrics
	}
}

// NewClient creates a Telegram Bot API client.
// For example, NewClient("token", WithBaseURL("https://api.example.com/"))
// stores the trimmed custom base URL.
func NewClient(botToken string, options ...ClientOption) Client {
	client := Client{
		baseURL:    defaultAPIBaseURL,
		botToken:   botToken,
		httpClient: http.DefaultClient,
	}

	for _, option := range options {
		option(&client)
	}

	return client
}

// Update is the subset of Telegram updates used by this service.
type Update struct {
	UpdateID int64            `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
}

// TelegramMessage is the subset of Telegram messages used by this service.
type TelegramMessage struct {
	MessageID int64    `json:"message_id"`
	Date      int64    `json:"date"`
	Text      string   `json:"text"`
	Chat      Chat     `json:"chat"`
	From      *User    `json:"from,omitempty"`
	Entities  []Entity `json:"entities,omitempty"`
}

type Entity struct {
	Type string `json:"type"`
}

// Chat identifies a Telegram chat.
type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type,omitempty"`
}

// User identifies a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	UserName  string `json:"username,omitempty"`
}

// Administrator identifies a Telegram chat administrator.
type Administrator struct {
	User User `json:"user"`
}

// SendMessage sends text to a Telegram chat.
func (client Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return client.SendMessageWithOptions(ctx, chatID, text, SendMessageOptions{})
}

// SendMessageWithOptions sends text to a Telegram chat with optional Telegram
// API sendMessage fields. For example, ParseMode "Markdown" adds
// parse_mode="Markdown" to the request body.
func (client Client) SendMessageWithOptions(ctx context.Context, chatID int64, text string, options SendMessageOptions) (err error) {
	started := time.Now()
	defer func() { client.observe("send_message", started, err) }()
	payload := sendMessagePayload(chatID, text, options)
	response, err := client.do(ctx, http.MethodPost, "sendMessage", nil, payload)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, "sendMessage"))
	}()

	err = telegramAPIError(response)
	if err != nil {
		return err
	}

	return nil
}

func sendMessagePayload(chatID int64, text string, options SendMessageOptions) []byte {
	payload := []byte(`{"chat_id":`)
	payload = strconv.AppendInt(payload, chatID, 10)
	if options.DisableWebPagePreview {
		payload = append(payload, `,"disable_web_page_preview":true`...)
	}
	if options.ParseMode != "" {
		payload = append(payload, `,"parse_mode":`...)
		payload = strconv.AppendQuote(payload, options.ParseMode)
	}
	payload = append(payload, `,"text":`...)
	payload = strconv.AppendQuote(payload, text)
	payload = append(payload, '}')

	return payload
}

// GetChatAdministrators returns the administrators of a Telegram chat.
func (client Client) GetChatAdministrators(ctx context.Context, chatID int64) (administrators []Administrator, err error) {
	started := time.Now()
	defer func() { client.observe("get_administrators", started, err) }()
	query := url.Values{"chat_id": {fmt.Sprint(chatID)}}
	response, err := client.do(ctx, http.MethodGet, "getChatAdministrators", query, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, "getChatAdministrators"))
	}()

	err = telegramAPIError(response)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result []Administrator `json:"result"`
	}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("decode getChatAdministrators response: %w", err)
	}

	return result.Result, nil
}

// GetMyCommands calls getMyCommands and discards the command list. The call
// has no side effects and fails on a frozen account, so the readiness
// checker uses it to read the account state. For example, a frozen account
// returns an error that IsFrozen reports as true.
func (client Client) GetMyCommands(ctx context.Context) (err error) {
	started := time.Now()
	defer func() { client.observe("get_my_commands", started, err) }()
	response, err := client.do(ctx, http.MethodGet, "getMyCommands", nil, nil)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, "getMyCommands"))
	}()

	return telegramAPIError(response)
}

// observe records a completed Telegram API request when metrics are enabled.
func (client Client) observe(operation string, started time.Time, err error) {
	if client.metrics != nil {
		client.metrics.ObserveDependency("telegram", operation, time.Since(started), err)
	}
}

// IsAdmin reports whether userID is an administrator of chatID.
func (client Client) IsAdmin(ctx context.Context, chatID int64, userID int64) (bool, error) {
	administrators, err := client.GetChatAdministrators(ctx, chatID)
	if err != nil {
		return false, err
	}

	for _, administrator := range administrators {
		if administrator.User.ID == userID {
			return true, nil
		}
	}

	return false, nil
}

// ParseUpdate decodes a Telegram webhook update.
// For example, {"message":{"text":"/topTen"}} becomes Update with Message.Text
// set to "/topTen".
func ParseUpdate(payload []byte) (Update, error) {
	if strings.TrimSpace(string(payload)) == "" {
		return Update{}, fmt.Errorf("telegram update payload is empty")
	}

	var update Update
	err := json.Unmarshal(payload, &update)
	if err != nil {
		return Update{}, fmt.Errorf("decode telegram update: %w", err)
	}

	return update, nil
}

// do sends a Telegram API request.
func (client Client) do(
	ctx context.Context,
	method string,
	endpoint string,
	query url.Values,
	payload []byte,
) (*http.Response, error) {
	requestURL := fmt.Sprintf("%s/bot%s/%s", client.baseURL, client.botToken, endpoint)
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create Telegram %s request: %w", endpoint, err)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, telegramTransportError(endpoint, err)
	}

	return response, nil
}

// telegramTransportError retains useful request state without exposing the bot token in a request URL.
// For example, a cancelled request becomes "call Telegram sendMessage: request cancelled".
func telegramTransportError(endpoint string, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("call Telegram %s: request cancelled", endpoint)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("call Telegram %s: request timed out", endpoint)
	default:
		return fmt.Errorf("call Telegram %s: transport request failed", endpoint)
	}
}

// APIError is a Telegram API error response. StatusCode is the HTTP status
// and Description is the error text Telegram returned. For example, a
// frozen account returns StatusCode 400 and Description
// "FROZEN_METHOD_INVALID".
type APIError struct {
	StatusCode      int
	Description     string
	MigrateToChatID int64
}

// Error returns the API error in the shape existing callers already log.
// For example, it returns "telegram API returned HTTP 400:
// FROZEN_METHOD_INVALID", or "telegram API returned HTTP 500" when Telegram
// sent no description.
func (apiErr *APIError) Error() string {
	if apiErr.Description == "" {
		return fmt.Sprintf("telegram API returned HTTP %d", apiErr.StatusCode)
	}

	return fmt.Sprintf("telegram API returned HTTP %d: %s", apiErr.StatusCode, apiErr.Description)
}

// IsFrozen reports whether err is a Telegram API error for a frozen
// account. Telegram prefixes the frozen code in live replies, so an
// APIError with description "Bad Request: FROZEN_METHOD_INVALID" returns
// true, while a rate limit returns false.
func IsFrozen(err error) bool {
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		return false
	}

	return strings.HasSuffix(apiErr.Description, frozenDescription)
}

// telegramAPIError converts non-2xx responses into errors. It reads the
// error body (bounded) so the returned APIError carries the description
// Telegram sent. For example, HTTP 429 with a "Too Many Requests"
// description becomes error "telegram API returned HTTP 429: Too Many
// Requests".
func telegramAPIError(response *http.Response) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	apiErr := &APIError{StatusCode: response.StatusCode}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
	if err != nil {
		return errors.Join(apiErr, fmt.Errorf("drain telegram API error response: %w", err))
	}

	var envelope struct {
		Description string `json:"description"`
		Parameters  struct {
			MigrateToChatID int64 `json:"migrate_to_chat_id"`
		} `json:"parameters"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		apiErr.Description = envelope.Description
		apiErr.MigrateToChatID = envelope.Parameters.MigrateToChatID
	}

	return apiErr
}

// closeResponseBody closes a Telegram HTTP response body with context.
func closeResponseBody(body io.Closer, endpoint string) error {
	err := body.Close()
	if err != nil {
		return fmt.Errorf("close Telegram %s response body: %w", endpoint, err)
	}

	return nil
}
