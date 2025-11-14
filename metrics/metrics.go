package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics represents a collection of operational metrics
type Metrics struct {
	// Connection metrics
	ConnectionsActive int64
	ConnectionsTotal  int64
	ConnectionsFailed int64
	ConnectionsIdle   int64

	// Operation metrics
	OperationsTotal   int64
	OperationsSuccess int64
	OperationsFailed  int64

	// Performance metrics
	TotalLatencyMs int64
	MinLatencyMs   int64
	MaxLatencyMs   int64

	// Health status
	LastHealthCheck time.Time
	HealthStatus    string
	ErrorCount      int64
	LastError       string
	LastErrorTime   time.Time

	mu sync.RWMutex
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		HealthStatus:    "unknown",
		LastHealthCheck: time.Now(),
		MinLatencyMs:    999999,
	}
}

// IncrementConnectionsActive increments active connections counter
func (m *Metrics) IncrementConnectionsActive() {
	atomic.AddInt64(&m.ConnectionsActive, 1)
}

// DecrementConnectionsActive decrements active connections counter
func (m *Metrics) DecrementConnectionsActive() {
	atomic.AddInt64(&m.ConnectionsActive, -1)
}

// IncrementConnectionsTotal increments total connections counter
func (m *Metrics) IncrementConnectionsTotal() {
	atomic.AddInt64(&m.ConnectionsTotal, 1)
}

// IncrementConnectionsFailed increments failed connections counter
func (m *Metrics) IncrementConnectionsFailed() {
	atomic.AddInt64(&m.ConnectionsFailed, 1)
}

// SetConnectionsIdle sets idle connections count
func (m *Metrics) SetConnectionsIdle(count int64) {
	atomic.StoreInt64(&m.ConnectionsIdle, count)
}

// IncrementOperationsTotal increments total operations counter
func (m *Metrics) IncrementOperationsTotal() {
	atomic.AddInt64(&m.OperationsTotal, 1)
}

// IncrementOperationsSuccess increments successful operations counter
func (m *Metrics) IncrementOperationsSuccess() {
	atomic.AddInt64(&m.OperationsSuccess, 1)
}

// IncrementOperationsFailed increments failed operations counter
func (m *Metrics) IncrementOperationsFailed() {
	atomic.AddInt64(&m.OperationsFailed, 1)
}

// RecordLatency records operation latency
func (m *Metrics) RecordLatency(latencyMs int64) {
	atomic.AddInt64(&m.TotalLatencyMs, latencyMs)

	// Update min latency
	for {
		currentMin := atomic.LoadInt64(&m.MinLatencyMs)
		if latencyMs >= currentMin {
			break
		}
		if atomic.CompareAndSwapInt64(&m.MinLatencyMs, currentMin, latencyMs) {
			break
		}
	}

	// Update max latency
	for {
		currentMax := atomic.LoadInt64(&m.MaxLatencyMs)
		if latencyMs <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.MaxLatencyMs, currentMax, latencyMs) {
			break
		}
	}
}

// RecordError records an error
func (m *Metrics) RecordError(err error) {
	atomic.AddInt64(&m.ErrorCount, 1)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.LastError = err.Error()
	m.LastErrorTime = time.Now()
}

// UpdateHealthStatus updates health status
func (m *Metrics) UpdateHealthStatus(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.HealthStatus = status
	m.LastHealthCheck = time.Now()
}

// GetHealthStatus returns current health status
func (m *Metrics) GetHealthStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.HealthStatus
}

// GetAverageLatencyMs returns average latency in milliseconds
func (m *Metrics) GetAverageLatencyMs() int64 {
	total := atomic.LoadInt64(&m.TotalLatencyMs)
	count := atomic.LoadInt64(&m.OperationsTotal)

	if count == 0 {
		return 0
	}

	return total / count
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MetricsSnapshot{
		ConnectionsActive: atomic.LoadInt64(&m.ConnectionsActive),
		ConnectionsTotal:  atomic.LoadInt64(&m.ConnectionsTotal),
		ConnectionsFailed: atomic.LoadInt64(&m.ConnectionsFailed),
		ConnectionsIdle:   atomic.LoadInt64(&m.ConnectionsIdle),
		OperationsTotal:   atomic.LoadInt64(&m.OperationsTotal),
		OperationsSuccess: atomic.LoadInt64(&m.OperationsSuccess),
		OperationsFailed:  atomic.LoadInt64(&m.OperationsFailed),
		AverageLatencyMs:  m.GetAverageLatencyMs(),
		MinLatencyMs:      atomic.LoadInt64(&m.MinLatencyMs),
		MaxLatencyMs:      atomic.LoadInt64(&m.MaxLatencyMs),
		HealthStatus:      m.HealthStatus,
		LastHealthCheck:   m.LastHealthCheck,
		ErrorCount:        atomic.LoadInt64(&m.ErrorCount),
		LastError:         m.LastError,
		LastErrorTime:     m.LastErrorTime,
	}
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	ConnectionsActive int64     `json:"connections_active"`
	ConnectionsTotal  int64     `json:"connections_total"`
	ConnectionsFailed int64     `json:"connections_failed"`
	ConnectionsIdle   int64     `json:"connections_idle"`
	OperationsTotal   int64     `json:"operations_total"`
	OperationsSuccess int64     `json:"operations_success"`
	OperationsFailed  int64     `json:"operations_failed"`
	AverageLatencyMs  int64     `json:"average_latency_ms"`
	MinLatencyMs      int64     `json:"min_latency_ms"`
	MaxLatencyMs      int64     `json:"max_latency_ms"`
	HealthStatus      string    `json:"health_status"`
	LastHealthCheck   time.Time `json:"last_health_check"`
	ErrorCount        int64     `json:"error_count"`
	LastError         string    `json:"last_error"`
	LastErrorTime     time.Time `json:"last_error_time"`
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	atomic.StoreInt64(&m.ConnectionsActive, 0)
	atomic.StoreInt64(&m.ConnectionsTotal, 0)
	atomic.StoreInt64(&m.ConnectionsFailed, 0)
	atomic.StoreInt64(&m.ConnectionsIdle, 0)
	atomic.StoreInt64(&m.OperationsTotal, 0)
	atomic.StoreInt64(&m.OperationsSuccess, 0)
	atomic.StoreInt64(&m.OperationsFailed, 0)
	atomic.StoreInt64(&m.TotalLatencyMs, 0)
	atomic.StoreInt64(&m.MinLatencyMs, 999999)
	atomic.StoreInt64(&m.MaxLatencyMs, 0)
	atomic.StoreInt64(&m.ErrorCount, 0)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.HealthStatus = "unknown"
	m.LastHealthCheck = time.Now()
	m.LastError = ""
	m.LastErrorTime = time.Time{}
}
