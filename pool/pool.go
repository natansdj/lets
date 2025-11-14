package pool

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/natansdj/lets/metrics"
)

var (
	// ErrPoolClosed is returned when pool is closed
	ErrPoolClosed = errors.New("connection pool is closed")
	
	// ErrPoolExhausted is returned when pool has no available connections
	ErrPoolExhausted = errors.New("connection pool exhausted")
	
	// ErrConnectionInvalid is returned when connection is invalid
	ErrConnectionInvalid = errors.New("connection is invalid")
)

// Connection represents a pooled connection
type Connection interface {
	// Close closes the connection
	Close() error
	
	// IsValid checks if connection is valid
	IsValid() bool
	
	// Reset resets connection state for reuse
	Reset() error
}

// Factory creates new connections
type Factory interface {
	// Create creates a new connection
	Create() (Connection, error)
	
	// Validate checks if connection is valid
	Validate(conn Connection) bool
}

// Config configures connection pool
type Config struct {
	// MaxIdle is maximum number of idle connections
	MaxIdle int
	
	// MaxOpen is maximum number of open connections
	MaxOpen int
	
	// MaxLifetime is maximum connection lifetime
	MaxLifetime time.Duration
	
	// MaxIdleTime is maximum idle time before closing
	MaxIdleTime time.Duration
	
	// WaitTimeout is maximum wait time for connection
	WaitTimeout time.Duration
}

// DefaultConfig returns default pool configuration
func DefaultConfig() Config {
	return Config{
		MaxIdle:     10,
		MaxOpen:     100,
		MaxLifetime: 3 * time.Minute,
		MaxIdleTime: 1 * time.Minute,
		WaitTimeout: 5 * time.Second,
	}
}

// connWrapper wraps a connection with metadata
type connWrapper struct {
	conn       Connection
	createdAt  time.Time
	lastUsedAt time.Time
	usedCount  int64
}

// Pool is a generic connection pool
type Pool struct {
	factory Factory
	config  Config
	metrics *metrics.Metrics
	
	mu       sync.Mutex
	conns    []*connWrapper
	numOpen  int
	closed   bool
	waiters  []chan *connWrapper
}

// NewPool creates a new connection pool
func NewPool(factory Factory, config Config) *Pool {
	return &Pool{
		factory: factory,
		config:  config,
		metrics: metrics.NewMetrics(),
		conns:   make([]*connWrapper, 0, config.MaxIdle),
		waiters: make([]chan *connWrapper, 0),
	}
}

// Get retrieves a connection from pool
func (p *Pool) Get(ctx context.Context) (Connection, error) {
	p.mu.Lock()
	
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	
	// Try to get idle connection
	for len(p.conns) > 0 {
		wrapper := p.conns[0]
		p.conns = p.conns[1:]
		
		p.mu.Unlock()
		
		// Validate connection
		if p.isValidWrapper(wrapper) {
			wrapper.lastUsedAt = time.Now()
			wrapper.usedCount++
			p.metrics.IncrementOperationsTotal()
			return wrapper.conn, nil
		}
		
		// Connection invalid, close it
		wrapper.conn.Close()
		p.metrics.IncrementOperationsFailed()
		
		p.mu.Lock()
		p.numOpen--
		continue
	}
	
	// Try to create new connection
	if p.numOpen < p.config.MaxOpen {
		p.numOpen++
		p.mu.Unlock()
		
		conn, err := p.createConnection()
		if err != nil {
			p.mu.Lock()
			p.numOpen--
			p.mu.Unlock()
			return nil, err
		}
		
		p.metrics.IncrementOperationsSuccess()
		return conn, nil
	}
	
	// Pool exhausted, wait for connection
	if p.config.WaitTimeout == 0 {
		p.mu.Unlock()
		return nil, ErrPoolExhausted
	}
	
	waiter := make(chan *connWrapper, 1)
	p.waiters = append(p.waiters, waiter)
	p.mu.Unlock()
	
	select {
	case wrapper := <-waiter:
		if wrapper == nil {
			return nil, ErrPoolClosed
		}
		wrapper.lastUsedAt = time.Now()
		wrapper.usedCount++
		p.metrics.IncrementOperationsTotal()
		return wrapper.conn, nil
		
	case <-time.After(p.config.WaitTimeout):
		p.mu.Lock()
		// Remove waiter from list
		for i, w := range p.waiters {
			if w == waiter {
				p.waiters = append(p.waiters[:i], p.waiters[i+1:]...)
				break
			}
		}
		p.mu.Unlock()
		return nil, ErrPoolExhausted
		
	case <-ctx.Done():
		p.mu.Lock()
		// Remove waiter from list
		for i, w := range p.waiters {
			if w == waiter {
				p.waiters = append(p.waiters[:i], p.waiters[i+1:]...)
				break
			}
		}
		p.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Put returns a connection to pool
func (p *Pool) Put(conn Connection) error {
	if conn == nil {
		return ErrConnectionInvalid
	}
	
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		conn.Close()
		return ErrPoolClosed
	}
	
	// Try to satisfy waiting request
	if len(p.waiters) > 0 {
		waiter := p.waiters[0]
		p.waiters = p.waiters[1:]
		
		wrapper := &connWrapper{
			conn:       conn,
			createdAt:  time.Now(),
			lastUsedAt: time.Now(),
		}
		
		waiter <- wrapper
		return nil
	}
	
	// Validate connection
	if !p.factory.Validate(conn) {
		conn.Close()
		p.numOpen--
		p.metrics.IncrementOperationsFailed()
		return ErrConnectionInvalid
	}
	
	// Reset connection state
	if err := conn.Reset(); err != nil {
		conn.Close()
		p.numOpen--
		p.metrics.IncrementOperationsFailed()
		return err
	}
	
	// Add to idle pool if not full
	if len(p.conns) < p.config.MaxIdle {
		wrapper := &connWrapper{
			conn:       conn,
			createdAt:  time.Now(),
			lastUsedAt: time.Now(),
		}
		p.conns = append(p.conns, wrapper)
		p.metrics.SetConnectionsIdle(int64(len(p.conns)))
		return nil
	}
	
	// Pool full, close connection
	conn.Close()
	p.numOpen--
	
	return nil
}

// Close closes the pool and all connections
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil
	}
	
	p.closed = true
	
	// Close all idle connections
	for _, wrapper := range p.conns {
		wrapper.conn.Close()
	}
	p.conns = nil
	p.numOpen = 0
	
	// Notify all waiters
	for _, waiter := range p.waiters {
		waiter <- nil
	}
	p.waiters = nil
	
	p.metrics.UpdateHealthStatus("closed")
	
	return nil
}

// Stats returns pool statistics
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	return PoolStats{
		OpenConnections:  p.numOpen,
		IdleConnections:  len(p.conns),
		WaitingRequests:  len(p.waiters),
		MaxOpenConns:     p.config.MaxOpen,
		MaxIdleConns:     p.config.MaxIdle,
	}
}

// PoolStats represents pool statistics
type PoolStats struct {
	OpenConnections int `json:"open_connections"`
	IdleConnections int `json:"idle_connections"`
	WaitingRequests int `json:"waiting_requests"`
	MaxOpenConns    int `json:"max_open_conns"`
	MaxIdleConns    int `json:"max_idle_conns"`
}

// GetMetrics returns pool metrics
func (p *Pool) GetMetrics() *metrics.Metrics {
	return p.metrics
}

// createConnection creates a new connection
func (p *Pool) createConnection() (Connection, error) {
	start := time.Now()
	
	conn, err := p.factory.Create()
	if err != nil {
		p.metrics.IncrementConnectionsFailed()
		p.metrics.RecordError(err)
		return nil, err
	}
	
	p.metrics.IncrementConnectionsTotal()
	p.metrics.IncrementConnectionsActive()
	p.metrics.RecordLatency(time.Since(start).Milliseconds())
	
	return conn, nil
}

// isValidWrapper checks if connection wrapper is valid
func (p *Pool) isValidWrapper(wrapper *connWrapper) bool {
	now := time.Now()
	
	// Check max lifetime
	if p.config.MaxLifetime > 0 && now.Sub(wrapper.createdAt) > p.config.MaxLifetime {
		return false
	}
	
	// Check max idle time
	if p.config.MaxIdleTime > 0 && now.Sub(wrapper.lastUsedAt) > p.config.MaxIdleTime {
		return false
	}
	
	// Validate connection
	if !p.factory.Validate(wrapper.conn) {
		return false
	}
	
	return true
}

// CleanupStale removes stale connections from pool
func (p *Pool) CleanupStale() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return 0
	}
	
	cleaned := 0
	validConns := make([]*connWrapper, 0, len(p.conns))
	
	for _, wrapper := range p.conns {
		if p.isValidWrapper(wrapper) {
			validConns = append(validConns, wrapper)
		} else {
			wrapper.conn.Close()
			p.numOpen--
			cleaned++
		}
	}
	
	p.conns = validConns
	p.metrics.SetConnectionsIdle(int64(len(p.conns)))
	
	return cleaned
}

// StartCleaner starts background cleaner goroutine
func (p *Pool) StartCleaner(interval time.Duration) chan struct{} {
	done := make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				cleaned := p.CleanupStale()
				if cleaned > 0 {
					// Log cleanup if needed
				}
				
			case <-done:
				return
			}
		}
	}()
	
	return done
}
