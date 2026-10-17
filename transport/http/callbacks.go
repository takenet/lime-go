package httptransport

import (
	"net/http"
	"strings"
	"sync"
)

// CallbackMux routes Iris callbacks to the transport that owns the HTTP session.
type CallbackMux struct {
	mu         sync.RWMutex
	transports map[string]*ClientTransport
}

func NewCallbackMux() *CallbackMux {
	return &CallbackMux{transports: make(map[string]*ClientTransport)}
}

func (m *CallbackMux) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimRight(request.URL.Path, "/")
	callbackPath := strings.TrimSuffix(path, "/"+lastPathSegment(path))
	m.mu.RLock()
	client := m.transports[callbackPath]
	m.mu.RUnlock()
	if client == nil {
		http.NotFound(writer, request)
		return
	}
	client.ServeHTTP(writer, request)
}

func (m *CallbackMux) register(callbackPath string, client *ClientTransport) {
	if callbackPath == "" {
		return
	}
	m.mu.Lock()
	m.transports[callbackPath] = client
	m.mu.Unlock()
}

func (m *CallbackMux) unregister(callbackPath string, client *ClientTransport) {
	if callbackPath == "" {
		return
	}
	m.mu.Lock()
	if m.transports[callbackPath] == client {
		delete(m.transports, callbackPath)
	}
	m.mu.Unlock()
}
