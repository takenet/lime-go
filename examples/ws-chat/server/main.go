package main

import (
	"context"
	"github.com/takenet/lime-go"
	"go.uber.org/multierr"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
)

var channels = make(map[string]*lime.ServerChannel)
var nodesToID = make(map[lime.Node]string)
var mu sync.RWMutex

func main() {
	wsConfig := &lime.WebsocketConfig{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	server := lime.NewServerBuilder().
		Established(func(sessionID string, c *lime.ServerChannel) {
			mu.Lock()
			defer mu.Unlock()
			// Register a new established user session
			channels[sessionID] = c
			nodesToID[c.RemoteNode()] = sessionID
		}).
		Finished(func(sessionID string) {
			mu.Lock()
			defer mu.Unlock()
			// Remove a finished session
			if channel, ok := channels[sessionID]; ok {
				delete(nodesToID, channel.RemoteNode())
				delete(channels, sessionID)
			}
		}).
		MessagesHandlerFunc(HandleMessage).
		ListenWebsocket(&net.TCPAddr{Port: 8080}, wsConfig).
		Build()

	go func() {
		if err := server.ListenAndServe(); err != lime.ErrServerClosed {
			log.Printf("server: listen: %v\n", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	log.Println("Listening at ws:8080. Press Ctrl+C to stop.")
	<-sig

	if err := server.Close(); err != nil {
		log.Printf("server: close: %v\n", err)
	}
}

func HandleMessage(ctx context.Context, msg *lime.Message, s lime.Sender) error {
	mu.RLock()
	defer mu.RUnlock()

	var err error
	// Check if it is a direct message to another user
	if msg.To != (lime.Node{}) {
		if sessionID, ok := nodesToID[msg.To]; ok {
			if c, ok := channels[sessionID]; ok {
				err = c.SendMessage(ctx, msg)
			}
		}
	} else {
		// Broadcast the message to all others sessions
		senderSessionID, _ := lime.ContextSessionID(ctx)
		for id, c := range channels {
			if id != senderSessionID {
				err = multierr.Append(err, c.SendMessage(ctx, msg))
			}
		}
	}
	return err
}
