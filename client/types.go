package client

import (
	"github.com/takenet/lime-go/channel"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/transport"
)

type Identity = protocol.Identity
type Node = protocol.Node
type Message = protocol.Message
type TextDocument = protocol.TextDocument
type RawDocument = protocol.RawDocument
type Notification = protocol.Notification
type RequestCommand = protocol.RequestCommand
type ResponseCommand = protocol.ResponseCommand
type Ping = protocol.Ping
type SessionCompression = protocol.SessionCompression
type SessionEncryption = protocol.SessionEncryption
type AuthenticationScheme = protocol.AuthenticationScheme
type Authentication = protocol.Authentication
type GuestAuthentication = protocol.GuestAuthentication
type TransportAuthentication = protocol.TransportAuthentication
type PlainAuthentication = protocol.PlainAuthentication
type KeyAuthentication = protocol.KeyAuthentication
type ExternalAuthentication = protocol.ExternalAuthentication
type Transport = transport.Transport
type ClientChannel = channel.ClientChannel
type EnvelopeMux = channel.EnvelopeMux
type Sender = channel.Sender
type MessageHandler = channel.MessageHandler
type MessagePredicate = channel.MessagePredicate
type MessageHandlerFunc = channel.MessageHandlerFunc
type NotificationHandler = channel.NotificationHandler
type NotificationPredicate = channel.NotificationPredicate
type NotificationHandlerFunc = channel.NotificationHandlerFunc
type RequestCommandHandler = channel.RequestCommandHandler
type RequestCommandPredicate = channel.RequestCommandPredicate
type RequestCommandHandlerFunc = channel.RequestCommandHandlerFunc
type ResponseCommandHandler = channel.ResponseCommandHandler
type ResponseCommandPredicate = channel.ResponseCommandPredicate
type ResponseCommandHandlerFunc = channel.ResponseCommandHandlerFunc
type CompressionSelector = channel.CompressionSelector
type EncryptionSelector = channel.EncryptionSelector
type Authenticator = channel.Authenticator
type CommandMethod = protocol.CommandMethod

const (
	CommandMethodGet              = protocol.CommandMethodGet
	CommandStatusSuccess          = protocol.CommandStatusSuccess
	NotificationEventReceived     = protocol.NotificationEventReceived
	SessionStateEstablished       = protocol.SessionStateEstablished
	SessionEncryptionNone         = protocol.SessionEncryptionNone
	SessionEncryptionTLS          = protocol.SessionEncryptionTLS
	SessionCompressionNone        = protocol.SessionCompressionNone
	SessionCompressionGzip        = protocol.SessionCompressionGzip
	AuthenticationSchemeGuest     = protocol.AuthenticationSchemeGuest
	AuthenticationSchemeTransport = protocol.AuthenticationSchemeTransport
	AuthenticationSchemePlain     = protocol.AuthenticationSchemePlain
	AuthenticationSchemeKey       = protocol.AuthenticationSchemeKey
	AuthenticationSchemeExternal  = protocol.AuthenticationSchemeExternal
)

var (
	NoneCompressionSelector = channel.NoneCompressionSelector
	NoneEncryptionSelector  = channel.NoneEncryptionSelector
	GuestAuthenticator      = channel.GuestAuthenticator
	TransportAuthenticator  = channel.TransportAuthenticator
)
