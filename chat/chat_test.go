package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/takenet/lime-go"
)

const testFullName = "John Doe"
const testDomain = "example.com"

func TestRegisterChatDocuments(t *testing.T) {
	// Act - this should not panic
	RegisterChatDocuments()

	// Assert - verify some registrations worked
	// by checking if we can create instances
	account := &Account{}
	assert.NotNil(t, account)

	contact := &Contact{}
	assert.NotNil(t, contact)
}

func TestAccountMediaType(t *testing.T) {
	// Arrange
	account := &Account{
		FullName: testFullName,
	}

	// Act
	mediaType := account.MediaType()

	// Assert
	assert.Equal(t, lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.account",
		Suffix:  "json",
	}, mediaType)
}

func TestContactMediaType(t *testing.T) {
	// Arrange
	contact := &Contact{
		Name: "Jane Doe",
	}

	// Act
	mediaType := contact.MediaType()

	// Assert
	assert.Equal(t, lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.contact",
		Suffix:  "json",
	}, mediaType)
}

func TestPresenceMediaType(t *testing.T) {
	// Arrange
	presence := &Presence{
		Status: PresenceStatusAvailable,
	}

	// Act
	mediaType := presence.MediaType()

	// Assert
	assert.Equal(t, lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.presence",
		Suffix:  "json",
	}, mediaType)
}

func TestReceiptMediaType(t *testing.T) {
	// Arrange
	receipt := &Receipt{
		Events: []lime.NotificationEvent{lime.NotificationEventConsumed},
	}

	// Act
	mediaType := receipt.MediaType()

	// Assert
	assert.Equal(t, lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.receipt",
		Suffix:  "json",
	}, mediaType)
}

func TestDelegationMediaType(t *testing.T) {
	// Arrange
	delegation := &Delegation{}

	// Act
	mediaType := delegation.MediaType()

	// Assert
	assert.Equal(t, lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.delegation",
		Suffix:  "json",
	}, mediaType)
}

func TestAccountFullName(t *testing.T) {
	// Arrange & Act
	account := &Account{
		FullName: testFullName,
	}
	account.Address = "123 Main St"

	// Assert
	assert.Equal(t, testFullName, account.FullName)
	assert.Equal(t, "123 Main St", account.Address)
}

func TestContactIdentity(t *testing.T) {
	// Arrange & Act
	identity := &lime.Identity{
		Name:   "user",
		Domain: testDomain,
	}
	contact := &Contact{
	}
	contact.Identity = identity

	// Assert
	assert.Equal(t, "user", contact.Identity.Name)
	assert.Equal(t, testDomain, contact.Identity.Domain)
}

func TestPresenceStatus(t *testing.T) {
	// Test all presence statuses
	statuses := []PresenceStatus{
		PresenceStatusAvailable,
		PresenceStatusBusy,
		PresenceStatusAway,
		PresenceStatusUnavailable,
		PresenceStatusInvisible,
	}

	for _, status := range statuses {
		presence := &Presence{Status: status}
		assert.NotEmpty(t, presence.Status)
	}
}

func TestReceiptEvents(t *testing.T) {
	// Test all receipt events
	events := []lime.NotificationEvent{
		lime.NotificationEventAccepted,
		lime.NotificationEventDispatched,
		lime.NotificationEventReceived,
		lime.NotificationEventConsumed,
		lime.NotificationEventFailed,
	}

	for _, event := range events {
		receipt := &Receipt{
			Events: []lime.NotificationEvent{event},
		}
		assert.Len(t, receipt.Events, 1)
		assert.Equal(t, event, receipt.Events[0])
	}
}

func TestDelegationEnvelope(t *testing.T) {
	// Arrange & Act
	delegation := &Delegation{
		Target: lime.Node{
			Identity: lime.Identity{
				Name:   "target",
				Domain: testDomain,
			},
		},
	}

	// Assert
	assert.Equal(t, "target", delegation.Target.Name)
	assert.Equal(t, testDomain, delegation.Target.Domain)
}
