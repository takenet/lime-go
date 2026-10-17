package channel

import (
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/transport"
)

type Identity = protocol.Identity
type Node = protocol.Node
type Envelope = protocol.Envelope
type EnvelopeValue = protocol.EnvelopeValue
type Reason = protocol.Reason
type Document = protocol.Document
type MediaType = protocol.MediaType
type Message = protocol.Message
type Notification = protocol.Notification
type NotificationEvent = protocol.NotificationEvent
type Command = protocol.Command
type RequestCommand = protocol.RequestCommand
type ResponseCommand = protocol.ResponseCommand
type CommandMethod = protocol.CommandMethod
type CommandStatus = protocol.CommandStatus
type URI = protocol.URI
type Session = protocol.Session
type SessionState = protocol.SessionState
type SessionEncryption = protocol.SessionEncryption
type SessionCompression = protocol.SessionCompression
type AuthenticationScheme = protocol.AuthenticationScheme
type Authentication = protocol.Authentication
type GuestAuthentication = protocol.GuestAuthentication
type PlainAuthentication = protocol.PlainAuthentication
type KeyAuthentication = protocol.KeyAuthentication
type TransportAuthentication = protocol.TransportAuthentication
type ExternalAuthentication = protocol.ExternalAuthentication

type Transport = transport.Transport
