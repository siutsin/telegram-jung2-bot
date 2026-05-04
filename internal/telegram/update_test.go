package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUpdateDecodesSupportedMessage(t *testing.T) {
	payload := []byte(`{"message":{"chat":{"id":123,"title":"Test","type":"group"},"from":{"id":456,"first_name":"Ada","username":"ada"},"text":"/topTen","entities":[{"type":"bot_command"}]}}`)

	update, err := ParseUpdate(payload)
	require.NoError(t, err)
	require.NotNil(t, update.Message)

	assert.Equal(t, int64(123), update.Message.Chat.ID)
	assert.Equal(t, "Test", update.Message.Chat.Title)
	assert.Equal(t, "/topTen", update.Message.Text)
	assert.Equal(t, []Entity{{Type: "bot_command"}}, update.Message.Entities)
	require.NotNil(t, update.Message.From)
	assert.Equal(t, int64(456), update.Message.From.ID)
}

func TestParseUpdateAcceptsUnsupportedUpdate(t *testing.T) {
	update, err := ParseUpdate([]byte(`{"edited_message":{"text":"ignored"}}`))
	require.NoError(t, err)

	assert.Nil(t, update.Message)
}

func TestParseUpdateRejectsEmptyPayload(t *testing.T) {
	_, err := ParseUpdate([]byte(" "))
	require.Error(t, err)
}

func TestParseUpdateRejectsMalformedJSON(t *testing.T) {
	_, err := ParseUpdate([]byte(`{invalid json`))
	require.Error(t, err)
}

func TestTruncateReportKeepsShortText(t *testing.T) {
	assert.Equal(t, "hello", TruncateReport("hello"))
}

func TestTruncateReportKeepsValidUTF8(t *testing.T) {
	text := strings.Repeat("冗", ReportLimit+10)

	truncated := TruncateReport(text)

	assert.Equal(t, ReportLimit, utf8.RuneCountInString(truncated))
	assert.True(t, utf8.ValidString(truncated))
}

func TestSendMessagePostsContractPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/bottoken/sendMessage", request.URL.Path)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "application/json", request.Header.Get("Content-Type"))

		body, readErr := io.ReadAll(request.Body)
		assert.NoError(t, readErr)
		assert.JSONEq(t, `{"chat_id":123,"text":"hi"}`, string(body))

		response.WriteHeader(http.StatusOK)
		_, writeErr := response.Write([]byte(`{"ok":true}`))
		assert.NoError(t, writeErr)
	}))
	defer server.Close()

	client := NewClient("token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	err := client.SendMessage(context.Background(), 123, "hi")

	require.NoError(t, err)
}

func TestSendMessageWithOptionsPostsOptionalTelegramFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, readErr := io.ReadAll(request.Body)
		assert.NoError(t, readErr)
		assert.JSONEq(t, `{"chat_id":123,"disable_web_page_preview":true,"parse_mode":"markdown","text":"hi"}`, string(body))

		response.WriteHeader(http.StatusOK)
		_, writeErr := response.Write([]byte(`{"ok":true}`))
		assert.NoError(t, writeErr)
	}))
	defer server.Close()

	client := NewClient("token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	err := client.SendMessageWithOptions(context.Background(), 123, "hi", SendMessageOptions{
		DisableWebPagePreview: true,
		ParseMode:             "markdown",
	})

	require.NoError(t, err)
}

func TestSendMessageReturnsTelegramHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()

	client := NewClient("token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	err := client.SendMessage(context.Background(), 123, "hi")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 418")
}

func TestSendMessageReturnsRequestCreationError(t *testing.T) {
	client := NewClient("token", WithBaseURL("\n"))

	err := client.SendMessage(context.Background(), 123, "hi")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create Telegram sendMessage request")
}

func TestSendMessageReturnsHTTPClientError(t *testing.T) {
	client := NewClient("token", WithHTTPClient(&http.Client{Transport: failingRoundTripper{}}))

	err := client.SendMessage(context.Background(), 123, "hi")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "call Telegram sendMessage")
}

func TestGetChatAdministratorsDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/bottoken/getChatAdministrators", request.URL.Path)
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Equal(t, "123", request.URL.Query().Get("chat_id"))

		_, writeErr := response.Write([]byte(`{"ok":true,"result":[{"user":{"id":234,"first_name":"first_name","username":"username"}}]}`))
		assert.NoError(t, writeErr)
	}))
	defer server.Close()

	client := NewClient("token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	administrators, err := client.GetChatAdministrators(context.Background(), 123)

	require.NoError(t, err)
	require.Len(t, administrators, 1)
	assert.Equal(t, int64(234), administrators[0].User.ID)
	assert.Equal(t, "first_name", administrators[0].User.FirstName)
	assert.Equal(t, "username", administrators[0].User.UserName)
}

func TestGetChatAdministratorsReturnsDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, writeErr := response.Write([]byte(`{`))
		assert.NoError(t, writeErr)
	}))
	defer server.Close()

	client := NewClient("token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	_, err := client.GetChatAdministrators(context.Background(), 123)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode getChatAdministrators response")
}

func TestGetChatAdministratorsReturnsHTTPClientError(t *testing.T) {
	client := NewClient("token", WithHTTPClient(&http.Client{Transport: failingRoundTripper{}}))

	_, err := client.GetChatAdministrators(context.Background(), 123)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "call Telegram getChatAdministrators")
}

func TestIsAdminReportsMembership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, writeErr := response.Write([]byte(`{"ok":true,"result":[{"user":{"id":234}},{"user":{"id":345}}]}`))
		assert.NoError(t, writeErr)
	}))
	defer server.Close()

	client := NewClient("token", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	isAdmin, err := client.IsAdmin(context.Background(), 123, 234)
	require.NoError(t, err)
	assert.True(t, isAdmin)

	isAdmin, err = client.IsAdmin(context.Background(), 123, 999)
	require.NoError(t, err)
	assert.False(t, isAdmin)
}

func TestIsAdminReturnsLookupError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient("token", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithHTTPClient(nil))

	_, err := client.IsAdmin(context.Background(), 123, 234)

	require.Error(t, err)
}

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network down")
}
