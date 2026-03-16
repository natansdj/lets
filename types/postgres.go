package types

import (
	"fmt"
	"os"
	"strconv"

	"gorm.io/gorm"
)

const (
	POSTGRES_DB_HOST     = "localhost"
	POSTGRES_DB_PORT     = "5432"
	POSTGRES_DB_USERNAME = "postgres"
	POSTGRES_DB_PASSWORD = ""
	POSTGRES_DB_DATABASE = "postgres"
	POSTGRES_DB_SSLMODE  = "disable"
	POSTGRES_DB_TIMEZONE = "UTC"

	POSTGRES_MAX_IDLE_CONNS    = 10
	POSTGRES_MAX_OPEN_CONNS    = 30
	POSTGRES_CONN_MAX_LIFETIME = 300
	POSTGRES_MAX_RETRIES       = 3
)

type IPostgres interface {
	GetHost() string
	GetPort() string
	GetUsername() string
	GetPassword() string
	GetDatabase() string
	GetSSLMode() string
	GetTimeZone() string
	DebugMode() bool
	GetRepositories() []IMySQLRepository
	GetDsn() string
	Migration() bool
	GetQueryFields() bool
	GetDisableNestedTransaction() bool
	GetMaxIdleConns() int
	GetMaxOpenConns() int
	GetConnMaxLifetime() int
	GetMaxRetries() int
}

type Postgres struct {
	Host                     string
	Port                     string
	Username                 string
	Password                 string
	Database                 string
	SSLMode                  string
	TimeZone                 string
	Debug                    bool
	Gorm                     *gorm.DB
	Repositories             []IMySQLRepository
	EnableMigration          bool
	QueryFields              bool
	DisableNestedTransaction bool
}

func (pg *Postgres) GetHost() string {
	if pg.Host == "" {
		fmt.Println("Configs Postgres: DB_HOST is not set in .env file, using default configuration.")
		return POSTGRES_DB_HOST
	}
	return pg.Host
}

func (pg *Postgres) GetPort() string {
	if pg.Port == "" {
		fmt.Println("Configs Postgres: DB_PORT is not set in .env file, using default configuration.")
		return POSTGRES_DB_PORT
	}
	return pg.Port
}

func (pg *Postgres) GetUsername() string {
	if pg.Username == "" {
		fmt.Println("Configs Postgres: DB_USERNAME is not set in .env file, using default configuration.")
		return POSTGRES_DB_USERNAME
	}
	return pg.Username
}

func (pg *Postgres) GetPassword() string {
	if pg.Password == "" {
		fmt.Println("Configs Postgres: DB_PASSWORD is not set in .env file, using default configuration.")
		return POSTGRES_DB_PASSWORD
	}
	return pg.Password
}

func (pg *Postgres) GetDatabase() string {
	if pg.Database == "" {
		fmt.Println("Configs Postgres: DB_DATABASE is not set in .env file, using default configuration.")
		return POSTGRES_DB_DATABASE
	}
	return pg.Database
}

func (pg *Postgres) GetSSLMode() string {
	if pg.SSLMode == "" {
		fmt.Println("Configs Postgres: DB_SSLMODE is not set in .env file, using default configuration.")
		return POSTGRES_DB_SSLMODE
	}
	return pg.SSLMode
}

func (pg *Postgres) GetTimeZone() string {
	if pg.TimeZone == "" {
		fmt.Println("Configs Postgres: DB_TIMEZONE is not set in .env file, using default configuration.")
		return POSTGRES_DB_TIMEZONE
	}
	return pg.TimeZone
}

func (pg *Postgres) DebugMode() bool {
	return pg.Debug
}

func (pg *Postgres) GetRepositories() []IMySQLRepository {
	return pg.Repositories
}

func (pg *Postgres) GetDsn() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		pg.GetHost(),
		pg.GetPort(),
		pg.GetUsername(),
		pg.GetPassword(),
		pg.GetDatabase(),
		pg.GetSSLMode(),
		pg.GetTimeZone(),
	)
}

func (pg *Postgres) Migration() bool {
	return pg.EnableMigration
}

func (pg *Postgres) GetQueryFields() bool {
	return pg.QueryFields
}

func (pg *Postgres) GetDisableNestedTransaction() bool {
	return pg.DisableNestedTransaction
}

func getPostgresEnvInt(envKey string, defaultValue int) int {
	envValue := os.Getenv(envKey)
	if envValue == "" {
		fmt.Printf("Configs Postgres: %s is not set in .env file, using default configuration (%d).\n", envKey, defaultValue)
		return defaultValue
	}

	parsedValue, err := strconv.Atoi(envValue)
	if err != nil {
		fmt.Printf("Configs Postgres: %s has invalid value '%s' in .env file, using default configuration (%d).\n", envKey, envValue, defaultValue)
		return defaultValue
	}

	if parsedValue <= 0 {
		fmt.Printf("Configs Postgres: %s must be positive, got %d, using default configuration (%d).\n", envKey, parsedValue, defaultValue)
		return defaultValue
	}

	return parsedValue
}

func (pg *Postgres) GetMaxIdleConns() int {
	return getPostgresEnvInt("POSTGRES_MAX_IDLE_CONNS", POSTGRES_MAX_IDLE_CONNS)
}

func (pg *Postgres) GetMaxOpenConns() int {
	return getPostgresEnvInt("POSTGRES_MAX_OPEN_CONNS", POSTGRES_MAX_OPEN_CONNS)
}

func (pg *Postgres) GetConnMaxLifetime() int {
	return getPostgresEnvInt("POSTGRES_CONN_MAX_LIFETIME", POSTGRES_CONN_MAX_LIFETIME)
}

func (pg *Postgres) GetMaxRetries() int {
	return getPostgresEnvInt("POSTGRES_MAX_RETRIES", POSTGRES_MAX_RETRIES)
}
