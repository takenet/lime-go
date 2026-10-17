package channel

import "github.com/takenet/lime-go/protocol"

const (
	CommandMethodGet         = protocol.CommandMethodGet
	CommandMethodSet         = protocol.CommandMethodSet
	CommandMethodDelete      = protocol.CommandMethodDelete
	CommandMethodSubscribe   = protocol.CommandMethodSubscribe
	CommandMethodUnsubscribe = protocol.CommandMethodUnsubscribe
	CommandMethodObserve     = protocol.CommandMethodObserve
	CommandMethodMerge       = protocol.CommandMethodMerge
	CommandStatusSuccess     = protocol.CommandStatusSuccess
	CommandStatusFailure     = protocol.CommandStatusFailure
)

const (
	NotificationEventAccepted   = protocol.NotificationEventAccepted
	NotificationEventDispatched = protocol.NotificationEventDispatched
	NotificationEventReceived   = protocol.NotificationEventReceived
	NotificationEventConsumed   = protocol.NotificationEventConsumed
	NotificationEventFailed     = protocol.NotificationEventFailed
)

const (
	SessionStateNew               = protocol.SessionStateNew
	SessionStateNegotiating       = protocol.SessionStateNegotiating
	SessionStateAuthenticating    = protocol.SessionStateAuthenticating
	SessionStateEstablished       = protocol.SessionStateEstablished
	SessionStateFinishing         = protocol.SessionStateFinishing
	SessionStateFinished          = protocol.SessionStateFinished
	SessionStateFailed            = protocol.SessionStateFailed
	SessionEncryptionNone         = protocol.SessionEncryptionNone
	SessionEncryptionTLS          = protocol.SessionEncryptionTLS
	SessionCompressionNone        = protocol.SessionCompressionNone
	SessionCompressionGzip        = protocol.SessionCompressionGzip
	AuthenticationSchemeGuest     = protocol.AuthenticationSchemeGuest
	AuthenticationSchemePlain     = protocol.AuthenticationSchemePlain
	AuthenticationSchemeKey       = protocol.AuthenticationSchemeKey
	AuthenticationSchemeTransport = protocol.AuthenticationSchemeTransport
	AuthenticationSchemeExternal  = protocol.AuthenticationSchemeExternal
)
