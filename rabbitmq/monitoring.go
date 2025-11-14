package rabbitmq

import (
	"fmt"
	"time"

	"github.com/natansdj/lets"
)

// MonitoringStats holds RabbitMQ performance metrics
type MonitoringStats struct {
	ConnectionCount  int
	ActiveChannels   int
	TotalChannels    int
	MessagesSent     uint64
	MessagesReceived uint64
	ErrorCount       uint64
	LastError        string
	LastErrorTime    time.Time
	UptimeSeconds    int64
	PoolUtilization  float64
}

var (
	stats     MonitoringStats
	startTime = time.Now()
)

// GetStats returns current monitoring statistics
func GetStats() MonitoringStats {
	pool := GetConnectionPool()
	poolStats := pool.GetStats()

	stats.ConnectionCount = poolStats["total_connections"].(int)
	stats.UptimeSeconds = int64(time.Since(startTime).Seconds())

	// Calculate pool utilization
	if stats.ConnectionCount > 0 {
		stats.PoolUtilization = 100.0
	}

	return stats
}

// IncrementMessagesSent increments the sent message counter
func IncrementMessagesSent() {
	stats.MessagesSent++
}

// IncrementMessagesReceived increments the received message counter
func IncrementMessagesReceived() {
	stats.MessagesReceived++
}

// RecordError records an error in the stats
func RecordError(err error) {
	if err != nil {
		stats.ErrorCount++
		stats.LastError = err.Error()
		stats.LastErrorTime = time.Now()
	}
}

// LogStats logs current statistics
func LogStats() {
	currentStats := GetStats()
	lets.LogI("RabbitMQ Stats: Connections=%d, Channels=%d/%d, Sent=%d, Received=%d, Errors=%d, Uptime=%ds",
		currentStats.ConnectionCount,
		currentStats.ActiveChannels,
		currentStats.TotalChannels,
		currentStats.MessagesSent,
		currentStats.MessagesReceived,
		currentStats.ErrorCount,
		currentStats.UptimeSeconds,
	)
}

// CheckHealth performs health checks and returns any issues
func CheckHealth() []string {
	var issues []string
	currentStats := GetStats()

	// Check connection count
	if currentStats.ConnectionCount == 0 {
		issues = append(issues, "No active connections")
	} else if currentStats.ConnectionCount > 10 {
		issues = append(issues, fmt.Sprintf("Too many connections: %d (expected ~5)", currentStats.ConnectionCount))
	}

	// Check error rate
	if currentStats.MessagesSent > 0 || currentStats.MessagesReceived > 0 {
		totalMessages := currentStats.MessagesSent + currentStats.MessagesReceived
		errorRate := float64(currentStats.ErrorCount) / float64(totalMessages) * 100

		if errorRate > 1.0 {
			issues = append(issues, fmt.Sprintf("High error rate: %.2f%% (threshold: 1%%)", errorRate))
		}
	}

	// Check recent errors
	if !currentStats.LastErrorTime.IsZero() {
		timeSinceError := time.Since(currentStats.LastErrorTime)
		if timeSinceError < 5*time.Minute {
			issues = append(issues, fmt.Sprintf("Recent error (%s ago): %s", timeSinceError.Round(time.Second), currentStats.LastError))
		}
	}

	return issues
}

// StartMonitoring starts periodic monitoring (call this once at startup)
func StartMonitoring(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			LogStats()

			if issues := CheckHealth(); len(issues) > 0 {
				for _, issue := range issues {
					lets.LogW("RabbitMQ Health Check: %s", issue)
				}
			}
		}
	}()

	lets.LogI("RabbitMQ Monitoring: Started with %v interval", interval)
}
