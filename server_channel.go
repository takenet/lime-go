package lime

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"
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
	if len(compOptions) == 0 {
		return nil, errors.New("no available options for compression negotiation")
	}
	if len(encryptOptions) == 0 {
		return nil, errors.New("no available options for encryption negotiation")
	}
	if err := c.ensureState(SessionStateNew, "negotiate session"); err != nil {
		return nil, err
	}

	c.setState(SessionStateNegotiating)

	ses := Session{
		Envelope: Envelope{
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
		Envelope: Envelope{
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
	if len(schemeOpts) == 0 {
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
		Envelope: Envelope{
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
		Envelope: Envelope{
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
		Envelope: Envelope{
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

const (
	errInvalidSessionID = "Invalid session id"
)

// AuthenticationResult represents the result of a session authentication.
type AuthenticationResult struct {
	Role      DomainRole
	RoundTrip Authentication
}

func UnknownAuthenticationResult() *AuthenticationResult {
	return &AuthenticationResult{Role: DomainRoleUnknown}
}

func MemberAuthenticationResult() *AuthenticationResult {
	return &AuthenticationResult{Role: DomainRoleMember}
}

func AuthorityAuthenticationResult() *AuthenticationResult {
	return &AuthenticationResult{Role: DomainRoleAuthority}
}

func RootAuthorityAuthenticationResult() *AuthenticationResult {
	return &AuthenticationResult{Role: DomainRoleRootAuthority}
}

// EstablishSession establishes a server channel with transport options negotiation and authentication.
func (c *ServerChannel) EstablishSession(
	ctx context.Context,
	compOpts []SessionCompression,
	encryptOpts []SessionEncryption,
	schemeOpts []AuthenticationScheme,
	authenticate func(context.Context, Identity, Authentication) (*AuthenticationResult, error),
	register func(context.Context, Node, *ServerChannel) (Node, error)) error {

	if err := c.validateEstablishSessionParams(compOpts, encryptOpts, authenticate, register); err != nil {
		return err
	}

	ses, err := c.receiveNewSession(ctx)
	if err != nil {
		slog.Error("Failed to receive new session", "error", err)
		return err
	}

	if err := c.validateSessionID(ctx, ses); err != nil {
		return err
	}

	if ses.State == SessionStateNew {
		if err := c.handleNewSession(ctx, compOpts, encryptOpts, schemeOpts, authenticate, register); err != nil {
			return err
		}
	}

	return c.finalizeEstablishment(ctx)
}

func (c *ServerChannel) validateEstablishSessionParams(
	compOpts []SessionCompression,
	encryptOpts []SessionEncryption,
	authenticate func(context.Context, Identity, Authentication) (*AuthenticationResult, error),
	register func(context.Context, Node, *ServerChannel) (Node, error)) error {

	if err := c.ensureTransportOK("establish session"); err != nil {
		slog.Error("Transport check failed", "error", err)
		return err
	}
	if compOpts == nil {
		panic("compOpts cannot be nil")
	}
	if encryptOpts == nil {
		panic("encryptOpts cannot be nil")
	}
	if authenticate == nil {
		panic("authenticate cannot be nil")
	}
	if register == nil {
		panic("register cannot be nil")
	}
	return nil
}

func (c *ServerChannel) validateSessionID(ctx context.Context, ses *Session) error {
	if ses.ID != "" {
		slog.Warn("Received session with existing ID", "session_id", ses.ID)
		return c.FailSession(ctx, &Reason{
			Code:        1,
			Description: errInvalidSessionID,
		})
	}
	return nil
}

func (c *ServerChannel) handleNewSession(
	ctx context.Context,
	compOpts []SessionCompression,
	encryptOpts []SessionEncryption,
	schemeOpts []AuthenticationScheme,
	authenticate func(context.Context, Identity, Authentication) (*AuthenticationResult, error),
	register func(context.Context, Node, *ServerChannel) (Node, error)) error {

	negCompOpts, negEncryptOpts := c.calculateNegotiationOptions(compOpts, encryptOpts)

	if len(negCompOpts) > 1 || len(negEncryptOpts) > 1 {
		if err := c.negotiateSession(ctx, negCompOpts, negEncryptOpts); err != nil {
			slog.Error("Session negotiation failed", "error", err)
			return err
		}
	}

	if c.state != SessionStateFailed {
		if err := c.authenticateSession(ctx, schemeOpts, authenticate, register); err != nil {
			slog.Error("Session authentication failed", "error", err)
			return err
		}
	}

	return nil
}

func (c *ServerChannel) calculateNegotiationOptions(
	compOpts []SessionCompression,
	encryptOpts []SessionEncryption) ([]SessionCompression, []SessionEncryption) {

	negCompOpts := make([]SessionCompression, 0)
	for v := range intersect(compOpts, c.transport.SupportedCompression()) {
		negCompOpts = append(negCompOpts, v)
	}

	if encryptOpts == nil {
		encryptOpts = []SessionEncryption{}
	}
	negEncryptOpts := make([]SessionEncryption, 0)
	for v := range intersect(encryptOpts, c.transport.SupportedEncryption()) {
		negEncryptOpts = append(negEncryptOpts, v)
	}

	return negCompOpts, negEncryptOpts
}

func (c *ServerChannel) finalizeEstablishment(ctx context.Context) error {
	if c.state != SessionStateEstablished && c.state != SessionStateFailed && c.transport.Connected() {
		slog.Warn("Session establishment incomplete, failing session")
		return c.FailSession(ctx, &Reason{
			Code:        1,
			Description: "The session establishment failed",
		})
	}

	slog.Info("Session established successfully", "session_id", c.sessionID)
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
			Description: errInvalidSessionID,
		})
	}

	if err := c.validateAndApplyNegotiationOptions(ctx, ses, compOpts, encryptOpts); err != nil {
		return err
	}

	return nil
}

func (c *ServerChannel) validateAndApplyNegotiationOptions(ctx context.Context, ses *Session, compOpts []SessionCompression, encryptOpts []SessionEncryption) error {
	if ses.State != SessionStateNegotiating || ses.Compression == "" || ses.Encryption == "" {
		return c.FailSession(ctx, &Reason{
			Code:        1,
			Description: "An invalid negotiation option was selected",
		})
	}

	if !c.isValidCompressionOption(ses.Compression, compOpts) || !c.isValidEncryptionOption(ses.Encryption, encryptOpts) {
		return c.FailSession(ctx, &Reason{
			Code:        1,
			Description: "An invalid negotiation option was selected",
		})
	}

	if err := c.sendNegotiatingConfirmationSession(ctx, ses.Compression, ses.Encryption); err != nil {
		return err
	}

	return c.applyTransportOptions(ctx, ses.Compression, ses.Encryption)
}

func (c *ServerChannel) isValidCompressionOption(compression SessionCompression, compOpts []SessionCompression) bool {
	return slices.Contains(compOpts, compression)
}

func (c *ServerChannel) isValidEncryptionOption(encryption SessionEncryption, encryptOpts []SessionEncryption) bool {
	return slices.Contains(encryptOpts, encryption)
}

func (c *ServerChannel) applyTransportOptions(ctx context.Context, compression SessionCompression, encryption SessionEncryption) error {
	if c.transport.Compression() != compression {
		if err := c.transport.SetCompression(ctx, compression); err != nil {
			return err
		}
	}

	if c.transport.Encryption() != encryption {
		if err := c.transport.SetEncryption(ctx, encryption); err != nil {
			return err
		}
	}

	return nil
}

func (c *ServerChannel) authenticateSession(
	ctx context.Context,
	schemeOpts []AuthenticationScheme,
	authenticate func(context.Context, Identity, Authentication) (*AuthenticationResult, error),
	register func(context.Context, Node, *ServerChannel) (Node, error)) error {

	schemeOptsMap := c.buildSchemeOptionsMap(schemeOpts)

	ses, err := c.sendAuthenticatingSession(ctx, schemeOpts)
	if err != nil {
		return err
	}

	for c.state == SessionStateAuthenticating {
		if err := c.validateAuthenticatingSession(ctx, ses, schemeOptsMap); err != nil {
			return err
		}

		authResult, err := authenticate(ctx, ses.From.Identity, ses.Authentication)
		if err != nil {
			return err
		}

		ses, err = c.processAuthenticationResult(ctx, authResult, ses, register)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ServerChannel) buildSchemeOptionsMap(schemeOpts []AuthenticationScheme) map[AuthenticationScheme]struct{} {
	schemeOptsMap := make(map[AuthenticationScheme]struct{})
	for _, v := range schemeOpts {
		schemeOptsMap[v] = struct{}{}
	}
	return schemeOptsMap
}

func (c *ServerChannel) validateAuthenticatingSession(ctx context.Context, ses *Session, schemeOptsMap map[AuthenticationScheme]struct{}) error {
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

	return nil
}

func (c *ServerChannel) processAuthenticationResult(
	ctx context.Context,
	authResult *AuthenticationResult,
	ses *Session,
	register func(context.Context, Node, *ServerChannel) (Node, error)) (*Session, error) {

	if authResult.Role != "" && authResult.Role != DomainRoleUnknown {
		return nil, c.handleSuccessfulAuthentication(ctx, ses, register)
	}

	if authResult.RoundTrip != nil {
		return c.sendAuthenticatingRoundTripSession(ctx, authResult.RoundTrip)
	}

	return nil, c.FailSession(ctx, &Reason{
		Code:        1,
		Description: "The session authentication failed",
	})
}

func (c *ServerChannel) handleSuccessfulAuthentication(
	ctx context.Context,
	ses *Session,
	register func(context.Context, Node, *ServerChannel) (Node, error)) error {

	node, err := register(ctx, ses.From, c)
	if err != nil {
		return err
	}

	return c.sendEstablishedSession(ctx, node)
}

func (c *ServerChannel) FinishSession(ctx context.Context) error {
	if err := c.ensureEstablished("send finished session"); err != nil {
		return err
	}

	ses := Session{
		Envelope: Envelope{
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
		Envelope: Envelope{
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

// intersect returns an iterator over the intersection of two slices.
// Uses Go 1.25 Generics and Iterators (range-over-func).
func intersect[T comparable](a, b []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range a {
			if contains(b, v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

func contains[T comparable](slice []T, e T) bool {
	return slices.Contains(slice, e)
}
