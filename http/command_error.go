package http

import (
	"errors"

	lime "github.com/takenet/lime-go/protocol"
)

// CommandError associates an application error with the LIME reason returned
// to the command caller.
type CommandError struct {
	Reason lime.Reason
	cause  error
}

// NewCommandError creates a command error that preserves cause for local error
// handling while exposing only reason in the LIME response.
func NewCommandError(reason lime.Reason, cause error) *CommandError {
	return &CommandError{Reason: reason, cause: cause}
}

func (e *CommandError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.Reason.Description
}

func (e *CommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func commandFailureResponse(cmd *lime.RequestCommand, err error) *lime.ResponseCommand {
	var commandErr *CommandError
	if errors.As(err, &commandErr) && commandErr != nil {
		return cmd.FailureResponse(&commandErr.Reason)
	}

	return cmd.FailureResponse(&lime.Reason{
		Code:        lime.ReasonCodeCommandProcessingError,
		Description: "The command could not be processed",
	})
}
