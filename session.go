package main

import (
	"fmt"
)

// Defines the supported session states
type SessionState string

const (
	// The session is new and doesn't exists an established context.
	// It is sent by a client node to start a session with a server.
	SessionNew            = "new"
	SessionNegotiating    = "negotiating"
	SessionAuthenticating = "authenticating"
	SessionEstablished    = "established"
	SessionFinishing      = "finishing"
	SessionFinished       = "finished"
	SessionFailed         = "failed"
)

func (s SessionState) Validate() error {
	switch s {
	case SessionNew, SessionNegotiating, SessionAuthenticating, SessionEstablished, SessionFinishing, SessionFinished, SessionFailed:
		return nil
	}

	return fmt.Errorf("invalid session state '%v'", s)
}

func (s SessionState) MarshalText() ([]byte, error) {
	err := s.Validate()
	if err != nil {
		return []byte{}, err
	}
	return []byte(s), nil
}

func (s *SessionState) UnmarshalText(text []byte) error {
	state := SessionState(text)
	err := state.Validate()
	if err != nil {
		return err
	}
	*s = state
	return nil
}

// Allows the configuration and establishment of the communication channel between nodes.
type Session struct {
	Envelope
	State SessionState `json:"state"`
}
