package http

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/takenet/lime-go"
)

func TestBuildBasicAuthorizationHeader(t *testing.T) {
	header := buildBasicAuthorizationHeader(lime.Identity{Name: "bot", Domain: "msging.net"}, "key123")

	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("bot@msging.net:key123")), header)
}

func TestParseAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name           string
		header         string
		wantIdentity   lime.Identity
		wantScheme     lime.AuthenticationScheme
		wantCredential string
		wantErr        error
	}{
		{
			name:           "basic scheme",
			header:         "Basic " + base64.StdEncoding.EncodeToString([]byte("bot@msging.net:secret")),
			wantIdentity:   lime.Identity{Name: "bot", Domain: "msging.net"},
			wantScheme:     lime.AuthenticationSchemePlain,
			wantCredential: "secret",
		},
		{
			name:           "key scheme",
			header:         "Key " + base64.StdEncoding.EncodeToString([]byte("bot@msging.net:accesskey")),
			wantIdentity:   lime.Identity{Name: "bot", Domain: "msging.net"},
			wantScheme:     lime.AuthenticationSchemeKey,
			wantCredential: "accesskey",
		},
		{
			name:    "empty header",
			header:  "",
			wantErr: ErrMissingAuthorizationHeader,
		},
		{
			name:    "missing scheme separator",
			header:  base64.StdEncoding.EncodeToString([]byte("bot@msging.net:secret")),
			wantErr: ErrInvalidAuthorizationHeader,
		},
		{
			name:    "invalid base64",
			header:  "Basic not-base64!!!",
			wantErr: ErrInvalidAuthorizationHeader,
		},
		{
			name:    "missing credential separator",
			header:  "Basic " + base64.StdEncoding.EncodeToString([]byte("bot@msging.net")),
			wantErr: ErrInvalidAuthorizationHeader,
		},
		{
			name:    "unsupported scheme",
			header:  "Bearer " + base64.StdEncoding.EncodeToString([]byte("bot@msging.net:secret")),
			wantErr: ErrInvalidAuthorizationHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, scheme, credential, err := parseAuthorizationHeader(tt.header)
			if tt.wantErr != nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantIdentity, identity)
			assert.Equal(t, tt.wantScheme, scheme)
			assert.Equal(t, tt.wantCredential, credential)
		})
	}
}
