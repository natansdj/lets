package frameworks

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
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
	name        string
	dsn         string
	options     []grpc.DialOption
	dialOptions []grpc.DialOption
	engine      *grpc.ClientConn
	engines     []*grpc.ClientConn
	maxPool     int
	mu          sync.Mutex
	stopMonitor chan struct{}
	tfSince     map[int]time.Time
}

func (rpc *grpcClient) init(config types.IGrpcClient) {
	rpc.name = config.GetName()
	rpc.dsn = fmt.Sprintf("%s:%s", config.GetHost(), config.GetPort())
	rpc.options = config.GetClientOptions()
	rpc.maxPool = config.GetMaxPoolConnectionsPerAddress()
}

func (rpc *grpcClient) connect() (err error) {
	opts := append(rpc.options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	rpc.dialOptions = opts

	poolSize := rpc.maxPool
	if poolSize <= 0 {
		poolSize = 1
	}

	rpc.engines = make([]*grpc.ClientConn, 0, poolSize)
	var lastErr error
	for i := 0; i < poolSize; i++ {
		conn, connErr := grpc.NewClient(rpc.dsn, opts...)
		if connErr != nil {
			lastErr = connErr
			continue
		}
		rpc.engines = append(rpc.engines, conn)
	}

	if len(rpc.engines) == 0 {
		// Fallback to single connection when pool creation failed entirely
		rpc.engine, err = grpc.NewClient(rpc.dsn, opts...)
		return
	}

	// Keep first as primary for backward compatibility
	rpc.engine = rpc.engines[0]
	if lastErr != nil && len(rpc.engines) < poolSize {
		lets.LogW("gRPC Client: some connections failed to initialize, using %d/%d pool", len(rpc.engines), poolSize)
	}

	// Start background monitor to keep the pool healthy
	rpc.startMonitor()
	return
}

func (rpc *grpcClient) Disconnect() {
	lets.LogI("GRPC Client Stopping ...")

	// Stop monitor
	if rpc.stopMonitor != nil {
		close(rpc.stopMonitor)
		rpc.stopMonitor = nil
	}

	if len(rpc.engines) > 0 {
		for _, conn := range rpc.engines {
			if conn != nil {
				if err := conn.Close(); err != nil {
					lets.LogErr(err)
				}
			}
		}
		rpc.engines = nil
	} else if rpc.engine != nil {
		if err := rpc.engine.Close(); err != nil {
			lets.LogErr(err)
			return
		}
	}

	lets.LogI("GRPC Client Stopped ...")
}

// Run gRPC server and client
// If you want to use connection pool, make sure to set MaxPoolConnections > 0
// Monitoring interval is set via GRPC_HEALTHCHECK_INTERVAL environment variable in seconds with default is 120s
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

			serviceClients := config.GetClients()
			if len(rpcClient.engines) > 0 {
				// Provide full pool only to clients that support pool capability
				for _, isc := range serviceClients {
					if scPool, ok := isc.(types.IGrpcServiceClientPool); ok {
						scPool.SetConnectionPool(rpcClient.engines)
					} else if scSingle, ok := isc.(types.IGrpcServiceClientSingle); ok {
						scSingle.SetConnection(rpcClient.engine)
					}
				}
			} else {
				for _, isc := range serviceClients {
					if scSingle, ok := isc.(types.IGrpcServiceClientSingle); ok {
						scSingle.SetConnection(rpcClient.engine)
					}
				}
			}
		}
	}

	return
}

// startMonitor launches a goroutine to monitor and heal the connection pool
func (rpc *grpcClient) startMonitor() {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()
	if rpc.stopMonitor != nil {
		return
	}
	rpc.stopMonitor = make(chan struct{})
	rpc.tfSince = make(map[int]time.Time)
	go rpc.monitorPool()
}

// monitorPool checks connection states periodically and recreates unhealthy ones
func (rpc *grpcClient) monitorPool() {
	healthGrpcInterval := os.Getenv("GRPC_HEALTHCHECK_INTERVAL")
	if healthGrpcInterval == "" {
		healthGrpcInterval = "120"
	}

	interval, err := strconv.Atoi(healthGrpcInterval)
	if err != nil {
		interval = 120
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Aggregate pool state counts
			readyCount := 0
			idleCount := 0
			connectingCount := 0
			failedCount := 0

			for i, conn := range rpc.engines {
				state := connectivity.Shutdown
				if conn != nil {
					state = conn.GetState()
				}

				// Update counters
				switch state {
				case connectivity.Ready:
					readyCount++
				case connectivity.Idle:
					idleCount++
				case connectivity.Connecting:
					connectingCount++
				case connectivity.TransientFailure, connectivity.Shutdown:
					failedCount++
				}

				// Immediate remediation for nil or Shutdown
				if conn == nil || state == connectivity.Shutdown {
					newConn, err := grpc.NewClient(rpc.dsn, rpc.dialOptions...)
					if err != nil {
						lets.LogW("gRPC Client: failed to recreate connection %d: %v", i, err)
						continue
					}
					if conn != nil {
						_ = conn.Close()
					}
					rpc.engines[i] = newConn
					delete(rpc.tfSince, i)
					lets.LogI("gRPC Client: recreated connection %d", i)
					continue
				}

				// Health check for non-ready states; recreate if not serving
				if state != connectivity.Ready {
					healthClient := grpc_health_v1.NewHealthClient(conn)
					resp, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
					if err != nil || resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
						lets.LogD("gRPC ERR health check target=%s idx=%d err=%v", rpc.dsn, i, err)
						newConn, err := grpc.NewClient(rpc.dsn, rpc.dialOptions...)
						if err != nil {
							lets.LogW("gRPC Client: failed to recreate connection %d after health check: %v", i, err)
							continue
						}
						_ = conn.Close()
						rpc.engines[i] = newConn
						delete(rpc.tfSince, i)
						lets.LogI("gRPC Client: recreated connection %d after failed health check", i)
						continue
					}

					// Track transient failure duration if still in that state
					if state == connectivity.TransientFailure {
						since, ok := rpc.tfSince[i]
						if !ok {
							rpc.tfSince[i] = time.Now()
						} else if time.Since(since) > 30*time.Second {
							newConn, err := grpc.NewClient(rpc.dsn, rpc.dialOptions...)
							if err != nil {
								lets.LogW("gRPC Client: failed to heal transient failure %d: %v", i, err)
							} else {
								_ = conn.Close()
								rpc.engines[i] = newConn
								delete(rpc.tfSince, i)
								lets.LogI("gRPC Client: healed transient failure by recreating connection %d", i)
							}
						}
					} else {
						// Clear transient-failure tracking when healthy or other states
						delete(rpc.tfSince, i)
					}
				} else {
					// Clear transient-failure tracking when ready
					delete(rpc.tfSince, i)
				}
			}

			// Log aggregated pool health for this DSN
			lets.LogI(
				"gRPC health check target=%s: ready=%d, idle=%d, connecting=%d, failed=%d, total=%d, maxConn=%d",
				rpc.dsn, readyCount, idleCount, connectingCount, failedCount, len(rpc.engines), rpc.maxPool,
			)
		case <-rpc.stopMonitor:
			return
		}
	}
}
