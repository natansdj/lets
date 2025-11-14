package drivers

import (
	"context"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/circuitbreaker"
	"github.com/natansdj/lets/health"
	"github.com/natansdj/lets/metrics"
	"github.com/natansdj/lets/monitoring"
	"github.com/natansdj/lets/types"

	"github.com/go-redis/redis/v8"
)

var RedisConfig types.IRedis

type redisProvider struct {
	dsn            string
	username       string
	password       string
	database       int
	redis          *redis.Client
	config         types.IRedis
	circuitBreaker *circuitbreaker.CircuitBreaker
	metrics        *metrics.Metrics
	monitorDone    chan struct{}
}

func (m *redisProvider) Connect() {
	// Initialize metrics
	m.metrics = metrics.NewMetrics()

	// Initialize circuit breaker
	cbSettings := circuitbreaker.Settings{
		MaxRequests: 5,
		Interval:    60 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts circuitbreaker.Counts) bool {
			// Trip after 5 consecutive failures OR 50% failure rate with min 10 requests
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.ConsecutiveFailures > 5 ||
				(failureRate > 0.5 && counts.Requests >= 10)
		},
		OnStateChange: func(name string, from, to circuitbreaker.State) {
			lets.LogI("Redis circuit breaker: %s -> %s", from, to)
			m.metrics.UpdateHealthStatus(to.String())
		},
	}
	m.circuitBreaker = circuitbreaker.NewCircuitBreaker("redis", cbSettings)

	// Register with monitoring
	monitoring.RegisterService("redis", "database")

	// Start connection
	m.connectWithRetry(0)

	// Register health checker
	health.Register(&redisHealthChecker{driver: m})

	// Start periodic monitoring updates
	m.monitorDone = make(chan struct{})
	go m.periodicMonitoringUpdate()
}

func (m *redisProvider) connectWithRetry(attempt int) {
	maxRetries := m.config.GetMaxRetries()
	if attempt >= maxRetries {
		lets.LogF("Redis: Max connection retries (%d) exceeded", maxRetries)
		return
	}

	m.redis = redis.NewClient(&redis.Options{
		Addr:     m.dsn,
		Username: m.username,
		Password: m.password,
		DB:       m.database,

		// Connection pool settings
		PoolSize:     m.config.GetPoolSize(),
		MinIdleConns: m.config.GetMinIdleConns(),
		MaxRetries:   m.config.GetMaxRetries(),

		// Timeout settings
		DialTimeout:  time.Duration(m.config.GetDialTimeout()) * time.Second,
		ReadTimeout:  time.Duration(m.config.GetReadTimeout()) * time.Second,
		WriteTimeout: time.Duration(m.config.GetWriteTimeout()) * time.Second,
		PoolTimeout:  time.Duration(m.config.GetPoolTimeout()) * time.Second,

		// Health check on connect
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			return cn.Ping(ctx).Err()
		},
	})

	// Verify connection with ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.redis.Ping(ctx).Err(); err != nil {
		// Calculate exponential backoff
		backoff := time.Second * time.Duration(1<<uint(attempt))
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		lets.LogERL("redis-connect-retry", "Redis connection attempt %d failed: %v. Retrying in %v...", attempt+1, err, backoff)
		m.metrics.IncrementConnectionsFailed()
		m.metrics.RecordError(err)
		time.Sleep(backoff)
		m.connectWithRetry(attempt + 1)
		return
	}

	m.metrics.IncrementConnectionsTotal()
	m.metrics.UpdateHealthStatus("healthy")
	lets.LogI("Redis Client Connected (Pool: %d, MinIdle: %d)", m.config.GetPoolSize(), m.config.GetMinIdleConns())
}

func (m *redisProvider) Disconnect() {
	lets.LogI("Redis Stopping ...")

	// Stop monitoring
	if m.monitorDone != nil {
		close(m.monitorDone)
	}

	// Close Redis client
	err := m.redis.Close()
	if err != nil {
		lets.LogErr(err)
		return
	}

	// Update health status
	if m.metrics != nil {
		m.metrics.UpdateHealthStatus("disconnected")
	}

	lets.LogI("Redis Stopped ...")
}

// periodicMonitoringUpdate updates monitoring dashboard periodically
func (m *redisProvider) periodicMonitoringUpdate() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateMonitoring()
		case <-m.monitorDone:
			return
		}
	}
}

// updateMonitoring updates service metrics and stats
func (m *redisProvider) updateMonitoring() {
	if m.redis == nil {
		return
	}

	// Update service metrics
	monitoring.UpdateServiceMetrics("redis", m.metrics)

	// Update circuit breaker stats
	cbCounts := m.circuitBreaker.GetCounts()
	successRate := float64(0)
	if cbCounts.Requests > 0 {
		successRate = float64(cbCounts.TotalSuccesses) / float64(cbCounts.Requests) * 100
	}

	monitoring.UpdateCircuitBreakerStats("redis", monitoring.CircuitBreakerStats{
		State:                m.circuitBreaker.GetState().String(),
		Requests:             cbCounts.Requests,
		TotalSuccesses:       cbCounts.TotalSuccesses,
		TotalFailures:        cbCounts.TotalFailures,
		ConsecutiveSuccesses: cbCounts.ConsecutiveSuccesses,
		ConsecutiveFailures:  cbCounts.ConsecutiveFailures,
		SuccessRate:          successRate,
	})

	// Update pool stats from Redis client
	poolStats := m.redis.PoolStats()
	utilization := float64(0)
	poolSize := m.config.GetPoolSize()
	if poolSize > 0 {
		utilization = float64(poolStats.TotalConns) / float64(poolSize) * 100
	}

	monitoring.UpdatePoolStats("redis", monitoring.PoolStats{
		OpenConnections: int(poolStats.TotalConns),
		IdleConnections: int(poolStats.IdleConns),
		WaitingRequests: 0, // Redis client doesn't expose this
		MaxOpenConns:    poolSize,
		MaxIdleConns:    m.config.GetMinIdleConns(),
		Utilization:     utilization,
	})
}

// redisHealthChecker implements health.Checker for Redis
type redisHealthChecker struct {
	driver *redisProvider
}

func (c *redisHealthChecker) Name() string {
	return "redis"
}

func (c *redisHealthChecker) Check(ctx context.Context) health.CheckResult {
	if c.driver.redis == nil {
		return health.CheckResult{
			Status:    health.StatusUnhealthy,
			Message:   "Redis client not initialized",
			Timestamp: time.Now(),
		}
	}

	// Check circuit breaker state
	cbState := c.driver.circuitBreaker.GetState()
	if cbState == circuitbreaker.StateOpen {
		return health.CheckResult{
			Status:    health.StatusUnhealthy,
			Message:   "Redis circuit breaker is open",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"circuit_breaker_state": cbState.String(),
			},
		}
	}

	// Ping with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := c.driver.redis.Ping(pingCtx).Err()
	if err != nil {
		return health.CheckResult{
			Status:    health.StatusUnhealthy,
			Message:   "Redis ping failed",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	// Check metrics for degraded performance
	snapshot := c.driver.metrics.GetSnapshot()
	if snapshot.ErrorCount > 100 {
		return health.CheckResult{
			Status:    health.StatusDegraded,
			Message:   "High error count detected",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"error_count": snapshot.ErrorCount,
			},
		}
	}

	return health.CheckResult{
		Status:    health.StatusHealthy,
		Message:   "Redis is operational",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"circuit_breaker_state": cbState.String(),
			"pool_size":             c.driver.config.GetPoolSize(),
		},
	}
}

// Redis initializes and returns Redis client with connection pooling and retry logic
func Redis() (disconnectors []func()) {
	if RedisConfig == nil {
		return
	}

	lets.LogI("Redis Client Starting ...")

	redis := redisProvider{
		dsn:      RedisConfig.GetDsn(),
		username: RedisConfig.GetUsername(),
		password: RedisConfig.GetPassword(),
		database: RedisConfig.GetDatabase(),
		config:   RedisConfig,
	}
	redis.Connect()
	disconnectors = append(disconnectors, redis.Disconnect)

	// Inject Redis client into repository
	for _, repository := range RedisConfig.GetRepositories() {
		repository.SetDriver(redis.redis)
	}
	return
}
