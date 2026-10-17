package resources

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/document"
	lime "github.com/takenet/lime-go/protocol"
)

const testFullName = "John Doe"
const testDomain = "example.com"

func TestRegisterResources(t *testing.T) {
	RegisterResources()
	for _, test := range []struct {
		mediaType document.MediaType
		want      document.Document
	}{
		{MediaTypeAccount(), &Account{}},
		{MediaTypeContact(), &Contact{}},
		{MediaTypePresence(), &Presence{}},
		{MediaTypeReceipt(), &Receipt{}},
		{MediaTypeDelegation(), &Delegation{}},
		{MediaTypeGroup(), &Group{}},
		{MediaTypeGroupMember(), &GroupMember{}},
		{MediaTypeCapability(), &Capability{}},
		{MediaTypeQuota(), &Quota{}},
	} {
		factory, err := document.GetDocumentFactory(test.mediaType)
		require.NoError(t, err)
		assert.IsType(t, test.want, factory())
	}
}

func TestContactMarshalsRichResourceFields(t *testing.T) {
	contact := &Contact{
		ContactDocument: ContactDocument{
			Identity:         &lime.Identity{Name: "user", Domain: testDomain},
			PhotoURI:         "https://example.com/photo.png",
			Gender:           ptr(GenderOther),
			Offset:           -3.5,
			TenantID:         "tenant-1",
			IsGroupEnabled:   ptr(true),
			WhatsAppBSUID:    "bsuid-1",
			WhatsAppWAID:     "waid-1",
			WhatsAppUserName: "user-name",
			WhatsAppParentID: "parent-1",
		},
		SubscriptionStatus: ptr(SubscriptionStatusNotSubscribed),
		LastUpdateDate:     ptr(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
	}

	data, err := json.Marshal(contact)
	require.NoError(t, err)
	assert.JSONEq(t, `{"identity":"user@example.com","photoUri":"https://example.com/photo.png","gender":"other","offset":-3.5,"tenantId":"tenant-1","isGroupEnabled":true,"whatsAppBsuid":"bsuid-1","whatsAppWaId":"waid-1","whatsAppUserName":"user-name","whatsAppParentId":"parent-1","subscriptionStatus":"not-subscribed","lastUpdateDate":"2026-01-02T03:04:05Z"}`, string(data))
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
	contact := &Contact{}
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

func TestGroupResourceWireShape(t *testing.T) {
	groupType := GroupTypePublic
	uri, err := lime.ParseLimeURI("/groups/team")
	require.NoError(t, err)
	group := &Group{
		Identity:    lime.Identity{Name: "team", Domain: "groups.example.com"},
		Name:        "Team",
		Description: "Example group",
		Type:        &groupType,
		PhotoURI:    "https://example.com/team.png",
		Creator:     lime.Identity{Name: "owner", Domain: testDomain},
		MembersURI:  uri,
	}
	encoded, err := json.Marshal(group)
	require.NoError(t, err)
	assert.JSONEq(t, `{"identity":"team@groups.example.com","name":"Team","description":"Example group","type":"public","photoUri":"https://example.com/team.png","creator":"owner@example.com","created":"0001-01-01T00:00:00Z","members":"/groups/team"}`, string(encoded))
}

func TestCapabilityQuotaAndGroupMemberWireShapes(t *testing.T) {
	capabilityJSON, err := json.Marshal(&Capability{
		ContentTypes:  []document.MediaType{lime.MediaTypeTextPlain()},
		ResourceTypes: []document.MediaType{MediaTypeContact()},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"contentTypes":["text/plain"],"resourceTypes":["application/vnd.lime.contact+json"]}`, string(capabilityJSON))

	threshold, throughput, maxSize := 100, 20, 4096
	quotaJSON, err := json.Marshal(&Quota{Threshold: &threshold, Throughput: &throughput, MaxEnvelopeSize: &maxSize})
	require.NoError(t, err)
	assert.JSONEq(t, `{"threshold":100,"throughput":20,"maxEnvelopeSize":4096}`, string(quotaJSON))

	memberJSON, err := json.Marshal(&GroupMember{
		Address: lime.Node{Identity: lime.Identity{Name: "member", Domain: testDomain}, Instance: "one"},
		Role:    GroupMemberRoleModerator,
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"address":"member@example.com/one","role":"moderator"}`, string(memberJSON))
}

func TestDelegationUsesProtocolJSONFieldNames(t *testing.T) {
	uri, err := lime.ParseLimeURI("/contacts")
	require.NoError(t, err)
	delegation := &Delegation{
		Target:        lime.Node{Identity: lime.Identity{Name: "target", Domain: testDomain}},
		EnvelopeTypes: []lime.EnvelopeType{lime.EnvelopeTypeMessage, lime.EnvelopeTypeCommand},
		Messages:      []DelegationMessage{{Type: lime.MediaTypeTextPlain()}},
		Notifications: []DelegationNotification{{Event: lime.NotificationEventReceived}},
		Commands:      []DelegationCommand{{Method: lime.CommandMethodGet, URI: uri}},
	}
	encoded, err := json.Marshal(delegation)
	require.NoError(t, err)
	assert.JSONEq(t, `{"target":"target@example.com","envelopeTypes":["message","command"],"messages":[{"type":"text/plain"}],"notifications":[{"event":"received"}],"commands":[{"method":"get","uri":"/contacts"}]}`, string(encoded))
}

func ptr[T any](value T) *T { return &value }
