package types

import (
	"fmt"
	"os"
	"strconv"
)

const (
	MONGODB_DSN      = "mongodb://localhost:27017"
	MONGODB_DATABASE = "default"

	// Pool configuration defaults
	MONGODB_MAX_POOL_SIZE            = 100 // Default MongoDB recommendation
	MONGODB_MIN_POOL_SIZE            = 10  // Keep connections warm
	MONGODB_MAX_CONN_IDLE_TIME       = 300 // 5 minutes idle before cleanup
	MONGODB_CONNECT_TIMEOUT          = 10  // Initial connection timeout
	MONGODB_SERVER_SELECTION_TIMEOUT = 5   // Replica set selection
	MONGODB_SOCKET_TIMEOUT           = 30  // Read/write operations timeout
	MONGODB_MAX_RETRIES              = 3   // Standard retry
)

type IMongoDB interface {
	GetDsn() string
	GetDatabase() string
	GetRepositories() []IMongoDBRepository
	// Connection pool configuration
	GetMaxPoolSize() uint64
	GetMinPoolSize() uint64
	GetMaxConnIdleTime() int        // in seconds
	GetConnectTimeout() int         // in seconds
	GetServerSelectionTimeout() int // in seconds
	GetSocketTimeout() int          // in seconds
	GetMaxRetries() int
}

type MongoDB struct {
	Dsn          string
	Database     string
	Repositories []IMongoDBRepository
}

func (r *MongoDB) GetDsn() string {
	if r.Dsn == "0" {
		fmt.Println("Configs MongoDB: MONGODB_DSN is not set in .env file, using default configuration.")
		return MONGODB_DATABASE
	}
	return r.Dsn
}

func (r *MongoDB) GetDatabase() string {
	if r.Database == "0" {
		fmt.Println("Configs MongoDB: MONGODB_DATABASE is not set in .env file, using default configuration.")
		return MONGODB_DATABASE
	}
	return r.Database
}

func (r *MongoDB) GetRepositories() []IMongoDBRepository {
	return r.Repositories
}

// getEnvInt retrieves an integer from environment variable with validation
// Returns the default value and logs a warning if env var is not set or invalid
func getMongoEnvInt(envKey string, defaultValue int, configName string) int {
	envValue := os.Getenv(envKey)
	if envValue == "" {
		fmt.Printf("Configs MongoDB: %s is not set in .env file, using default configuration (%d).\n", envKey, defaultValue)
		return defaultValue
	}

	parsedValue, err := strconv.Atoi(envValue)
	if err != nil {
		fmt.Printf("Configs MongoDB: %s has invalid value '%s' in .env file, using default configuration (%d).\n", envKey, envValue, defaultValue)
		return defaultValue
	}

	if parsedValue <= 0 {
		fmt.Printf("Configs MongoDB: %s must be positive, got %d, using default configuration (%d).\n", envKey, parsedValue, defaultValue)
		return defaultValue
	}

	return parsedValue
}

// getEnvUint64 retrieves a uint64 from environment variable with validation
// Returns the default value and logs a warning if env var is not set or invalid
func getMongoEnvUint64(envKey string, defaultValue uint64, configName string) uint64 {
	envValue := os.Getenv(envKey)
	if envValue == "" {
		fmt.Printf("Configs MongoDB: %s is not set in .env file, using default configuration (%d).\n", envKey, defaultValue)
		return defaultValue
	}

	parsedValue, err := strconv.ParseUint(envValue, 10, 64)
	if err != nil {
		fmt.Printf("Configs MongoDB: %s has invalid value '%s' in .env file, using default configuration (%d).\n", envKey, envValue, defaultValue)
		return defaultValue
	}

	if parsedValue == 0 {
		fmt.Printf("Configs MongoDB: %s must be positive, got %d, using default configuration (%d).\n", envKey, parsedValue, defaultValue)
		return defaultValue
	}

	return parsedValue
}

// GetMaxPoolSize returns the maximum number of connections in the pool
func (r *MongoDB) GetMaxPoolSize() uint64 {
	return getMongoEnvUint64("MONGODB_MAX_POOL_SIZE", MONGODB_MAX_POOL_SIZE, "Max Pool Size")
}

// GetMinPoolSize returns the minimum number of connections in the pool
func (r *MongoDB) GetMinPoolSize() uint64 {
	return getMongoEnvUint64("MONGODB_MIN_POOL_SIZE", MONGODB_MIN_POOL_SIZE, "Min Pool Size")
}

// GetMaxConnIdleTime returns the maximum connection idle time in seconds
func (r *MongoDB) GetMaxConnIdleTime() int {
	return getMongoEnvInt("MONGODB_MAX_CONN_IDLE_TIME", MONGODB_MAX_CONN_IDLE_TIME, "Max Connection Idle Time")
}

// GetConnectTimeout returns the connection timeout in seconds
func (r *MongoDB) GetConnectTimeout() int {
	return getMongoEnvInt("MONGODB_CONNECT_TIMEOUT", MONGODB_CONNECT_TIMEOUT, "Connect Timeout")
}

// GetServerSelectionTimeout returns the server selection timeout in seconds
func (r *MongoDB) GetServerSelectionTimeout() int {
	return getMongoEnvInt("MONGODB_SERVER_SELECTION_TIMEOUT", MONGODB_SERVER_SELECTION_TIMEOUT, "Server Selection Timeout")
}

// GetSocketTimeout returns the socket timeout in seconds
func (r *MongoDB) GetSocketTimeout() int {
	return getMongoEnvInt("MONGODB_SOCKET_TIMEOUT", MONGODB_SOCKET_TIMEOUT, "Socket Timeout")
}

// GetMaxRetries returns the maximum number of retry attempts
func (r *MongoDB) GetMaxRetries() int {
	return getMongoEnvInt("MONGODB_MAX_RETRIES", MONGODB_MAX_RETRIES, "Max Retries")
}
