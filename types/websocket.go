package types

import (
	"github.com/natansdj/lets"
)

// Default gRPC configuration
const (
	SERVER_WS_PORT = "5050"
	SERVER_WS_MODE = "debug"
)

// Interface for accessable method
type IWebSocketServer interface {
	GetPort() string
	GetMode() string
	GetRoutes() []IWebSocketRoute
}

// Serve information
type WebSocketServer struct {
	Port          string
	Mode          string
	Routes        []IWebSocketRoute
}

// Get Port
func (wss *WebSocketServer) GetPort() string {
	if wss.Port == "" {
		lets.LogW("Config: SERVER_WS_PORT is not set, using default configuration.")

		return SERVER_WS_PORT
	}

	return wss.Port
}

// Get Mode
func (wss *WebSocketServer) GetMode() string {
	if wss.Mode == "" {
		lets.LogW("Config: SERVER_WS_MODE is not set, using default configuration.")

		return SERVER_WS_MODE
	}

	return wss.Mode
}

// Get Router
func (wss *WebSocketServer) GetRoutes() []IWebSocketRoute {
	return wss.Routes
}
