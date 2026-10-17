package http

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lime "github.com/takenet/lime-go/protocol"
)

func TestCommandMuxHandle_FirstMatchingProcessorHandlesRequest(t *testing.T) {
	mux := &CommandMux{}
	firstCalled := false
	secondCalled := false

	err := mux.Register(func(cmd *lime.RequestCommand) bool {
		return cmd.Method == lime.CommandMethodGet && cmd.URI.Path() == "/schedules/123"
	}, func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
		firstCalled = true
		return cmd.SuccessResponse(), nil
	})
	require.NoError(t, err)
	err = mux.Register(nil, func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
		secondCalled = true
		return cmd.SuccessResponse(), nil
	})
	require.NoError(t, err)

	cmd := newMuxTestCommand(lime.CommandMethodGet, "/schedules/123")
	response, err := mux.Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, lime.CommandStatusSuccess, response.Status)
	assert.Equal(t, cmd.ID, response.ID)
	assert.True(t, firstCalled)
	assert.False(t, secondCalled)
}

func TestCommandMuxHandle_NoMatchReturnsProtocolFailure(t *testing.T) {
	mux := &CommandMux{}
	cmd := newMuxTestCommand(lime.CommandMethodGet, "/unknown")

	response, err := mux.Handle(context.Background(), cmd)

	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, lime.CommandStatusFailure, response.Status)
	assert.Equal(t, cmd.ID, response.ID)
	assert.Equal(t, cmd.Method, response.Method)
	require.NotNil(t, response.Reason)
	assert.Equal(t, lime.ReasonCodeCommandResourceNotSupported, response.Reason.Code)
}

func TestCommandMuxHandle_UnmatchedObserveWithoutIDHasNoResponse(t *testing.T) {
	cmd := newMuxTestCommand(lime.CommandMethodObserve, "/presence")
	cmd.ID = ""

	response, err := (&CommandMux{}).Handle(context.Background(), cmd)

	require.NoError(t, err)
	assert.Nil(t, response)
}

func TestCommandMuxHandle_ProcessorErrorIsWrapped(t *testing.T) {
	wantErr := errors.New("processor failed")
	mux := &CommandMux{}
	require.NoError(t, mux.Register(nil, func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
		return nil, wantErr
	}))

	_, err := mux.Handle(context.Background(), newMuxTestCommand(lime.CommandMethodGet, "/schedules"))

	require.ErrorIs(t, err, wantErr)
}

func TestCommandMuxRegister_NilHandlerReturnsError(t *testing.T) {
	require.Error(t, (&CommandMux{}).Register(nil, nil))
}

func newMuxTestCommand(method lime.CommandMethod, uri string) *lime.RequestCommand {
	cmd := &lime.RequestCommand{
		Command: lime.Command{
			Envelope: lime.Envelope{ID: "command-id"},
			Method:   method,
		},
	}
	cmd.SetURIString(uri)
	return cmd
}
