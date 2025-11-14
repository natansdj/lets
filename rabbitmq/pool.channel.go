package rabbitmq

import (
	"fmt"
	"sync"

	"github.com/natansdj/lets"
	"github.com/rabbitmq/amqp091-go"
)

// ChannelManager manages per-goroutine channel allocation and reuse
type ChannelManager struct {
	mu       sync.RWMutex
	channels map[uint64]*PooledChannel // goroutine ID -> channel
	conn     *PooledConnection
}

// PooledChannel represents a channel with automatic recreation capability
type PooledChannel struct {
	mu          sync.RWMutex
	channel     *amqp091.Channel
	conn        *PooledConnection
	goroutineID uint64
	closed      bool
}

// NewChannelManager creates a new channel manager for a connection
func NewChannelManager(conn *PooledConnection) *ChannelManager {
	return &ChannelManager{
		channels: make(map[uint64]*PooledChannel),
		conn:     conn,
	}
}

// GetChannel returns a channel for the current goroutine, creating it if necessary
func (cm *ChannelManager) GetChannel() (*PooledChannel, error) {
	gid := getGoroutineID()

	cm.mu.RLock()
	pc, exists := cm.channels[gid]
	cm.mu.RUnlock()

	if exists && !pc.IsClosed() {
		// Reuse existing channel
		return pc, nil
	}

	// Need to create new channel
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check after acquiring write lock
	if pc, exists := cm.channels[gid]; exists && !pc.IsClosed() {
		return pc, nil
	}

	// Create new channel
	lets.LogD("RabbitMQ Channel: Creating channel for goroutine %d", gid)

	rawConn := cm.conn.GetRawConnection()
	if rawConn == nil || rawConn.IsClosed() {
		return nil, fmt.Errorf("connection is closed")
	}

	ch, err := rawConn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	pc = &PooledChannel{
		channel:     ch,
		conn:        cm.conn,
		goroutineID: gid,
	}

	cm.channels[gid] = pc

	// Monitor channel errors
	go pc.monitorChannel(cm)

	return pc, nil
}

// monitorChannel watches for channel errors and handles cleanup
func (pc *PooledChannel) monitorChannel(cm *ChannelManager) {
	closeError := <-pc.channel.NotifyClose(make(chan *amqp091.Error))

	pc.mu.RLock()
	closed := pc.closed
	gid := pc.goroutineID
	pc.mu.RUnlock()

	if closed {
		lets.LogD("RabbitMQ Channel: Channel closed gracefully for goroutine %d", gid)
		return
	}

	lets.LogERL("rabbitmq-channel-error", "RabbitMQ Channel: Channel closed for goroutine %d: %v", gid, closeError)

	// Remove from manager so it will be recreated on next use
	cm.mu.Lock()
	delete(cm.channels, gid)
	cm.mu.Unlock()
}

// GetRawChannel returns the underlying amqp091.Channel
func (pc *PooledChannel) GetRawChannel() *amqp091.Channel {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.channel
}

// IsClosed checks if the channel is closed
func (pc *PooledChannel) IsClosed() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if pc.closed {
		return true
	}

	if pc.channel == nil {
		return true
	}

	// Check connection status
	if pc.conn.IsClosed() {
		return true
	}

	return false
}

// Close gracefully closes the channel
func (pc *PooledChannel) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.closed {
		return nil
	}

	pc.closed = true

	if pc.channel != nil {
		lets.LogD("RabbitMQ Channel: Closing channel for goroutine %d", pc.goroutineID)
		return pc.channel.Close()
	}

	return nil
}

// CloseAll closes all channels in the manager
func (cm *ChannelManager) CloseAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	lets.LogI("RabbitMQ Channel: Closing all channels")

	var lastErr error
	for gid, pc := range cm.channels {
		if err := pc.Close(); err != nil {
			lets.LogE("RabbitMQ Channel: Error closing channel for goroutine %d: %v", gid, err)
			lastErr = err
		}
		delete(cm.channels, gid)
	}

	return lastErr
}

// GetStats returns channel manager statistics
func (cm *ChannelManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	activeChannels := 0
	for _, pc := range cm.channels {
		if !pc.IsClosed() {
			activeChannels++
		}
	}

	return map[string]interface{}{
		"total_channels":  len(cm.channels),
		"active_channels": activeChannels,
	}
}

// RecreateChannel attempts to recreate a channel
func (cm *ChannelManager) RecreateChannel(gid uint64) (*PooledChannel, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Remove old channel
	if pc, exists := cm.channels[gid]; exists {
		pc.Close()
		delete(cm.channels, gid)
	}

	// Create new channel
	rawConn := cm.conn.GetRawConnection()
	if rawConn == nil || rawConn.IsClosed() {
		return nil, fmt.Errorf("connection is closed")
	}

	ch, err := rawConn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to recreate channel: %w", err)
	}

	pc := &PooledChannel{
		channel:     ch,
		conn:        cm.conn,
		goroutineID: gid,
	}

	cm.channels[gid] = pc

	// Monitor channel errors
	go pc.monitorChannel(cm)

	lets.LogI("RabbitMQ Channel: Recreated channel for goroutine %d", gid)

	return pc, nil
}
