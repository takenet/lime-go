package server

import (
	"github.com/takenet/lime-go/channel"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/transport"
	ws "github.com/takenet/lime-go/transport/websocket"
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
type Session = protocol.Session
type SessionCompression = protocol.SessionCompression
type SessionEncryption = protocol.SessionEncryption
type AuthenticationScheme = protocol.AuthenticationScheme
type Authentication = protocol.Authentication
type GuestAuthentication = protocol.GuestAuthentication
type TransportAuthentication = protocol.TransportAuthentication
type PlainAuthentication = protocol.PlainAuthentication
type KeyAuthentication = protocol.KeyAuthentication
type ExternalAuthentication = protocol.ExternalAuthentication
type WebsocketConfig = ws.WebsocketConfig
type Transport = transport.Transport
type TransportListener = transport.TransportListener
type ServerChannel = channel.ServerChannel
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
type DomainRole = channel.DomainRole
type AuthenticationResult = channel.AuthenticationResult
type CommandMethod = protocol.CommandMethod
type CommandStatus = protocol.CommandStatus

const (
	CommandMethodGet              = protocol.CommandMethodGet
	CommandStatusSuccess          = protocol.CommandStatusSuccess
	NotificationEventReceived     = protocol.NotificationEventReceived
	SessionStateNew               = protocol.SessionStateNew
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
	DomainRoleUnknown             = channel.DomainRoleUnknown
	DomainRoleMember              = channel.DomainRoleMember
)

func NewClientChannel(t Transport, bufferSize int) *channel.ClientChannel {
	return channel.NewClientChannel(t, bufferSize)
}

func NewServerChannel(t Transport, bufferSize int, node Node, sessionID string) *channel.ServerChannel {
	return channel.NewServerChannel(t, bufferSize, node, sessionID)
}

func UnknownAuthenticationResult() *AuthenticationResult {
	return channel.UnknownAuthenticationResult()
}

func MemberAuthenticationResult() *AuthenticationResult {
	return channel.MemberAuthenticationResult()
}
