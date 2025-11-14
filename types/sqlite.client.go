package types

type SqLite struct {
	Debug           bool
	DBPath          string
	Repositories    []IMySQLRepository
	EnableMigration bool

	QueryFields              bool
	DisableNestedTransaction bool

	dsn string
}

// GetMaxIdleConns returns the maximum number of connections in the idle connection pool
// SQLite benefits from low connection counts
func (s *SqLite) GetMaxIdleConns() int {
	// Default: 1 connection (SQLite single writer optimization)
	return 1
}

// GetMaxOpenConns returns the maximum number of open connections to the database
// SQLite doesn't benefit from high connection counts
func (s *SqLite) GetMaxOpenConns() int {
	// Default: 5 connections (lower for SQLite)
	return 5
}

// GetConnMaxLifetime returns the maximum amount of time a connection may be reused (in seconds)
func (s *SqLite) GetConnMaxLifetime() int {
	// Default: 1 hour (3600 seconds)
	return 3600
}
