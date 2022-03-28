package messaging

import (
	"github.com/takenet/lime-go"
	"net/url"
	"time"
)

// Base type for contact information data.
type contact struct {
	// The user's identity, in the name@domain format.
	Identity *lime.Identity `json:"identity,omitempty"`
	// The user's street address.
	Address string `json:"address,omitempty"`
	// The user's city.
	City string `json:"city,omitempty"`
	// The user's e-mail address.
	Email string `json:"email,omitempty"`
	// The user's phone number.
	PhoneNumber string `json:"phoneNumber,omitempty"`
	// The user's photo URI.
	PhotoUri *url.URL `json:"photoUri,omitempty"`
	// The user's cellphone number
	CellPhoneNumber string `json:"cellPhoneNumber,omitempty"`
	// Represents the person gender.
	Gender string `json:"gender,omitempty"`
	// Represents the account time zone name.
	TimeZoneName string `json:"timeZoneName,omitempty"`
	// Represents the user's location time offset relative to UTC.
	Offset float32 `json:"offset,omitempty"`
	// Represents the person account culture info, in the IETF language tag format.
	// <a href="https://en.wikipedia.org/wiki/IETF_language_tag"/>.
	Culture string `json:"culture,omitempty"`
	// Contact extra information.
	Extras map[string]string `json:"extras,omitempty"`
	// Where the account came from.
	Source string `json:"source,omitempty"`
	// The contact first name.
	FirstName string `json:"firstName,omitempty"`
	// The contact last name.
	LastName string `json:"lastName,omitempty"`
	// The contact birth's date following ISO 8601.
	BirthDate *time.Time `json:"birthDate,omitempty"`
	// The contact tax document (CPF, CNPJ, social security number and others).
	TaxDocument string `json:"taxDocument,omitempty"`
	// Indicates when the contact was created.
	CreationDate *time.Time `json:"creationDate,omitempty"`
}

// Account represents a user account information.
type Account struct {
	contact
	// The user's full name.
	FullName string `json:"fullName,omitempty"`
	// Indicates that the account is temporary is valid only in the current session.
	IsTemporary *bool `json:"isTemporary,omitempty"`
	// Base64 representation of the account password.
	Password string `json:"password,omitempty"`
	// Base64 representation of the account password.
	// Mandatory in case of updating account password.
	OldPassword string `json:"oldPassword,omitempty"`
	// Size of account inbox for storing offline messages.
	InboxSize *int `json:"inboxSize,omitempty"`
	// Indicates if this account allows receiving messages from anonymous users.
	AllowAnonymousSender *bool `json:"allowAnonymousSender,omitempty"`
	// Indicates if this account allows receiving messages from users that are not in the account contact list.
	AllowUnknownSender *bool `json:"allowUnknownSender,omitempty"`
	// Indicates if the content of messages from this account should be stored in the server.
	// Note that for offline messages, this will always happen.
	StoreMessageContent *bool `json:"storeMessageContent,omitempty"`
	// Indicates if the content of messages from this account should be encrypted in the server.
	EncryptMessageContent *bool `json:"encryptMessageContent,omitempty"`
	// Access key for updating the account without knowing the old password.
	AccessKey string `json:"accessKey,omitempty"`
	// Alternative account address.
	AlternativeAccount *lime.Identity `json:"alternativeAccount,omitempty"`
	// Indicates if the account info should be published to the domain directory.
	PublishToDirectory *bool `json:"publishToDirectory,omitempty"`
}

func MediaTypeAccount() lime.MediaType {
	return lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.account",
		Suffix:  "json",
	}
}

func (a *Account) MediaType() lime.MediaType {
	return MediaTypeAccount()
}

// Contact represents a contact information.
type Contact struct {
	contact
	// The name of the contact.
	// This information is only visible by the roster owner.
	Name string `json:"name,omitempty"`
	// Determines if the contact is pending for acceptance by the roster owner.
	// The default value is false.
	IsPending *bool `json:"isPending,omitempty"`
	// Indicates if the roster owner wants to share presence information with the contact.
	// If true, the server provides a get delegation permission to the contact identity into the roster owner presence resource.
	// The default value is true.
	SharePresence *bool `json:"sharePresence,omitempty"`
	// Indicates if the roster owner wants to share account information with the contact.
	// If true, the server provides a get delegation permission to the contact identity into the roster owner account resource.
	// The default value is true.
	ShareAccountInfo *bool `json:"shareAccountInfo,omitempty"`
	// Indicates the contact priority in the roster.
	Priority *int `json:"priority,omitempty"`
	// Indicates the contact group name.
	Group string `json:"group,omitempty"`
	// Indicates the last message received or sent date off the contact.
	LastMessageDate *time.Time `json:"lastMessageDate,omitempty"`
}

func MediaTypeContact() lime.MediaType {
	return lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.contact",
		Suffix:  "json",
	}
}

func (c *Contact) MediaType() lime.MediaType {
	return MediaTypeContact()
}

// Presence represents the availability status of a node in a network.
// A node can only receive envelopes from another nodes in the network if it sets its presence to an available status
// (except from the server, who always knows if a node is available or node, since this information is enforced by the
// existing session). In a new session, the node starts with an unavailable status.
type Presence struct {
	Status           PresenceStatus `json:"status,omitempty"`
	Message          string         `json:"message,omitempty"`
	RoutingRule      RoutingRule    `json:"routingRule,omitempty"`
	LastSeen         *time.Time     `json:"lastSeen,omitempty"`
	Priority         *int           `json:"priority,omitempty"`
	FilterByDistance *bool          `json:"filterByDistance,omitempty"`
	RoundRobin       *bool          `json:"roundRobin,omitempty"`
	Echo             *bool          `json:"echo,omitempty"`
	Promiscuous      *bool          `json:"promiscuous,omitempty"`
	Instances        []string       `json:"instances,omitempty"`
}

func MediaTypePresence() lime.MediaType {
	return lime.MediaType{
		Type:    "application",
		Subtype: "vnd.lime.presence",
		Suffix:  "json",
	}
}

func (p *Presence) MediaType() lime.MediaType {
	return MediaTypePresence()
}

// PresenceStatus represents the possible presence status values.
type PresenceStatus string

const (
	// PresenceStatusUnavailable indicates that the node is not available for messaging and should not receive any
	// envelope by any node, except by the connected server.
	PresenceStatusUnavailable = PresenceStatus("unavailable")
	// PresenceStatusAvailable indicates that the node is available for messaging and envelopes can be routed to the
	// node according to the defined routing rule.
	PresenceStatusAvailable = PresenceStatus("available")
	// PresenceStatusBusy indicates that the node is available but the senders should notice that it is busy and doesn't
	//want to the disturbed, or it is on heavy load and don't want to receive any envelope.
	PresenceStatusBusy = PresenceStatus("busy")
	// PresenceStatusAway indicates that the node is available but the senders should notice that it may not be reading
	// or processing the received envelopes.
	PresenceStatusAway = PresenceStatus("away")
	// PresenceStatusInvisible indicates that the node is available for messaging but the actual stored presence value
	// is unavailable.
	PresenceStatusInvisible = PresenceStatus("invisible")
)

// RoutingRule defines the routing rules that should be applied by the server during the node's session for receiving
// envelopes.
type RoutingRule string

const (
	// RoutingRuleInstance indicates that the server should only deliver envelopes addressed to the current session
	// instance (name@domain/instance).
	RoutingRuleInstance = RoutingRule("instance")
	// RoutingRuleIdentity indicates that the server should deliver envelopes addressed to the current session instance
	// (name@domain/instance) and to the identity (name@domain).
	RoutingRuleIdentity = RoutingRule("identity")
	// RoutingRuleDomain indicates that the server should deliver envelopes addressed to the current session instance
	// (name@domain/instance) and to the node domain.
	// This rule is intended to be used only by sessions authenticated with DomainRole value as DomainRoleAuthority.
	RoutingRuleDomain = RoutingRule("domain")
	// RoutingRuleRootDomain indicates that the server should Deliver envelopes addressed to the current session
	// instance (name@domain/instance), to the node domain and all its subdomains.
	// This rule is intended to be used only by sessions authenticated with DomainRole value as DomainRoleRootAuthority.
	RoutingRuleRootDomain = RoutingRule("rootDomain")
)
