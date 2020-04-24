package main

// Defines the supported session states
type SessionState int

const (
	// The session is new and doesn't exists an established context.
	// It is sent by a client node to start a session with a server.
	SessionNew = iota
	SessionNegotiating
	SessionAuthenticating
	SessionEstablished
	SessionFinishing
	SessionFinished
	SessionFailed
)

var sessionStateToString = map[SessionState]string{
	SessionNew:            "new",
	SessionNegotiating:    "negotiating",
	SessionAuthenticating: "authenticating",
	SessionEstablished:    "established",
	SessionFinishing:      "finishing",
	SessionFinished:       "finished",
	SessionFailed:         "failed",
}

var sessionStateToID = map[string]SessionState{
	"new":            SessionNew,
	"negotiating":    SessionNegotiating,
	"authenticating": SessionAuthenticating,
	"established":    SessionEstablished,
	"finishing":      SessionFinishing,
	"finished":       SessionFinished,
	"failed":         SessionFailed,
}

func (s SessionState) MarshalText() ([]byte, error) {
	return []byte(sessionStateToString[s]), nil
}

func (s *SessionState) UnmarshalText(text []byte) error {
	state := string(text)
	*s = sessionStateToID[state]
	return nil
}

// Allows the configuration and establishment of the communication channel between nodes.
type Session struct {
	Envelope
	State SessionState `json:"state"`
}
