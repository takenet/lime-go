package messaging

import (
	"github.com/takenet/lime-go"
	"net/url"
	"time"
)

type contact struct {
	Identity        *lime.Identity    `json:"identity,omitempty"`
	Address         string            `json:"address,omitempty"`
	City            string            `json:"city,omitempty"`
	Email           string            `json:"email,omitempty"`
	PhoneNumber     string            `json:"phoneNumber,omitempty"`
	PhotoUri        *url.URL          `json:"photoUri,omitempty"`
	CellPhoneNumber string            `json:"cellPhoneNumber,omitempty"`
	Gender          string            `json:"gender,omitempty"`
	TimeZoneName    string            `json:"timeZoneName,omitempty"`
	Offset          float32           `json:"offset,omitempty"`
	Culture         string            `json:"culture,omitempty"`
	Extras          map[string]string `json:"extras,omitempty"`
	Source          string            `json:"source,omitempty"`
	FirstName       string            `json:"firstName,omitempty"`
	LastName        string            `json:"lastName,omitempty"`
	BirthDate       time.Time         `json:"birthDate,omitempty"`
	TaxDocument     string            `json:"taxDocument,omitempty"`
	CreationDate    time.Time         `json:"creationDate,omitempty"`
}

type Account struct {
	contact
	FullName              string         `json:"fullName,omitempty"`
	IsTemporary           *bool          `json:"isTemporary,omitempty"`
	Password              string         `json:"password,omitempty"`
	OldPassword           string         `json:"oldPassword,omitempty"`
	InboxSize             *int           `json:"inboxSize,omitempty"`
	AllowAnonymousSender  *bool          `json:"allowAnonymousSender,omitempty"`
	AllowUnknownSender    *bool          `json:"allowUnknownSender,omitempty"`
	StoreMessageContent   *bool          `json:"storeMessageContent,omitempty"`
	EncryptMessageContent *bool          `json:"encryptMessageContent,omitempty"`
	AccessKey             string         `json:"accessKey,omitempty"`
	AlternativeAccount    *lime.Identity `json:"alternativeAccount,omitempty"`
	PublishToDirectory    *bool          `json:"publishToDirectory,omitempty"`
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
