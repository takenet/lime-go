// Command client demonstrates sending LIME envelopes to Iris (Blip's HTTP
// gateway) using the github.com/takenet/lime-go/http package.
//
// Usage:
//
//	IRIS_IDENTITY=bot@msging.net IRIS_KEY=<access-key> go run ./examples/http-iris/client "destination@messenger.gw.msging.net" "Hello!"
package main

import (
	"context"
	"log"
	"os"
	"time"

	limehttp "github.com/takenet/lime-go/http"
	lime "github.com/takenet/lime-go/protocol"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("usage: %s <destination> <message>", os.Args[0])
	}

	identity := lime.ParseIdentity(envOrDefault("IRIS_IDENTITY", "bot@msging.net"))
	key := envOrDefault("IRIS_KEY", "")

	client := limehttp.NewClient(identity, key)

	msg := &lime.Message{
		Envelope: lime.Envelope{To: lime.ParseNode(os.Args[1])},
	}
	msg.SetContent(lime.TextDocument(os.Args[2]))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.SendMessage(ctx, msg); err != nil {
		log.Fatalf("send message: %v", err)
	}

	log.Println("Message sent to Iris successfully")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
