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

func (a *Contact) MediaType() lime.MediaType {
	return MediaTypeContact()
}
