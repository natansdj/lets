package types

import (
	"fmt"
)

const (
	MONGODB_DSN      = "mongodb://localhost:27017"
	MONGODB_DATABASE = "default"
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

// GetMaxPoolSize returns the maximum number of connections in the pool
func (r *MongoDB) GetMaxPoolSize() uint64 {
	// Default: 100 connections
	return 100
}

// GetMinPoolSize returns the minimum number of connections in the pool
func (r *MongoDB) GetMinPoolSize() uint64 {
	// Default: 10 connections
	return 10
}

// GetMaxConnIdleTime returns the maximum connection idle time in seconds
func (r *MongoDB) GetMaxConnIdleTime() int {
	// Default: 5 minutes (300 seconds)
	return 300
}

// GetConnectTimeout returns the connection timeout in seconds
func (r *MongoDB) GetConnectTimeout() int {
	// Default: 10 seconds
	return 10
}

// GetServerSelectionTimeout returns the server selection timeout in seconds
func (r *MongoDB) GetServerSelectionTimeout() int {
	// Default: 5 seconds
	return 5
}

// GetSocketTimeout returns the socket timeout in seconds
func (r *MongoDB) GetSocketTimeout() int {
	// Default: 3 seconds
	return 3
}

// GetMaxRetries returns the maximum number of retry attempts
func (r *MongoDB) GetMaxRetries() int {
	// Default: 3 retries
	return 3
}
