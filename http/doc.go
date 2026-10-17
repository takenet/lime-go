/*
Package http implements the LIME-over-HTTP integration used by Blip's HTTP
gateway ("Iris", publicly exposed at https://http.msging.net).

This package provides stateless gateway/webhook helpers. It is not a
`transport.Transport` implementation: each HTTP request carries one envelope,
while session-oriented HTTP lives in `transport/http`.

  - Outbound (client) role: an application authenticates every request with an
    "Authorization: Basic base64(identity:key)" header and POSTs a single
    envelope (message, notification or command) as a JSON body to the
    "/messages", "/notifications" or "/commands" endpoints. Commands receive a
    synchronous ResponseCommand in the HTTP response; messages and
    notifications receive an empty "202 Accepted".

  - Inbound (webhook/server) role: Iris forwards envelopes to a registered
    HTTPS endpoint via POST, using the same envelope JSON body and
    Basic/Key authentication. The receiving application must authenticate the
    request, dispatch the envelope to application handlers and, for commands,
    write a ResponseCommand back synchronously in the HTTP response.

# Sending envelopes to Iris

	client := http.NewClient(identity, accessKey)
	err := client.SendMessage(ctx, (&protocol.Message{}).
		SetToString("destination@messenger.gw.msging.net").
		SetContent(protocol.TextDocument("Hello!")))

# Receiving webhooks from Iris

	handler := &http.Handler{
		LocalNode:     localNode,
		Authenticator: myAuthenticator,
		MessageHandler: func(ctx context.Context, msg *protocol.Message) (*protocol.Notification, error) {
			// process the message...
			return msg.Notification(protocol.NotificationEventConsumed), nil
		},
	}
	stdhttp.ListenAndServe(":8080", handler)

# Routing commands

	commands := &http.CommandMux{}
	_ = commands.Register(func(cmd *protocol.RequestCommand) bool {
		return cmd.Method == protocol.CommandMethodGet && cmd.URI.Path() == "/healthz"
	}, func(ctx context.Context, cmd *protocol.RequestCommand) (*protocol.ResponseCommand, error) {
		return cmd.SuccessResponse(), nil
	})

	handler := &http.Handler{
		Authenticator:  myAuthenticator,
		CommandHandler: commands.Handle,
	}
	stdhttp.ListenAndServe(":8080", handler)

CommandMux evaluates processors in registration order. A request with no
matching processor receives a LIME failure response with reason code 62
(command resource not supported). An observe command without an ID is
fire-and-forget and is acknowledged with an empty "202 Accepted" response.
Command handler errors for correlated requests become a LIME failure with
reason code 61 by default. Return `http.NewCommandError` to provide a public
reason while retaining the underlying error for local handling.

See https://limeprotocol.org/ for the protocol specification and
https://docs.blip.ai/ for Blip/Iris specific integration details.
*/
package http
