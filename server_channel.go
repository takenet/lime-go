package lime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type ServerChannel struct {
	*channel
}

func NewServerChannel(t Transport, bufferSize int, serverNode Node, sessionID string) *ServerChannel {
	if !serverNode.IsComplete() {
		panic("the server node must be complete")
	}

	if sessionID == "" {
		panic("the sessionID cannot be zero")
	}

	c := newChannel(t, bufferSize)
	c.localNode = serverNode
	c.sessionID = sessionID

	return &ServerChannel{channel: c}
}

// receiveNewSession receives a new session envelope from the client node.
func (c *ServerChannel) receiveNewSession(ctx context.Context) (*Session, error) {
	if err := c.ensureState(SessionStateNew, "receive new session"); err != nil {
		return nil, err
	}

	return c.receiveSession(ctx)
}

// sendNegotiatingOptionsSession changes the session state and sends a "negotiating" session envelope with the available options to the client node and awaits for the selected option.
func (c *ServerChannel) sendNegotiatingOptionsSession(ctx context.Context, compOptions []SessionCompression, encryptOptions []SessionEncryption) (*Session, error) {
	if compOptions == nil || len(compOptions) == 0 {
		return nil, errors.New("no available options for compression negotiation")
	}
	if encryptOptions == nil || len(encryptOptions) == 0 {
		return nil, errors.New("no available options for encryption negotiation")
	}
	if err := c.ensureState(SessionStateNew, "negotiate session"); err != nil {
		return nil, err
	}

	c.setState(SessionStateNegotiating)

	ses := Session{
		EnvelopeBase: EnvelopeBase{
			ID:   c.sessionID,
			From: c.localNode,
		},
		State:              SessionStateNegotiating,
		CompressionOptions: compOptions,
		EncryptionOptions:  encryptOptions,
	}
	if err := c.sendSession(ctx, &ses); err != nil {
		return nil, err
	}

	return c.receiveSession(ctx)
}

// sendNegotiatingConfirmationSession send a "negotiating" session envelope to the client node to confirm the session negotiation options.
func (c *ServerChannel) sendNegotiatingConfirmationSession(ctx context.Context, comp SessionCompression, encrypt SessionEncryption) error {
	if err := c.ensureState(SessionStateNegotiating, "send negotiating session"); err != nil {
		return err
	}

	ses := Session{
		EnvelopeBase: EnvelopeBase{
			ID:   c.sessionID,
			From: c.localNode,
		},
		State:       SessionStateNegotiating,
		Compression: comp,
		Encryption:  encrypt,
	}
	return c.sendSession(ctx, &ses)
}

// sendAuthenticatingSession changes the session state and sends an "authenticating" envelope with the available scheme options to the client node and awaits for the authentication.
func (c *ServerChannel) sendAuthenticatingSession(ctx context.Context, schemeOpts []AuthenticationScheme) (*Session, error) {
	if schemeOpts == nil || len(schemeOpts) == 0 {
		return nil, errors.New("there's no available options for authentication")
	}
	if err := c.ensureTransportOK("authenticate session"); err != nil {
		return nil, err
	}
	if c.state != SessionStateNew && c.state != SessionStateNegotiating {
		return nil, fmt.Errorf("cannot authenticate session in the %v state", c.state)
	}

	c.setState(SessionStateAuthenticating)

	ses := Session{
		EnvelopeBase: EnvelopeBase{
			ID:   c.sessionID,
			From: c.localNode,
		},
		State:         SessionStateAuthenticating,
		SchemeOptions: schemeOpts,
	}
	if err := c.sendSession(ctx, &ses); err != nil {
		return nil, err
	}

	return c.receiveSession(ctx)
}

// sendAuthenticatingRoundTripSession sends authentication round-trip information to the connected node and awaits for the client authentication.
func (c *ServerChannel) sendAuthenticatingRoundTripSession(ctx context.Context, roundTrip Authentication) (*Session, error) {
	if roundTrip == nil {
		panic("auth roundTrip cannot be nil")
	}
	if err := c.ensureState(SessionStateAuthenticating, "perform authentication roundTrip"); err != nil {
		return nil, err
	}

	ses := Session{
		EnvelopeBase: EnvelopeBase{
			ID:   c.sessionID,
			From: c.localNode,
		},
		State:          SessionStateAuthenticating,
		Authentication: roundTrip,
	}
	if err := c.sendSession(ctx, &ses); err != nil {
		return nil, err
	}

	return c.receiveSession(ctx)
}

// sendEstablishedSession changes the session state to "established" and sends a session envelope to the node to communicate the establishment of the session.
func (c *ServerChannel) sendEstablishedSession(ctx context.Context, node Node) error {
	if err := c.ensureTransportOK("send established session"); err != nil {
		return err
	}
	if c.state != SessionStateNew && c.state != SessionStateNegotiating && c.state != SessionStateAuthenticating {
		return fmt.Errorf("cannot establish the session in the %v state", c.state)
	}

	c.setState(SessionStateEstablished)

	c.remoteNode = node

	ses := Session{
		EnvelopeBase: EnvelopeBase{
			ID:   c.sessionID,
			From: c.localNode,
			To:   c.remoteNode,
		},
		State: SessionStateEstablished,
	}
	return c.sendSession(ctx, &ses)
}

// DomainRole indicates the role of an identity in a domain.
type DomainRole string

const (
	DomainRoleUnknown       = DomainRole("unknown")       // The identity is not part of the domain.
	DomainRoleMember        = DomainRole("member")        // The identity is a member of the domain.
	DomainRoleAuthority     = DomainRole("authority")     // The identity is an authority of the domain.
	DomainRoleRootAuthority = DomainRole("rootAuthority") // The identity is an authority of the domain and its subdomains.
)

// AuthenticationResult represents the result of a session authentication.
type AuthenticationResult struct {
	Role      DomainRole
	RoundTrip Authentication
}

// EstablishSession establishes a server channel with transport options negotiation and authentication.
func (c *ServerChannel) EstablishSession(
	ctx context.Context,
	compOpts []SessionCompression,
	encryptOpts []SessionEncryption,
	schemeOpts []AuthenticationScheme,
	authFunc func(Identity, Authentication) (AuthenticationResult, error),
	registerFunc func(Node, *ServerChannel) (Node, error)) error {

	if err := c.ensureTransportOK("establish session"); err != nil {
		return err
	}
	if compOpts == nil {
		panic("compOpts cannot be nil")
	}
	if encryptOpts == nil {
		panic("encryptOpts cannot be nil")
	}
	if authFunc == nil {
		panic("authentication func cannot be nil")
	}
	if registerFunc == nil {
		panic("registration func cannot be nil")
	}

	ses, err := c.receiveNewSession(ctx)
	if err != nil {
		return err
	}

	if ses.ID != "" {
		return c.FailSession(ctx, &Reason{
			Code:        1,
			Description: "Invalid session id",
		})
	}

	if ses.State == SessionStateNew {
		// Check if there's any transport negotiation option to be presented to the client
		negCompOpts := make([]SessionCompression, 0)
		for _, v := range intersect(compOpts, c.transport.GetSupportedCompression()) {
			negCompOpts = append(negCompOpts, v.(SessionCompression))
		}
		if encryptOpts == nil {
			encryptOpts = []SessionEncryption{}
		}
		negEncryptOpts := make([]SessionEncryption, 0)
		for _, v := range intersect(encryptOpts, c.transport.GetSupportedEncryption()) {
			negEncryptOpts = append(negEncryptOpts, v.(SessionEncryption))
		}

		if len(negCompOpts) > 1 || len(negEncryptOpts) > 1 {
			// Negotiate the session options
			if err = c.negotiateSession(ctx, negCompOpts, negEncryptOpts); err != nil {
				return err
			}
		}

		// Proceed to the authentication if the channel is not failed
		if c.state != SessionStateFailed {
			if err = c.authenticateSession(ctx, schemeOpts, authFunc, registerFunc); err != nil {
				return err
			}
		}
	}

	// If the channel state is not final at this point, fail the session
	if c.state != SessionStateEstablished && c.state != SessionStateFailed && c.transport.Connected() {
		return c.FailSession(ctx, &Reason{
			Code:        1,
			Description: "The session establishment failed",
		})
	}

	return nil
}

func (c *ServerChannel) negotiateSession(ctx context.Context, compOpts []SessionCompression, encryptOpts []SessionEncryption) error {
	ses, err := c.sendNegotiatingOptionsSession(ctx, compOpts, encryptOpts)
	if err != nil {
		return err
	}

	if ses.ID != c.sessionID {
		return c.FailSession(ctx, &Reason{
			Code:        1,
			Description: "Invalid session id",
		})
	}

	// Convert the slices to maps for lookup
	compOptsMap := make(map[SessionCompression]struct{}, len(compOpts))
	for _, v := range compOpts {
		compOptsMap[v] = struct{}{}
	}
	encryptOptsMap := make(map[SessionEncryption]struct{}, len(encryptOpts))
	for _, v := range encryptOpts {
		encryptOptsMap[v] = struct{}{}
	}

	if ses.State == SessionStateNegotiating && ses.Compression != "" && ses.Encryption != "" {
		if _, ok := compOptsMap[ses.Compression]; ok {
			if _, ok := encryptOptsMap[ses.Encryption]; ok {
				if err := c.sendNegotiatingConfirmationSession(ctx, ses.Compression, ses.Encryption); err != nil {
					return err
				}

				if c.transport.GetCompression() != ses.Compression {
					if err = c.transport.SetCompression(ctx, ses.Compression); err != nil {
						return err
					}
				}

				if c.transport.GetEncryption() != ses.Encryption {
					if err = c.transport.SetEncryption(ctx, ses.Encryption); err != nil {
						return err
					}
				}

				return nil
			}
		}
	}

	return c.FailSession(ctx, &Reason{
		Code:        1,
		Description: "An invalid negotiation option was selected",
	})
}

func (c *ServerChannel) authenticateSession(
	ctx context.Context,
	schemeOpts []AuthenticationScheme,
	authFunc func(Identity, Authentication) (AuthenticationResult, error),
	registerFunc func(Node, *ServerChannel) (Node, error)) error {
	// Convert the slice to a map for lookup
	schemeOptsMap := make(map[AuthenticationScheme]struct{})
	for _, v := range schemeOpts {
		schemeOptsMap[v] = struct{}{}
	}

	ses, err := c.sendAuthenticatingSession(ctx, schemeOpts)
	if err != nil {
		return err
	}

	for c.state == SessionStateAuthenticating {
		if ses.State != SessionStateAuthenticating {
			return c.FailSession(ctx, &Reason{
				Code:        1,
				Description: "Invalid session state",
			})
		}

		if ses.ID != c.sessionID {
			return c.FailSession(ctx, &Reason{
				Code:        1,
				Description: "Invalid session id",
			})
		}
		if _, ok := schemeOptsMap[ses.Scheme]; !ok {
			return c.FailSession(ctx, &Reason{
				Code:        1,
				Description: "An invalid authentication scheme was selected",
			})
		}

		// Authenticate using the provided func
		authResult, err := authFunc(ses.From.Identity, ses.Authentication)
		if err != nil {
			return err
		}

		// If the auth result contains the identity domain role, it has succeeded
		if authResult.Role != "" && authResult.Role != DomainRoleUnknown {
			node, err := registerFunc(ses.From, c)
			if err != nil {
				return err
			}

			if err = c.sendEstablishedSession(ctx, node); err != nil {
				return err
			}
		} else if authResult.RoundTrip != nil {
			ses, err = c.sendAuthenticatingRoundTripSession(ctx, authResult.RoundTrip)
			if err != nil {
				return err
			}

		} else {
			if err = c.FailSession(ctx, &Reason{
				Code:        1,
				Description: "The session authentication failed",
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *ServerChannel) FinishSession(ctx context.Context) error {
	if err := c.ensureEstablished("send finished session"); err != nil {
		return err
	}

	ses := Session{
		EnvelopeBase: EnvelopeBase{
			ID:   c.sessionID,
			From: c.localNode,
			To:   c.remoteNode,
		},
		State: SessionStateFinished,
	}

	err := c.sendSession(ctx, &ses)

	c.setState(SessionStateFinished)

	if err == nil {
		if err = c.transport.Close(); err != nil {
			err = fmt.Errorf("closing the transport failed: %w", err)
		}
	}

	return err
}
func (c *ServerChannel) FailSession(ctx context.Context, reason *Reason) error {
	if err := c.ensureTransportOK("send failed session"); err != nil {
		return err
	}

	ses := Session{
		EnvelopeBase: EnvelopeBase{
			ID:   c.sessionID,
			From: c.localNode,
			To:   c.remoteNode,
		},
		State:  SessionStateFailed,
		Reason: reason,
	}
	err := c.sendSession(ctx, &ses)

	c.setState(SessionStateFailed)

	if err == nil {
		if err = c.transport.Close(); err != nil {
			err = fmt.Errorf("closing the transport failed: %w", err)
		}
	}

	return err
}

// Source: https://github.com/juliangruber/go-intersect
func intersect(a interface{}, b interface{}) []interface{} {
	set := make([]interface{}, 0)
	av := reflect.ValueOf(a)

	for i := 0; i < av.Len(); i++ {
		el := av.Index(i).Interface()
		if contains(b, el) {
			set = append(set, el)
		}
	}

	return set
}

func contains(a interface{}, e interface{}) bool {
	v := reflect.ValueOf(a)

	for i := 0; i < v.Len(); i++ {
		if v.Index(i).Interface() == e {
			return true
		}
	}
	return false
}
