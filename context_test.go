package lime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextKeyString(t *testing.T) {
	key := contextKey("test")
	result := key.String()
	assert.Equal(t, "lime:test", result)
}

func TestContextSessionID(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, contextKeySessionID, "session-123")

	sessionID, ok := ContextSessionID(ctx)
	assert.True(t, ok, "should find session ID in context")
	assert.Equal(t, "session-123", sessionID)
}

func TestContextSessionIDNotFound(t *testing.T) {
	ctx := context.Background()

	sessionID, ok := ContextSessionID(ctx)
	assert.False(t, ok, "should not find session ID in empty context")
	assert.Empty(t, sessionID)
}

func TestContextSessionRemoteNode(t *testing.T) {
	ctx := context.Background()
	remoteNode := Node{
		Identity: Identity{Name: "remote", Domain: "example.com"},
		Instance: "home",
	}
	ctx = context.WithValue(ctx, contextKeySessionRemoteNode, remoteNode)

	node, ok := ContextSessionRemoteNode(ctx)
	assert.True(t, ok, "should find remote node in context")
	assert.Equal(t, remoteNode, node)
}

func TestContextSessionRemoteNodeNotFound(t *testing.T) {
	ctx := context.Background()

	node, ok := ContextSessionRemoteNode(ctx)
	assert.False(t, ok, "should not find remote node in empty context")
	assert.Equal(t, Node{}, node)
}

func TestContextSessionLocalNode(t *testing.T) {
	ctx := context.Background()
	localNode := Node{
		Identity: Identity{Name: "local", Domain: "example.com"},
		Instance: "work",
	}
	ctx = context.WithValue(ctx, contextKeySessionLocalNode, localNode)

	node, ok := ContextSessionLocalNode(ctx)
	assert.True(t, ok, "should find local node in context")
	assert.Equal(t, localNode, node)
}

func TestContextSessionLocalNodeNotFound(t *testing.T) {
	ctx := context.Background()

	node, ok := ContextSessionLocalNode(ctx)
	assert.False(t, ok, "should not find local node in empty context")
	assert.Equal(t, Node{}, node)
}

func TestSessionContext(t *testing.T) {
	baseCtx := context.Background()

	ch := &channel{
		sessionID: "test-session",
		remoteNode: Node{
			Identity: Identity{Name: "remote", Domain: "example.com"},
		},
		localNode: Node{
			Identity: Identity{Name: "local", Domain: "example.com"},
		},
	}

	ctx := sessionContext(baseCtx, ch)

	// Verify all values were set
	sessionID, ok := ContextSessionID(ctx)
	assert.True(t, ok)
	assert.Equal(t, "test-session", sessionID)

	remoteNode, ok := ContextSessionRemoteNode(ctx)
	assert.True(t, ok)
	assert.Equal(t, ch.remoteNode, remoteNode)

	localNode, ok := ContextSessionLocalNode(ctx)
	assert.True(t, ok)
	assert.Equal(t, ch.localNode, localNode)
}
