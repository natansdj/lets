package types

import "google.golang.org/grpc"

// GRPC configuration interface
type IGrpcConfig interface {
	GetServer() IGrpcServer
	GetClients() []IGrpcClient
}

// GRPC configuration struct
type GrpcConfig struct {
	Server  IGrpcServer
	Clients []IGrpcClient
}

// Get gRPC server configuration
func (g *GrpcConfig) GetServer() IGrpcServer {
	return g.Server
}

// Get gRPC client configuration
func (g *GrpcConfig) GetClients() []IGrpcClient {
	return g.Clients
}

// Base marker interface for service clients; optional capabilities below
type IGrpcServiceClient interface{}

// Optional capability: single connection setter
type IGrpcServiceClientSingle interface {
	SetConnection(*grpc.ClientConn)
}

// Optional capability: receive full pool for per-request selection
type IGrpcServiceClientPool interface {
	SetConnectionPool([]*grpc.ClientConn)
}
