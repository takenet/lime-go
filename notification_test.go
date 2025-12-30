package lime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createNotification() *Notification {
	n := Notification{}
	n.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	n.To = Node{}
	n.To.Name = "golang"
	n.To.Domain = "limeprotocol.org"
	n.To.Instance = "default"
	n.Event = NotificationEventReceived
	return &n
}

func TestNotificationSetEvent(t *testing.T) {
	not := &Notification{}

	result := not.SetEvent(NotificationEventReceived)

	assert.Equal(t, NotificationEventReceived, not.Event)
	assert.Equal(t, not, result, "should return self for chaining")
}

func TestNotificationSetFailed(t *testing.T) {
	not := &Notification{}
	reason := &Reason{
		Code:        500,
		Description: "Server error",
	}

	result := not.SetFailed(reason)

	assert.Equal(t, NotificationEventFailed, not.Event)
	assert.Equal(t, reason, not.Reason)
	assert.Equal(t, not, result, "should return self for chaining")
}

func TestNotificationMarshalJSON(t *testing.T) {
	not := &Notification{
		Envelope: Envelope{
			ID:   "test-id-123",
			From: Node{Identity: Identity{Name: "sender", Domain: testExampleDomain}},
			To:   Node{Identity: Identity{Name: "receiver", Domain: testExampleDomain}},
		},
		Event: NotificationEventReceived,
	}

	bytes, err := not.MarshalJSON()

	assert.NoError(t, err)
	assert.NotNil(t, bytes)

	// Verify it's valid JSON
	var raw map[string]interface{}
	err = json.Unmarshal(bytes, &raw)
	assert.NoError(t, err)
	assert.Equal(t, "test-id-123", raw["id"])
	assert.Equal(t, "received", raw["event"])
}

func TestNotificationUnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "test-id-456",
		"from": "sender@example.com",
		"to": "receiver@example.com",
		"event": "consumed"
	}`

	var not Notification
	err := not.UnmarshalJSON([]byte(jsonData))

	assert.NoError(t, err)
	assert.Equal(t, "test-id-456", not.ID)
	assert.Equal(t, "sender", not.From.Name)
	assert.Equal(t, "example.com", not.From.Domain)
	assert.Equal(t, NotificationEventConsumed, not.Event)
}

func TestNotificationUnmarshalJSONWithReason(t *testing.T) {
	jsonData := `{
		"id": "test-id-789",
		"event": "failed",
		"reason": {
			"code": 404,
			"description": "Not found"
		}
	}`

	var not Notification
	err := not.UnmarshalJSON([]byte(jsonData))

	assert.NoError(t, err)
	assert.Equal(t, NotificationEventFailed, not.Event)
	assert.NotNil(t, not.Reason)
	assert.Equal(t, 404, not.Reason.Code)
	assert.Equal(t, "Not found", not.Reason.Description)
}

func TestNotificationEventValidate(t *testing.T) {
	tests := []struct {
		name    string
		event   NotificationEvent
		wantErr bool
	}{
		{"accepted", NotificationEventAccepted, false},
		{"dispatched", NotificationEventDispatched, false},
		{"received", NotificationEventReceived, false},
		{"consumed", NotificationEventConsumed, false},
		{"failed", NotificationEventFailed, false},
		{"invalid", NotificationEvent("invalid"), true},
		{"empty", NotificationEvent(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNotificationEventMarshalText(t *testing.T) {
	event := NotificationEventReceived

	bytes, err := event.MarshalText()

	assert.NoError(t, err)
	assert.Equal(t, []byte("received"), bytes)
}

func TestNotificationEventMarshalTextInvalid(t *testing.T) {
	event := NotificationEvent("invalid")

	bytes, err := event.MarshalText()

	assert.Error(t, err)
	assert.Empty(t, bytes)
}

func TestNotificationEventUnmarshalText(t *testing.T) {
	var event NotificationEvent

	err := event.UnmarshalText([]byte("consumed"))

	assert.NoError(t, err)
	assert.Equal(t, NotificationEventConsumed, event)
}

func TestNotificationEventUnmarshalTextInvalid(t *testing.T) {
	var event NotificationEvent

	err := event.UnmarshalText([]byte("invalid-event"))

	assert.Error(t, err)
}

func TestNotificationChaining(t *testing.T) {
	not := &Notification{}
	reason := &Reason{Code: 500, Description: "Error"}

	result := not.
		SetEvent(NotificationEventAccepted).
		SetFailed(reason)

	assert.Equal(t, not, result)
	assert.Equal(t, NotificationEventFailed, not.Event)
	assert.Equal(t, reason, not.Reason)
}
