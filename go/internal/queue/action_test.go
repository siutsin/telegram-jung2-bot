package queue

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeMessageSupportsStringValueCasing(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "upper case",
			raw:  `{"body":{"chatId":123},"messageAttributes":{"action":{"StringValue":"topten"}}}`,
		},
		{
			name: "lower case",
			raw:  `{"body":{"chatId":123},"messageAttributes":{"action":{"stringValue":"topten"}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var message RawMessage
			require.NoError(t, json.Unmarshal([]byte(test.raw), &message))

			action, err := DecodeMessage(message)
			require.NoError(t, err)

			assert.Equal(t, ActionTopTen, action.Name)
			assert.JSONEq(t, `{"chatId":123}`, string(action.Payload))
		})
	}
}

func TestDecodeMessageRejectsMissingAction(t *testing.T) {
	action, err := DecodeMessage(RawMessage{})

	require.Error(t, err)
	assert.Empty(t, action.Name)
}

func TestActionNamesRemainStable(t *testing.T) {
	assert.Equal(t, "junghelp", ActionJungHelp)
	assert.Equal(t, "topten", ActionTopTen)
	assert.Equal(t, "topdiver", ActionTopDiver)
	assert.Equal(t, "alljung", ActionAllJung)
	assert.Equal(t, "enableAllJung", ActionEnableAllJung)
	assert.Equal(t, "disableAllJung", ActionDisableAllJung)
	assert.Equal(t, "setOffFromWorkTimeUTC", ActionSetOffWorkTime)
	assert.Equal(t, "offFromWork", ActionOffFromWork)
	assert.Equal(t, "onOffFromWork", ActionOnOffFromWork)
}
