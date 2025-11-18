package types

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	"gorm.io/gorm"
)

const (
	MYSQL_DB_HOST       = "localhost"
	MYSQL_DB_PORT       = "3306"
	MYSQL_DB_USERNAME   = "root"
	MYSQL_DB_PASSWORD   = ""
	MYSQL_DB_DATABASE   = "lets"
	MYSQL_DB_CHARSET    = "utf8"
	MYSQL_DB_PARSE_TIME = "True"
	MYSQL_DB_LOC        = "Local"
	MYSQL_DB_MIGRATION  = false

	// Pool configuration defaults
	MYSQL_MAX_IDLE_CONNS = 10 // Conservative for MySQL
	// If you have 5 app instances: Max pool size should be: 151 ÷ 5 = 30 connections per instance
	MYSQL_MAX_OPEN_CONNS    = 30
	MYSQL_CONN_MAX_LIFETIME = 300 // 5 minutes (rotate connections)
	MYSQL_MAX_RETRIES       = 3
)

type IMySQL interface {
	GetHost() string
	GetPort() string
	GetUsername() string
	GetPassword() string
	GetDatabase() string
	GetCharset() string
	GetParseTime() string
	GetLoc() string
	DebugMode() bool
	GetRepositories() []IMySQLRepository
	GetDsn() string
	Migration() bool
	GetQueryFields() bool
	GetDisableNestedTransaction() bool
	// Connection pool configuration
	GetMaxIdleConns() int
	GetMaxOpenConns() int
	GetConnMaxLifetime() int // in seconds
	GetMaxRetries() int
}

type MySQL struct {
	Host                     string
	Port                     string
	Username                 string
	Password                 string
	Database                 string
	Charset                  string
	ParseTime                string
	Loc                      string
	Debug                    bool
	Gorm                     *gorm.DB
	DB                       *sql.DB
	Repositories             []IMySQLRepository
	EnableMigration          bool
	QueryFields              bool
	DisableNestedTransaction bool
}

func (mysql *MySQL) GetHost() string {
	if mysql.Host == "" {
		fmt.Println("Configs MySQL: DB_HOST is not set in .env file, using default configuration.")
		return MYSQL_DB_HOST
	}
	return mysql.Host
}

func (mysql *MySQL) GetPort() string {
	if mysql.Host == "" {
		fmt.Println("Configs MySQL: DB_PORT is not set in .env file, using default configuration.")
		return MYSQL_DB_PORT
	}
	return mysql.Port
}

func (mysql *MySQL) GetUsername() string {
	if mysql.Username == "" {
		fmt.Println("Configs MySQL: DB_USERNAME is not set in .env file, using default configuration.")
		return MYSQL_DB_USERNAME
	}
	return mysql.Username
}

func (mysql *MySQL) GetPassword() string {
	if mysql.Host == "" {
		fmt.Println("Configs MySQL: DB_PASSWORD is not set in .env file, using default configuration.")
		return MYSQL_DB_PASSWORD
	}
	return mysql.Password
}

func (mysql *MySQL) GetDatabase() string {
	if mysql.Database == "" {
		fmt.Println("Configs MySQL: DB_DATABASE is not set in .env file, using default configuration.")
		return MYSQL_DB_DATABASE
	}
	return mysql.Database
}

func (mysql *MySQL) GetCharset() string {
	if mysql.Charset == "" {
		fmt.Println("Configs MySQL: Charset is not set in configs, using default configuration.")
		return MYSQL_DB_CHARSET
	}
	return mysql.Charset
}

func (mysql *MySQL) GetParseTime() string {
	if mysql.ParseTime == "" {
		fmt.Println("Configs MySQL: ParseTime is not set in configs, using default configuration.")
		return MYSQL_DB_CHARSET
	}
	return mysql.ParseTime
}

func (mysql *MySQL) GetLoc() string {
	if mysql.Loc == "" {
		fmt.Println("Configs MySQL: Loc is not set in configs, using default configuration.")
		return MYSQL_DB_LOC
	}
	return mysql.Loc
}

func (mysql *MySQL) DebugMode() bool {
	return mysql.Debug
}

func (mysql *MySQL) GetRepositories() []IMySQLRepository {
	return mysql.Repositories
}

func (mysql *MySQL) GetDsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=%s&loc=%s",
		mysql.GetUsername(),
		mysql.GetPassword(),
		mysql.GetHost(),
		mysql.GetPort(),
		mysql.GetDatabase(),
		mysql.GetCharset(),
		mysql.GetParseTime(),
		mysql.GetLoc(),
	)
}

func (mysql *MySQL) Migration() bool {
	return mysql.EnableMigration
}

func (mysql *MySQL) GetQueryFields() bool {
	return mysql.QueryFields
}

func (mysql *MySQL) GetDisableNestedTransaction() bool {
	return mysql.DisableNestedTransaction
}

// getMySQLEnvInt retrieves an integer from environment variable with validation
// Returns the default value and logs a warning if env var is not set or invalid
func getMySQLEnvInt(envKey string, defaultValue int, configName string) int {
	envValue := os.Getenv(envKey)
	if envValue == "" {
		fmt.Printf("Configs MySQL: %s is not set in .env file, using default configuration (%d).\n", envKey, defaultValue)
		return defaultValue
	}

	parsedValue, err := strconv.Atoi(envValue)
	if err != nil {
		fmt.Printf("Configs MySQL: %s has invalid value '%s' in .env file, using default configuration (%d).\n", envKey, envValue, defaultValue)
		return defaultValue
	}

	if parsedValue <= 0 {
		fmt.Printf("Configs MySQL: %s must be positive, got %d, using default configuration (%d).\n", envKey, parsedValue, defaultValue)
		return defaultValue
	}

	return parsedValue
}

// GetMaxIdleConns returns the maximum number of connections in the idle connection pool
func (mysql *MySQL) GetMaxIdleConns() int {
	return getMySQLEnvInt("MYSQL_MAX_IDLE_CONNS", MYSQL_MAX_IDLE_CONNS, "Max Idle Connections")
}

// GetMaxOpenConns returns the maximum number of open connections to the database
func (mysql *MySQL) GetMaxOpenConns() int {
	return getMySQLEnvInt("MYSQL_MAX_OPEN_CONNS", MYSQL_MAX_OPEN_CONNS, "Max Open Connections")
}

// GetConnMaxLifetime returns the maximum amount of time a connection may be reused (in seconds)
func (mysql *MySQL) GetConnMaxLifetime() int {
	return getMySQLEnvInt("MYSQL_CONN_MAX_LIFETIME", MYSQL_CONN_MAX_LIFETIME, "Connection Max Lifetime")
}

// GetMaxRetries returns the maximum number of retry attempts for connection
func (mysql *MySQL) GetMaxRetries() int {
	return getMySQLEnvInt("MYSQL_MAX_RETRIES", MYSQL_MAX_RETRIES, "Max Retries")
}
