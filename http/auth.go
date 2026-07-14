package http

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/takenet/lime-go"
)

// ErrMissingAuthorizationHeader is returned when an incoming request has no
// Authorization header.
var ErrMissingAuthorizationHeader = errors.New("http: missing authorization header")

// ErrInvalidAuthorizationHeader is returned when an incoming request has a
// malformed Authorization header.
var ErrInvalidAuthorizationHeader = errors.New("http: invalid authorization header")

// buildBasicAuthorizationHeader builds the value of the "Authorization" HTTP
// header used by the outbound Client, in the form "Basic
// base64(identity:credential)", matching Iris' expected authentication
// scheme for the "plain" (password) and "key" authentication schemes.
func buildBasicAuthorizationHeader(identity lime.Identity, credential string) string {
	raw := fmt.Sprintf("%s:%s", identity.String(), credential)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// Authenticator validates the credentials extracted from an incoming
// request's Authorization header. Implementations are expected to check the
// identity/credential pair against their own user base (e.g. an access key
// store) and return whether the identity is authorized.
type Authenticator interface {
	Authenticate(ctx context.Context, identity lime.Identity, scheme lime.AuthenticationScheme, credential string) (bool, error)
}

// AuthenticatorFunc is an adapter that allows the use of ordinary functions
// as an Authenticator.
type AuthenticatorFunc func(ctx context.Context, identity lime.Identity, scheme lime.AuthenticationScheme, credential string) (bool, error)

func (f AuthenticatorFunc) Authenticate(ctx context.Context, identity lime.Identity, scheme lime.AuthenticationScheme, credential string) (bool, error) {
	return f(ctx, identity, scheme, credential)
}

// parseAuthorizationHeader parses an "Authorization" header value in the
// "Basic base64(identity:credential)" or "Key base64(identity:key)" forms,
// mirroring the schemes accepted by lime-csharp's HttpMiddleware, and
// returns the extracted identity, authentication scheme and credential.
func parseAuthorizationHeader(header string) (identity lime.Identity, scheme lime.AuthenticationScheme, credential string, err error) {
	if header == "" {
		return lime.Identity{}, "", "", ErrMissingAuthorizationHeader
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return lime.Identity{}, "", "", ErrInvalidAuthorizationHeader
	}

	schemeName := strings.ToLower(parts[0])

	decoded, decErr := base64.StdEncoding.DecodeString(parts[1])
	if decErr != nil {
		return lime.Identity{}, "", "", fmt.Errorf("%w: %v", ErrInvalidAuthorizationHeader, decErr)
	}

	identityAndCredential := strings.SplitN(string(decoded), ":", 2)
	if len(identityAndCredential) != 2 {
		return lime.Identity{}, "", "", fmt.Errorf("%w: expected identity:credential", ErrInvalidAuthorizationHeader)
	}

	identity = lime.ParseIdentity(identityAndCredential[0])
	credential = identityAndCredential[1]

	switch schemeName {
	case "basic":
		scheme = lime.AuthenticationSchemePlain
	case "key":
		scheme = lime.AuthenticationSchemeKey
	default:
		return lime.Identity{}, "", "", fmt.Errorf("%w: unsupported scheme %q", ErrInvalidAuthorizationHeader, parts[0])
	}

	return identity, scheme, credential, nil
}
