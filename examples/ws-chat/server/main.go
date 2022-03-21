package main

import (
	"context"
	"fmt"
	"github.com/takenet/lime-go"
	"go.uber.org/multierr"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
)

var channels = make(map[string]*lime.ServerChannel)
var nodesToID = make(map[string]string)
var mu sync.RWMutex
var nodeFriends = make(map[string][]string)

func main() {
	server := lime.NewServerBuilder().
		// Handler for registering new user sessions
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
		// Callback for finished sessions, useful for updating our online users map
		Finished(func(sessionID string) {
			mu.Lock()
			defer mu.Unlock()
			// Remove a finished session
			if channel, ok := channels[sessionID]; ok {
				delete(nodesToID, channel.RemoteNode().Name)
				delete(channels, sessionID)
			}
		}).
		// Handler for all messages received by the server
		MessagesHandlerFunc(handleMessage).
		// Handler for commands with the "/friends" resource
		CommandHandlerFunc(
			func(cmd *lime.Command) bool {
				uri := cmd.URI.ToURL()
				return cmd.ID != "" && cmd.Status == "" && strings.HasPrefix(uri.Path, "/friends")
			},
			handleFriendsCommand).
		// Listen using the websocket transport in the 8080 port
		ListenWebsocket(
			&net.TCPAddr{Port: 8080},
			&lime.WebsocketConfig{
				CheckOrigin: func(r *http.Request) bool {
					return true
				}}).
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

func handleMessage(ctx context.Context, msg *lime.Message, s lime.Sender) error {
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

func handleFriendsCommand(ctx context.Context, cmd *lime.Command, s lime.Sender) error {
	node, _ := lime.ContextSessionRemoteNode(ctx)

	var respCmd *lime.Command
	switch cmd.Method {
	case lime.CommandMethodGet:
		respCmd = getFriends(node, cmd)
	case lime.CommandMethodSet:
		respCmd = addFriend(cmd, node)
	case lime.CommandMethodDelete:
		respCmd = removeFriend(cmd, node)
	default:
		respCmd = cmd.FailureResponse(&lime.Reason{
			Code:        1,
			Description: "Unsupported method",
		})
	}

	return s.SendCommand(ctx, respCmd)
}

func getFriends(node lime.Node, cmd *lime.Command) *lime.Command {
	var respCmd *lime.Command

	if friends, ok := nodeFriends[node.Name]; ok {
		items := make([]lime.Document, len(friends))
		for i, f := range friends {
			_, online := nodesToID[f]
			items[i] = &Friend{
				Nickname: f,
				Online:   online,
			}
		}

		respCmd = cmd.SuccessResponseWithResource(
			&lime.DocumentCollection{
				Total:    len(friends),
				ItemType: friendMediaType,
				Items:    items,
			})
	} else {
		respCmd = cmd.FailureResponse(&lime.Reason{
			Code:        1,
			Description: "No friends found",
		})
	}
	return respCmd
}

func addFriend(cmd *lime.Command, node lime.Node) *lime.Command {
	var respCmd *lime.Command

	if f, ok := cmd.Resource.(*Friend); ok {
		friends := nodeFriends[node.Name]
		friends = append(friends, f.Nickname)
		nodeFriends[node.Name] = friends
		respCmd = cmd.SuccessResponse()

	} else {
		respCmd = cmd.FailureResponse(&lime.Reason{
			Code:        1,
			Description: fmt.Sprintf("Unexpected resource type, should be '%v'", friendMediaType.String()),
		})
	}
	return respCmd
}

func removeFriend(cmd *lime.Command, node lime.Node) *lime.Command {
	var respCmd *lime.Command

	url := cmd.URI.ToURL()

	segments := strings.Split(url.Path, "/")
	if len(segments) >= 2 {
		friends := nodeFriends[node.Name]
		toRemove := -1

		for i, f := range friends {
			if string(f) == segments[1] {
				toRemove = i
				break
			}
		}

		if toRemove >= 0 {
			friends = append(friends[:toRemove], friends[toRemove+1:]...)
			nodeFriends[node.Name] = friends
			respCmd = cmd.SuccessResponse()
		} else {
			respCmd = cmd.FailureResponse(&lime.Reason{
				Code:        1,
				Description: fmt.Sprintf("Friend '%v' not found", segments[1]),
			})
		}
	} else {
		respCmd = cmd.FailureResponse(&lime.Reason{
			Code:        1,
			Description: "Invalid URI, should be '/friends/<nickname>'",
		})
	}

	return respCmd
}

var friendMediaType = lime.MediaType{
	Type:    "application",
	Subtype: "x-chat-friend",
	Suffix:  "json",
}

type Friend struct {
	Nickname string `json:"nickname"`
	Online   bool   `json:"online,omitempty"`
}

func (f *Friend) MediaType() lime.MediaType {
	return friendMediaType
}

func init() {
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Friend{}
	})
}
