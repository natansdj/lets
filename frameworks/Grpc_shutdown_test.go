package frameworks

import (
	"testing"
	"time"
)

// TestGrpcClientDisconnectStopsMonitor is a regression test for the shutdown data
// race in which Disconnect wrote rpc.stopMonitor = nil (and tore down rpc.engines)
// while the monitorPool goroutine was still reading/recreating those same fields.
//
// Run with -race: on the buggy version the monitor churns rpc.engines while
// Disconnect nils it, which the race detector flags. On the fixed version
// Disconnect signals the monitor and blocks on monitorDone until it has exited,
// so there is no concurrent access and monitorDone is closed by the time
// Disconnect returns.
func TestGrpcClientDisconnectStopsMonitor(t *testing.T) {
	// 1s ticker so the monitor actively iterates and recreates rpc.engines during the test.
	t.Setenv("GRPC_HEALTHCHECK_INTERVAL", "1")

	rpc := &grpcClient{
		name:    "test",
		dsn:     "127.0.0.1:1", // nothing listening -> health checks fail fast, monitor recreates conns
		maxPool: 3,
	}
	if err := rpc.connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Let the monitor tick at least once so it is mid-flight with rpc.engines
	// concurrently with the Disconnect below.
	time.Sleep(1300 * time.Millisecond)

	rpc.Disconnect()

	// Disconnect must have waited for the monitor to fully exit before returning.
	select {
	case <-rpc.monitorDone:
		// monitor exited as expected
	default:
		t.Fatal("monitor goroutine still running after Disconnect returned (Disconnect did not wait for it)")
	}
}
