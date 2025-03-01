package frameworks

import (
	"fmt"
	"log"
	"net"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// gRPC framework configurations
var GrpcConfig types.IGrpcConfig

// gRPC Server
type grpcServer struct {
	dsn    string
	engine *grpc.Server
	router func(*grpc.Server)
}

// Internal function for initialize gRPC server
func (g *grpcServer) init(config types.IGrpcServer) {
	g.dsn = fmt.Sprintf(":%s", config.GetPort())
	g.engine = grpc.NewServer(config.GetServerOptions()...)
	g.router = config.GetRouter()
}

// Internal function for starting gRPC server
func (rpc *grpcServer) serve() {
	listener, err := net.Listen("tcp", rpc.dsn)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	go rpc.engine.Serve(listener)
}

func (rpc *grpcServer) Disconnect() {
	lets.LogI("GRPC server Stopping ...")

	rpc.engine.Stop()

	lets.LogI("GRPC server Stopped ...")
}

type grpcClient struct {
	name    string
	dsn     string
	options []grpc.DialOption
	engine  *grpc.ClientConn
}

func (rpc *grpcClient) init(config types.IGrpcClient) {
	rpc.name = config.GetName()
	rpc.dsn = fmt.Sprintf("%s:%s", config.GetHost(), config.GetPort())
	rpc.options = config.GetClientOptions()
}

func (rpc *grpcClient) connect() (err error) {
	opts := append(rpc.options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	rpc.engine, err = grpc.NewClient(rpc.dsn, opts...)
	return
}

func (rpc *grpcClient) Disconnect() {
	lets.LogI("GRPC Client Stopping ...")

	if err := rpc.engine.Close(); err != nil {
		lets.LogErr(err)
		return
	}

	lets.LogI("GRPC Client Stopped ...")
}

// Run gRPC server and client
func Grpc() (disconnectors []func()) {
	if GrpcConfig == nil {
		return
	}

	// Running gRPC server
	if config := GrpcConfig.GetServer(); GrpcConfig.GetServer() != nil {
		lets.LogI("gRPC Server Starting ...")

		var rpcServer grpcServer
		rpcServer.init(config)
		rpcServer.router(rpcServer.engine)
		rpcServer.serve()
		disconnectors = append(disconnectors, rpcServer.Disconnect)
	}

	// Running gRPC client
	if clients := GrpcConfig.GetClients(); len(clients) != 0 {
		lets.LogI("gRPC Client Starting ...")

		for _, config := range clients {
			var rpcClient grpcClient

			lets.LogI("gRPC Client: %s", config.GetName())
			rpcClient.init(config)

			if err := rpcClient.connect(); err != nil {
				lets.LogE("gRPC Client: %s", err.Error())
				continue
			}

			disconnectors = append(disconnectors, rpcClient.Disconnect)

			for _, isc := range config.GetClients() {
				isc.SetConnection(rpcClient.engine)
			}
		}
	}

	return
}
