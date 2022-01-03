package lime

import "context"

type contextKey string

func (c contextKey) String() string {
	return "lime:" + string(c)
}

var (
	contextKeySessionID         = contextKey("sessionID")
	contextKeySessionRemoteNode = contextKey("sessionRemoteNode")
	contextKeySessionLocalNode  = contextKey("sessionLocalNode")
)

func sessionContext(ctx context.Context, c *channel) context.Context {
	ctx = context.WithValue(ctx, contextKeySessionID, c.sessionID)
	ctx = context.WithValue(ctx, contextKeySessionRemoteNode, c.remoteNode)
	ctx = context.WithValue(ctx, contextKeySessionLocalNode, c.localNode)
	return ctx
}

// ContextSessionID gets the session id from the context.
func ContextSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(contextKeySessionID).(string)
	return sessionID, ok
}

// ContextSessionRemoteNode gets the session remote node from the context.
func ContextSessionRemoteNode(ctx context.Context) (Node, bool) {
	node, ok := ctx.Value(contextKeySessionRemoteNode).(Node)
	return node, ok
}

// ContextSessionLocalNode gets the session local node from the context.
func ContextSessionLocalNode(ctx context.Context) (Node, bool) {
	node, ok := ctx.Value(contextKeySessionLocalNode).(Node)
	return node, ok
}
