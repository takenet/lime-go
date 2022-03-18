package main

import (
	"context"
	"github.com/takenet/lime-go"
	"go.uber.org/multierr"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
)

var channels = make(map[string]*lime.ServerChannel)
var nodesToID = make(map[string]string)
var mu sync.RWMutex

func main() {
	wsConfig := &lime.WebsocketConfig{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	server := lime.NewServerBuilder().
		Register(func(ctx context.Context, candidate lime.Node, c *lime.ServerChannel) (lime.Node, error) {
			mu.Lock()
			defer mu.Unlock()
			// Detect and resolve name collisions
			name := candidate.Name
			for {
				if _, ok := nodesToID[candidate.Name]; ok {
					candidate.Name = name + strconv.Itoa(rand.Intn(len(nodesToID)*10))
				} else {
					break
				}
			}

			// Register a new established user session
			sessionID := c.ID()
			channels[sessionID] = c
			nodesToID[candidate.Name] = sessionID
			return candidate, nil
		}).
		Finished(func(sessionID string) {
			mu.Lock()
			defer mu.Unlock()
			// Remove a finished session
			if channel, ok := channels[sessionID]; ok {
				delete(nodesToID, channel.RemoteNode().Name)
				delete(channels, sessionID)
			}
		}).
		MessagesHandlerFunc(HandleMessage).
		ListenWebsocket(&net.TCPAddr{Port: 8080}, wsConfig).
		Build()

	sig := make(chan os.Signal)

	go func() {
		if err := server.ListenAndServe(); err != lime.ErrServerClosed {
			log.Printf("server: listen: %v\n", err)
			close(sig)
		}
	}()

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
	if msg.To.Name != "" {
		if sessionID, ok := nodesToID[msg.To.Name]; ok {
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
