/*
Package http implements the LIME-over-HTTP integration used by Blip's HTTP
gateway ("Iris", publicly exposed at https://http.msging.net).

Unlike the TCP and WebSocket transports in the parent lime package, LIME-over-HTTP
is not a persistent, session-negotiated transport. It is a stateless,
per-request protocol:

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

Because this model does not fit the session-oriented lime.Transport
abstraction (which assumes persistent connections and compression/encryption
negotiation), this package is self-contained and only depends on already
exported lime package types (lime.Message, lime.Notification,
lime.RequestCommand, lime.ResponseCommand, lime.Identity, lime.Node, etc.).

# Sending envelopes to Iris

	client := http.NewClient(identity, accessKey)
	err := client.SendMessage(ctx, (&lime.Message{}).
		SetToString("destination@messenger.gw.msging.net").
		SetContent(lime.TextDocument("Hello!")))

# Receiving webhooks from Iris

	handler := &http.Handler{
		LocalNode:     localNode,
		Authenticator: myAuthenticator,
		MessageHandler: func(ctx context.Context, msg *lime.Message) (*lime.Notification, error) {
			// process the message...
			return msg.Notification(lime.NotificationEventConsumed), nil
		},
	}
	stdhttp.ListenAndServe(":8080", handler)

See https://limeprotocol.org/ for the protocol specification and
https://docs.blip.ai/ for Blip/Iris specific integration details.
*/
package http
