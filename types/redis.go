package types

import (
	"fmt"
	"os"
	"strconv"

	"github.com/natansdj/lets"
)

const (
	REDIS_HOST     = "localhost"
	REDIS_PORT     = "6379"
	REDIS_USERNAME = ""
	REDIS_PASSWORD = ""
	REDIS_DATABASE = 0

	// Pool configuration defaults
	REDIS_POOL_SIZE      = 50 // 50-100 for most apps
	REDIS_MIN_IDLE_CONNS = 10 // Keep warm connections
	REDIS_MAX_RETRIES    = 3  // Standard retry
	REDIS_DIAL_TIMEOUT   = 5  // Network timeout
	REDIS_READ_TIMEOUT   = 3  // Fast read timeout
	REDIS_WRITE_TIMEOUT  = 3  // Fast write timeout
	REDIS_POOL_TIMEOUT   = 4  // Wait for connection
)

type IRedis interface {
	GetHost() string
	GetPort() string
	GetUsername() string
	GetPassword() string
	GetDatabase() int
	GetDsn() string
	// DebugMode() bool
	GetRepositories() []IRedisRepository
	// Connection pool configuration
	GetPoolSize() int
	GetMinIdleConns() int
	GetMaxRetries() int
	GetDialTimeout() int  // in seconds
	GetReadTimeout() int  // in seconds
	GetWriteTimeout() int // in seconds
	GetPoolTimeout() int  // in seconds
}

type Redis struct {
	Host         string
	Port         string
	Username     string
	Password     string
	Database     int
	Debug        bool
	Repositories []IRedisRepository
}

func (r *Redis) GetHost() string {
	if r.Host == "" {
		lets.LogW("Configs Redis: REDIS_HOST is not set in .env file, using default configuration.")
		return REDIS_HOST
	}
	return r.Host
}

func (r *Redis) GetPort() string {
	if r.Port == "" {
		lets.LogW("Configs Redis: REDIS_PORT is not set in .env file, using default configuration.")
		return REDIS_PORT
	}
	return r.Port
}

func (r *Redis) GetUsername() string {
	// if r.Username == "" {
	// 	lets.LogW("Configs Redis: REDIS_USERNAME is not set in .env file, using default configuration.")
	// 	return REDIS_USERNAME
	// }
	return r.Username
}
func (r *Redis) GetPassword() string {
	if r.Password == "" {
		lets.LogW("Configs Redis: REDIS_PASSWORD is not set in .env file, using default configuration.")
		return REDIS_PASSWORD
	}
	return r.Password
}

func (r *Redis) GetDatabase() int {
	if r.Database == 0 {
		lets.LogW("Configs Redis: REDIS_DATABASE is not set in .env file, using default configuration.")
		return REDIS_DATABASE
	}
	return r.Database
}

func (r *Redis) DebugMode() bool {
	return r.Debug
}

func (r *Redis) GetRepositories() []IRedisRepository {
	return r.Repositories
}

func (r *Redis) GetDsn() string {
	//  return fmt.Sprintf("redis://%s:%s@%s:%s",
	//  r.GetUsername(),
	//  r.GetPassword(),
	//  r.GetHost(),
	//  r.GetPort(),
	// )

	return fmt.Sprintf("%s:%s",
		r.GetHost(),
		r.GetPort(),
	)
}

// getEnvInt retrieves an integer from environment variable with validation
// Returns the default value and logs a warning if env var is not set or invalid
func getEnvInt(envKey string, defaultValue int, configName string) int {
	envValue := os.Getenv(envKey)
	if envValue == "" {
		lets.LogW("Configs Redis: %s is not set in .env file, using default configuration (%d).", envKey, defaultValue)
		return defaultValue
	}

	parsedValue, err := strconv.Atoi(envValue)
	if err != nil {
		lets.LogW("Configs Redis: %s has invalid value '%s' in .env file, using default configuration (%d).", envKey, envValue, defaultValue)
		return defaultValue
	}

	if parsedValue <= 0 {
		lets.LogW("Configs Redis: %s must be positive, got %d, using default configuration (%d).", envKey, parsedValue, defaultValue)
		return defaultValue
	}

	return parsedValue
}

// GetPoolSize returns the maximum number of socket connections
func (r *Redis) GetPoolSize() int {
	return getEnvInt("REDIS_POOL_SIZE", REDIS_POOL_SIZE, "Pool Size")
}

// GetMinIdleConns returns the minimum number of idle connections
func (r *Redis) GetMinIdleConns() int {
	return getEnvInt("REDIS_MIN_IDLE_CONNS", REDIS_MIN_IDLE_CONNS, "Min Idle Connections")
}

// GetMaxRetries returns the maximum number of retries before giving up
func (r *Redis) GetMaxRetries() int {
	return getEnvInt("REDIS_MAX_RETRIES", REDIS_MAX_RETRIES, "Max Retries")
}

// GetDialTimeout returns dial timeout in seconds
func (r *Redis) GetDialTimeout() int {
	return getEnvInt("REDIS_DIAL_TIMEOUT", REDIS_DIAL_TIMEOUT, "Dial Timeout")
}

// GetReadTimeout returns read timeout in seconds
func (r *Redis) GetReadTimeout() int {
	return getEnvInt("REDIS_READ_TIMEOUT", REDIS_READ_TIMEOUT, "Read Timeout")
}

// GetWriteTimeout returns write timeout in seconds
func (r *Redis) GetWriteTimeout() int {
	return getEnvInt("REDIS_WRITE_TIMEOUT", REDIS_WRITE_TIMEOUT, "Write Timeout")
}

// GetPoolTimeout returns pool timeout in seconds
func (r *Redis) GetPoolTimeout() int {
	return getEnvInt("REDIS_POOL_TIMEOUT", REDIS_POOL_TIMEOUT, "Pool Timeout")
}
