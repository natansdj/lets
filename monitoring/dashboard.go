package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/natansdj/lets/health"
	"github.com/natansdj/lets/metrics"
)

// ServiceMetrics represents metrics for a specific service
type ServiceMetrics struct {
	Name             string                `json:"name"`
	Type             string                `json:"type"` // redis, mongodb, mysql, sqlite, http, grpc, etc.
	Status           health.Status         `json:"status"`
	Metrics          metrics.MetricsSnapshot `json:"metrics"`
	PoolStats        *PoolStats            `json:"pool_stats,omitempty"`
	CircuitBreaker   *CircuitBreakerStats  `json:"circuit_breaker,omitempty"`
	LastUpdated      time.Time             `json:"last_updated"`
}

// PoolStats represents connection pool statistics
type PoolStats struct {
	OpenConnections  int     `json:"open_connections"`
	IdleConnections  int     `json:"idle_connections"`
	WaitingRequests  int     `json:"waiting_requests"`
	MaxOpenConns     int     `json:"max_open_conns"`
	MaxIdleConns     int     `json:"max_idle_conns"`
	Utilization      float64 `json:"utilization"` // percentage
}

// CircuitBreakerStats represents circuit breaker statistics
type CircuitBreakerStats struct {
	State                string `json:"state"`
	Requests             uint32 `json:"requests"`
	TotalSuccesses       uint32 `json:"total_successes"`
	TotalFailures        uint32 `json:"total_failures"`
	ConsecutiveSuccesses uint32 `json:"consecutive_successes"`
	ConsecutiveFailures  uint32 `json:"consecutive_failures"`
	SuccessRate          float64 `json:"success_rate"` // percentage
}

// Dashboard aggregates metrics from all services
type Dashboard struct {
	services map[string]*ServiceMetrics
	mu       sync.RWMutex
}

// NewDashboard creates a new monitoring dashboard
func NewDashboard() *Dashboard {
	return &Dashboard{
		services: make(map[string]*ServiceMetrics),
	}
}

// RegisterService registers a service for monitoring
func (d *Dashboard) RegisterService(name, serviceType string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	d.services[name] = &ServiceMetrics{
		Name:        name,
		Type:        serviceType,
		Status:      health.StatusUnknown,
		LastUpdated: time.Now(),
	}
}

// UpdateServiceMetrics updates metrics for a service
func (d *Dashboard) UpdateServiceMetrics(name string, m *metrics.Metrics) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if service, ok := d.services[name]; ok {
		service.Metrics = m.GetSnapshot()
		service.Status = health.Status(m.GetHealthStatus())
		service.LastUpdated = time.Now()
	}
}

// UpdatePoolStats updates pool statistics for a service
func (d *Dashboard) UpdatePoolStats(name string, stats PoolStats) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if service, ok := d.services[name]; ok {
		service.PoolStats = &stats
		service.LastUpdated = time.Now()
	}
}

// UpdateCircuitBreakerStats updates circuit breaker stats for a service
func (d *Dashboard) UpdateCircuitBreakerStats(name string, stats CircuitBreakerStats) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if service, ok := d.services[name]; ok {
		service.CircuitBreaker = &stats
		service.LastUpdated = time.Now()
	}
}

// GetServiceMetrics returns metrics for a specific service
func (d *Dashboard) GetServiceMetrics(name string) (*ServiceMetrics, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	service, ok := d.services[name]
	return service, ok
}

// GetAllMetrics returns metrics for all services
func (d *Dashboard) GetAllMetrics() map[string]*ServiceMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	result := make(map[string]*ServiceMetrics, len(d.services))
	for name, service := range d.services {
		// Create copy
		serviceCopy := *service
		result[name] = &serviceCopy
	}
	
	return result
}

// GetDashboardSnapshot returns a complete dashboard snapshot
func (d *Dashboard) GetDashboardSnapshot() DashboardSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	services := make(map[string]*ServiceMetrics, len(d.services))
	for name, service := range d.services {
		serviceCopy := *service
		services[name] = &serviceCopy
	}
	
	// Calculate overall status
	overallStatus := health.StatusHealthy
	for _, service := range services {
		if service.Status == health.StatusUnhealthy {
			overallStatus = health.StatusUnhealthy
			break
		}
		if service.Status == health.StatusDegraded && overallStatus != health.StatusUnhealthy {
			overallStatus = health.StatusDegraded
		}
	}
	
	return DashboardSnapshot{
		Timestamp:     time.Now(),
		OverallStatus: overallStatus,
		Services:      services,
		Summary:       d.calculateSummary(services),
	}
}

// DashboardSnapshot represents a point-in-time dashboard snapshot
type DashboardSnapshot struct {
	Timestamp     time.Time                   `json:"timestamp"`
	OverallStatus health.Status               `json:"overall_status"`
	Services      map[string]*ServiceMetrics  `json:"services"`
	Summary       DashboardSummary            `json:"summary"`
}

// DashboardSummary provides aggregated statistics
type DashboardSummary struct {
	TotalServices        int     `json:"total_services"`
	HealthyServices      int     `json:"healthy_services"`
	DegradedServices     int     `json:"degraded_services"`
	UnhealthyServices    int     `json:"unhealthy_services"`
	TotalConnections     int64   `json:"total_connections"`
	TotalOperations      int64   `json:"total_operations"`
	TotalErrors          int64   `json:"total_errors"`
	AverageLatencyMs     int64   `json:"average_latency_ms"`
	OverallSuccessRate   float64 `json:"overall_success_rate"`
}

// calculateSummary calculates dashboard summary
func (d *Dashboard) calculateSummary(services map[string]*ServiceMetrics) DashboardSummary {
	summary := DashboardSummary{
		TotalServices: len(services),
	}
	
	var totalLatency int64
	var totalOps int64
	var totalSuccess int64
	
	for _, service := range services {
		switch service.Status {
		case health.StatusHealthy:
			summary.HealthyServices++
		case health.StatusDegraded:
			summary.DegradedServices++
		case health.StatusUnhealthy:
			summary.UnhealthyServices++
		}
		
		summary.TotalConnections += service.Metrics.ConnectionsActive
		summary.TotalOperations += service.Metrics.OperationsTotal
		summary.TotalErrors += service.Metrics.ErrorCount
		
		if service.Metrics.OperationsTotal > 0 {
			totalLatency += service.Metrics.AverageLatencyMs * service.Metrics.OperationsTotal
			totalOps += service.Metrics.OperationsTotal
			totalSuccess += service.Metrics.OperationsSuccess
		}
	}
	
	if totalOps > 0 {
		summary.AverageLatencyMs = totalLatency / totalOps
		summary.OverallSuccessRate = float64(totalSuccess) / float64(totalOps) * 100
	}
	
	return summary
}

// GetServicesByType returns all services of a specific type
func (d *Dashboard) GetServicesByType(serviceType string) []*ServiceMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	var result []*ServiceMetrics
	for _, service := range d.services {
		if service.Type == serviceType {
			serviceCopy := *service
			result = append(result, &serviceCopy)
		}
	}
	
	return result
}

// GetUnhealthyServices returns all unhealthy services
func (d *Dashboard) GetUnhealthyServices() []*ServiceMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	var result []*ServiceMetrics
	for _, service := range d.services {
		if service.Status == health.StatusUnhealthy {
			serviceCopy := *service
			result = append(result, &serviceCopy)
		}
	}
	
	return result
}

// HealthCheckAll performs health checks on all registered services
func (d *Dashboard) HealthCheckAll(ctx context.Context) map[string]health.CheckResult {
	return health.Check(ctx)
}

// StartPeriodicUpdate starts periodic metrics update
func (d *Dashboard) StartPeriodicUpdate(interval time.Duration) chan struct{} {
	done := make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Perform health checks
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = d.HealthCheckAll(ctx)
				cancel()
				
			case <-done:
				return
			}
		}
	}()
	
	return done
}

// Global dashboard instance
var globalDashboard = NewDashboard()

// RegisterService registers a service in global dashboard
func RegisterService(name, serviceType string) {
	globalDashboard.RegisterService(name, serviceType)
}

// UpdateServiceMetrics updates service metrics in global dashboard
func UpdateServiceMetrics(name string, m *metrics.Metrics) {
	globalDashboard.UpdateServiceMetrics(name, m)
}

// UpdatePoolStats updates pool stats in global dashboard
func UpdatePoolStats(name string, stats PoolStats) {
	globalDashboard.UpdatePoolStats(name, stats)
}

// UpdateCircuitBreakerStats updates circuit breaker stats in global dashboard
func UpdateCircuitBreakerStats(name string, stats CircuitBreakerStats) {
	globalDashboard.UpdateCircuitBreakerStats(name, stats)
}

// GetDashboardSnapshot returns snapshot from global dashboard
func GetDashboardSnapshot() DashboardSnapshot {
	return globalDashboard.GetDashboardSnapshot()
}

// GetServiceMetrics returns service metrics from global dashboard
func GetServiceMetrics(name string) (*ServiceMetrics, bool) {
	return globalDashboard.GetServiceMetrics(name)
}

// GetAllMetrics returns all metrics from global dashboard
func GetAllMetrics() map[string]*ServiceMetrics {
	return globalDashboard.GetAllMetrics()
}
