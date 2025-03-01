package types

import (
	"github.com/natansdj/lets"

	"google.golang.org/grpc"
)

// Default grpc server configuration
const (
	SERVER_GRPC_PORT = "5100"
)

// Interface for gRPC
type IGrpcServer interface {
	GetPort() string
	GetRouter() func(*grpc.Server)
	GetServerOptions() []grpc.ServerOption
}

// Server information
type GrpcServer struct {
	Port          string
	Router        func(*grpc.Server)
	ServerOptions []grpc.ServerOption
}

// Get Port
func (g *GrpcServer) GetPort() string {
	if g.Port == "" {
		lets.LogW("Config: SERVER_GRPC_PORT is not set, using default configuration.")

		return SERVER_GRPC_PORT
	}

	return g.Port
}

// Get Router
func (g *GrpcServer) GetRouter() func(*grpc.Server) {
	return g.Router
}

// Get Router
func (g *GrpcServer) GetServerOptions() []grpc.ServerOption {
	return g.ServerOptions
}
