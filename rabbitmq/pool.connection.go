package rabbitmq

import (
	"fmt"
	"sync"
	"time"

	"github.com/natansdj/lets"
	"github.com/rabbitmq/amqp091-go"
)

// ConnectionPool manages a single connection per vhost with automatic reconnection
type ConnectionPool struct {
	mu          sync.RWMutex
	connections map[string]*PooledConnection
	config      amqp091.Config
}

// PooledConnection represents a single RabbitMQ connection with reconnection logic
type PooledConnection struct {
	mu            sync.RWMutex
	conn          *amqp091.Connection
	dsn           string
	config        amqp091.Config
	vhost         string
	reconnecting  bool
	closed        bool
	retryDuration time.Duration
	maxRetries    int
	notifyClose   chan *amqp091.Error
}

// Global connection pool instance
var globalConnectionPool *ConnectionPool
var poolOnce sync.Once

// GetConnectionPool returns the global connection pool singleton
func GetConnectionPool() *ConnectionPool {
	poolOnce.Do(func() {
		globalConnectionPool = &ConnectionPool{
			connections: make(map[string]*PooledConnection),
			config:      amqp091.Config{Properties: amqp091.NewConnectionProperties()},
		}
	})
	return globalConnectionPool
}

// GetConnection returns a connection for the given DSN, creating it if necessary
func (cp *ConnectionPool) GetConnection(dsn string, config amqp091.Config) (*PooledConnection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Extract vhost from DSN for pooling key
	vhost := extractVHostFromDSN(dsn)

	// Check if connection already exists
	if pc, exists := cp.connections[vhost]; exists {
		if !pc.IsClosed() {
			lets.LogD("RabbitMQ Pool: Reusing existing connection for vhost: %s", vhost)
			return pc, nil
		}
		// Connection is closed, remove it
		delete(cp.connections, vhost)
	}

	// Create new pooled connection
	lets.LogI("RabbitMQ Pool: Creating new connection for vhost: %s", vhost)
	pc := &PooledConnection{
		dsn:           dsn,
		config:        config,
		vhost:         vhost,
		retryDuration: 10 * time.Second,
		maxRetries:    -1, // Infinite retries
		notifyClose:   make(chan *amqp091.Error, 1),
	}

	if err := pc.connect(); err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	cp.connections[vhost] = pc
	return pc, nil
}

// connect establishes the connection with retry logic
func (pc *PooledConnection) connect() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	var err error
	retryCount := 0

	for {
		pc.conn, err = amqp091.DialConfig(pc.dsn, pc.config)
		if err == nil {
			lets.LogI("RabbitMQ Pool: Connection established for vhost: %s", pc.vhost)

			// Setup connection monitoring
			go pc.monitorConnection()

			return nil
		}

		retryCount++
		if pc.maxRetries > 0 && retryCount >= pc.maxRetries {
			return fmt.Errorf("max retries reached: %w", err)
		}

		lets.LogERL("rabbitmq-pool-connect-failed", "RabbitMQ Pool: Connection failed for vhost %s (attempt %d): %v", pc.vhost, retryCount, err)
		time.Sleep(pc.retryDuration)
	}
}

// monitorConnection watches for connection errors and triggers reconnection
func (pc *PooledConnection) monitorConnection() {
	closeError := <-pc.conn.NotifyClose(make(chan *amqp091.Error))

	pc.mu.RLock()
	closed := pc.closed
	pc.mu.RUnlock()

	if closed {
		lets.LogD("RabbitMQ Pool: Connection closed gracefully for vhost: %s", pc.vhost)
		return
	}

	lets.LogERL("rabbitmq-pool-connection-closed", "RabbitMQ Pool: Connection lost for vhost %s: %v", pc.vhost, closeError)

	// Trigger reconnection
	go pc.reconnect()
}

// reconnect attempts to re-establish the connection
func (pc *PooledConnection) reconnect() {
	pc.mu.Lock()
	if pc.reconnecting || pc.closed {
		pc.mu.Unlock()
		return
	}
	pc.reconnecting = true
	pc.mu.Unlock()

	defer func() {
		pc.mu.Lock()
		pc.reconnecting = false
		pc.mu.Unlock()
	}()

	lets.LogI("RabbitMQ Pool: Attempting reconnection for vhost: %s", pc.vhost)

	retryCount := 0
	backoff := pc.retryDuration

	for {
		time.Sleep(backoff)

		conn, err := amqp091.DialConfig(pc.dsn, pc.config)
		if err == nil {
			pc.mu.Lock()
			pc.conn = conn
			pc.mu.Unlock()

			lets.LogI("RabbitMQ Pool: Reconnection successful for vhost: %s", pc.vhost)

			// Restart monitoring
			go pc.monitorConnection()
			return
		}

		retryCount++
		lets.LogERL("rabbitmq-pool-reconnect-attempt", "RabbitMQ Pool: Reconnection attempt %d failed for vhost %s: %v", retryCount, pc.vhost, err)

		// Exponential backoff with max 60 seconds
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
	}
}

// GetRawConnection returns the underlying amqp091.Connection (read-only access)
func (pc *PooledConnection) GetRawConnection() *amqp091.Connection {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.conn
}

// IsClosed checks if the connection is closed
func (pc *PooledConnection) IsClosed() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed {
		return true
	}

	if pc.conn == nil {
		return true
	}

	return pc.conn.IsClosed()
}

// Close gracefully closes the connection
func (pc *PooledConnection) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return nil
	}

	pc.closed = true

	if pc.conn != nil && !pc.conn.IsClosed() {
		lets.LogI("RabbitMQ Pool: Closing connection for vhost: %s", pc.vhost)
		return pc.conn.Close()
	}

	return nil
}

// CloseAll closes all connections in the pool
func (cp *ConnectionPool) CloseAll() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	lets.LogI("RabbitMQ Pool: Closing all connections")

	var lastErr error
	for vhost, pc := range cp.connections {
		if err := pc.Close(); err != nil {
			lets.LogE("RabbitMQ Pool: Error closing connection for vhost %s: %v", vhost, err)
			lastErr = err
		}
		delete(cp.connections, vhost)
	}

	return lastErr
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := map[string]interface{}{
		"total_connections": len(cp.connections),
		"connections":       make([]map[string]interface{}, 0, len(cp.connections)),
	}

	for vhost, pc := range cp.connections {
		connStats := map[string]interface{}{
			"vhost":        vhost,
			"closed":       pc.IsClosed(),
			"reconnecting": pc.reconnecting,
		}
		stats["connections"] = append(stats["connections"].([]map[string]interface{}), connStats)
	}

	return stats
}

// extractVHostFromDSN extracts the vhost from the connection string
func extractVHostFromDSN(dsn string) string {
	// DSN format: amqp://user:pass@host:port/vhost
	// Find the last '/' and extract vhost
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			if i+1 < len(dsn) {
				return dsn[i+1:]
			}
			return "/"
		}
	}
	return "/"
}
