package health

import (
	"context"
	"sync"
	"time"
)

// Status represents health check status
type Status string

const (
	// StatusHealthy indicates service is healthy
	StatusHealthy Status = "healthy"
	
	// StatusDegraded indicates service is degraded but functional
	StatusDegraded Status = "degraded"
	
	// StatusUnhealthy indicates service is unhealthy
	StatusUnhealthy Status = "unhealthy"
	
	// StatusUnknown indicates health status is unknown
	StatusUnknown Status = "unknown"
)

// CheckResult represents result of a health check
type CheckResult struct {
	Status      Status                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Duration    time.Duration          `json:"duration"`
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Checker defines health check interface
type Checker interface {
	// Check performs health check
	Check(ctx context.Context) CheckResult
	
	// Name returns checker name
	Name() string
}

// Registry manages health checkers
type Registry struct {
	checkers map[string]Checker
	mu       sync.RWMutex
}

// NewRegistry creates a new health check registry
func NewRegistry() *Registry {
	return &Registry{
		checkers: make(map[string]Checker),
	}
}

// Register registers a health checker
func (r *Registry) Register(checker Checker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.checkers[checker.Name()] = checker
}

// Unregister removes a health checker
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.checkers, name)
}

// Check runs all health checks
func (r *Registry) Check(ctx context.Context) map[string]CheckResult {
	r.mu.RLock()
	checkers := make([]Checker, 0, len(r.checkers))
	for _, checker := range r.checkers {
		checkers = append(checkers, checker)
	}
	r.mu.RUnlock()
	
	results := make(map[string]CheckResult)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for _, checker := range checkers {
		wg.Add(1)
		go func(c Checker) {
			defer wg.Done()
			
			result := c.Check(ctx)
			
			mu.Lock()
			results[c.Name()] = result
			mu.Unlock()
		}(checker)
	}
	
	wg.Wait()
	
	return results
}

// CheckOne runs a specific health check
func (r *Registry) CheckOne(ctx context.Context, name string) (CheckResult, bool) {
	r.mu.RLock()
	checker, ok := r.checkers[name]
	r.mu.RUnlock()
	
	if !ok {
		return CheckResult{
			Status:    StatusUnknown,
			Timestamp: time.Now(),
			Message:   "checker not found",
		}, false
	}
	
	return checker.Check(ctx), true
}

// GetOverallStatus returns overall system health status
func (r *Registry) GetOverallStatus(ctx context.Context) Status {
	results := r.Check(ctx)
	
	if len(results) == 0 {
		return StatusUnknown
	}
	
	hasUnhealthy := false
	hasDegraded := false
	
	for _, result := range results {
		switch result.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}
	
	if hasUnhealthy {
		return StatusUnhealthy
	}
	
	if hasDegraded {
		return StatusDegraded
	}
	
	return StatusHealthy
}

// HealthReport represents overall health report
type HealthReport struct {
	Status    Status                  `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]CheckResult  `json:"checks"`
	Duration  time.Duration           `json:"duration"`
}

// GetHealthReport returns comprehensive health report
func (r *Registry) GetHealthReport(ctx context.Context) HealthReport {
	start := time.Now()
	
	checks := r.Check(ctx)
	overallStatus := StatusHealthy
	
	for _, check := range checks {
		if check.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
			break
		}
		if check.Status == StatusDegraded && overallStatus != StatusUnhealthy {
			overallStatus = StatusDegraded
		}
	}
	
	return HealthReport{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    checks,
		Duration:  time.Since(start),
	}
}

// Global health check registry
var globalRegistry = NewRegistry()

// Register registers a checker in global registry
func Register(checker Checker) {
	globalRegistry.Register(checker)
}

// Unregister removes a checker from global registry
func Unregister(name string) {
	globalRegistry.Unregister(name)
}

// Check runs all health checks in global registry
func Check(ctx context.Context) map[string]CheckResult {
	return globalRegistry.Check(ctx)
}

// CheckOne runs specific health check in global registry
func CheckOne(ctx context.Context, name string) (CheckResult, bool) {
	return globalRegistry.CheckOne(ctx, name)
}

// GetOverallStatus returns overall status from global registry
func GetOverallStatus(ctx context.Context) Status {
	return globalRegistry.GetOverallStatus(ctx)
}

// GetHealthReport returns health report from global registry
func GetHealthReport(ctx context.Context) HealthReport {
	return globalRegistry.GetHealthReport(ctx)
}
