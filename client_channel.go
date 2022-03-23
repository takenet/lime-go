package lime

import (
	"context"
	"fmt"
)

// ClientChannel implements the client-side communication channel in a Lime session.
type ClientChannel struct {
	*channel
}

func NewClientChannel(t Transport, bufferSize int) *ClientChannel {
	c := newChannel(t, bufferSize)
	c.client = true
	return &ClientChannel{channel: c}
}

// receiveSessionFromServer receives a session from the remote node.
func (c *ClientChannel) receiveSessionFromServer(ctx context.Context) (*Session, error) {
	ses, err := c.receiveSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("receive session: %w", err)
	}

	if ses.State == SessionStateEstablished {
		c.localNode = ses.To
		c.remoteNode = ses.From
	}

	c.sessionID = ses.ID
	c.setState(ses.State)

	if ses.State == SessionStateFinished || ses.State == SessionStateFailed {
		if err := c.transport.Close(); err != nil {
			return nil, fmt.Errorf("closing the transport failed: %w", err)
		}
	}

	return ses, nil
}

// startNewSession sends a new session envelope to the server and awaits for the response.
func (c *ClientChannel) startNewSession(ctx context.Context) (*Session, error) {
	if err := c.ensureState(SessionStateNew, "start new session"); err != nil {
		return nil, err
	}

	if err := c.sendSession(ctx, &Session{State: SessionStateNew}); err != nil {
		return nil, fmt.Errorf("sending new session failed: %w", err)
	}

	ses, err := c.receiveSessionFromServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("receiving on new session failed: %w", err)
	}

	return ses, nil
}

// negotiateSession sends a "negotiate" session envelope to accept the session negotiation options and awaits for the server confirmation.
func (c *ClientChannel) negotiateSession(ctx context.Context, comp SessionCompression, encrypt SessionEncryption) (*Session, error) {
	if err := c.ensureState(SessionStateNegotiating, "negotiate session"); err != nil {
		return nil, err
	}

	negSes := Session{
		Envelope: Envelope{
			ID: c.sessionID,
		},
		State:       SessionStateNegotiating,
		Compression: comp,
		Encryption:  encrypt,
	}

	if err := c.sendSession(ctx, &negSes); err != nil {
		return nil, fmt.Errorf("sending negotiating session failed: %w", err)
	}

	ses, err := c.receiveSessionFromServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("receiving on session negotiation failed: %w", err)
	}

	return ses, nil
}

// authenticateSession send an "authenticate" session envelope to the server to establish an authenticated session and awaits for the response.
func (c *ClientChannel) authenticateSession(ctx context.Context, identity Identity, auth Authentication, instance string) (*Session, error) {
	if err := c.ensureState(SessionStateAuthenticating, "authenticate session"); err != nil {
		return nil, err
	}

	authSes := Session{
		Envelope: Envelope{
			ID: c.sessionID,
			From: Node{
				identity,
				instance,
			},
		},
		State: SessionStateAuthenticating,
	}
	authSes.SetAuthentication(auth)

	if err := c.sendSession(ctx, &authSes); err != nil {
		return nil, fmt.Errorf("sending authenticating session failed: %w", err)
	}

	ses, err := c.receiveSessionFromServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("receiving on session authentication failed: %w", err)
	}

	return ses, nil
}

func (c *ClientChannel) sendFinishingSession(ctx context.Context) error {
	if err := c.ensureState(SessionStateEstablished, "finish the session"); err != nil {
		return err
	}

	ses := Session{
		Envelope: Envelope{
			ID: c.sessionID,
		},
		State: SessionStateFinishing,
	}

	return c.sendSession(ctx, &ses)
}

// CompressionSelector defines a function for selecting the compression for a session.
type CompressionSelector func(options []SessionCompression) SessionCompression

var NoneCompressionSelector CompressionSelector = func(options []SessionCompression) SessionCompression {
	return SessionCompressionNone
}

// EncryptionSelector defines a function for selecting the encryption for a session.
type EncryptionSelector func(options []SessionEncryption) SessionEncryption

var NoneEncryptionSelector EncryptionSelector = func(options []SessionEncryption) SessionEncryption {
	return SessionEncryptionNone
}
var TLSEncryptionSelector EncryptionSelector = func(options []SessionEncryption) SessionEncryption {
	return SessionEncryptionTLS
}

type Authenticator func(schemes []AuthenticationScheme, roundTrip Authentication) Authentication

var GuestAuthenticator Authenticator = func(schemes []AuthenticationScheme, roundTrip Authentication) Authentication {
	return &GuestAuthentication{}
}

var TransportAuthenticator Authenticator = func(schemes []AuthenticationScheme, roundTrip Authentication) Authentication {
	return &TransportAuthentication{}
}

// EstablishSession performs the client session negotiation and authentication handshake.
func (c *ClientChannel) EstablishSession(
	ctx context.Context,
	compSelector CompressionSelector,
	encryptSelector EncryptionSelector,
	identity Identity,
	authenticator Authenticator,
	instance string,
) (*Session, error) {
	if authenticator == nil {
		panic("the authenticator should not be nil")
	}

	if c.state != SessionStateNew {
		panic("channel state is not new")
	}

	ses, err := c.startNewSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("establish session: %w", err)
	}

	// Session negotiation
	if ses.State == SessionStateNegotiating {
		if compSelector == nil {
			panic("nil compression selector")
		}

		if encryptSelector == nil {
			panic("nil encrypt selector")
		}

		// Select options
		ses, err = c.negotiateSession(
			ctx,
			compSelector(ses.CompressionOptions),
			encryptSelector(ses.EncryptionOptions))
		if err != nil {
			return nil, fmt.Errorf("establish session: %w", err)
		}

		if ses.State == SessionStateNegotiating {
			if ses.Compression != "" && ses.Compression != c.transport.Compression() {
				err = c.transport.SetCompression(ctx, ses.Compression)
				if err != nil {
					return nil, fmt.Errorf("establish session: set compression: %w", err)
				}
			}
			if ses.Encryption != "" && ses.Encryption != c.transport.Encryption() {
				err = c.transport.SetEncryption(ctx, ses.Encryption)
				if err != nil {
					return nil, fmt.Errorf("establish session: set encryption: %w", err)
				}
			}
		}

		// Await for authentication options
		ses, err = c.receiveSessionFromServer(ctx)
		if err != nil {
			return nil, fmt.Errorf("establish session: %w", err)
		}
	}

	// Session authentication
	var roundTrip Authentication

	for ses.State == SessionStateAuthenticating {
		ses, err = c.authenticateSession(
			ctx,
			identity,
			authenticator(ses.SchemeOptions, roundTrip),
			instance,
		)
		if err != nil {
			return nil, fmt.Errorf("establish session: %w", err)
		}
		roundTrip = ses.Authentication
	}

	return ses, nil
}

// FinishSession performs the session finishing handshake.
func (c *ClientChannel) FinishSession(ctx context.Context) (*Session, error) {
	if err := c.sendFinishingSession(ctx); err != nil {
		return nil, fmt.Errorf("finish session: %w", err)
	}

	ses, err := c.receiveSessionFromServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("finish session: %w", err)
	}

	return ses, nil
}
