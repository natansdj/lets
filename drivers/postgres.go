package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var PostgresConfig []types.IPostgres

type postgresProvider struct {
	Gorm   *gorm.DB
	Sql    *sql.DB
	Config types.IPostgres
	Mu     sync.RWMutex
}

func (p *postgresProvider) Connect() {
	p.connectWithRetry(0)
}

func (p *postgresProvider) connectWithRetry(attempt int) {
	maxRetries := p.Config.GetMaxRetries()
	if attempt >= maxRetries {
		lets.LogF("Postgres: Max connection retries (%d) exceeded", maxRetries)
		return
	}

	var logType logger.Interface = logger.Default.LogMode(logger.Warn)
	if p.Config.DebugMode() {
		logType = logger.Default.LogMode(logger.Info)
	}

	var err error
	p.Gorm, err = gorm.Open(postgres.Open(p.Config.GetDsn()), &gorm.Config{
		Logger:      logType,
		QueryFields: p.Config.GetQueryFields(),
		NamingStrategy: schema.NamingStrategy{
			NoLowerCase:   true,
			SingularTable: true,
		},
		PrepareStmt:              false,
		DisableNestedTransaction: p.Config.GetDisableNestedTransaction(),
	})

	if err != nil {
		backoff := time.Second * time.Duration(1<<uint(attempt))
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		lets.LogERL("postgres-connect-retry", "Postgres connection attempt %d failed: %v. Retrying in %v...", attempt+1, err, backoff)
		time.Sleep(backoff)
		p.connectWithRetry(attempt + 1)
		return
	}

	p.Sql, err = p.Gorm.DB()
	if err != nil {
		backoff := time.Second * time.Duration(1<<uint(attempt))
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		lets.LogERL("postgres-db-retry", "Postgres DB() attempt %d failed: %v. Retrying in %v...", attempt+1, err, backoff)
		time.Sleep(backoff)
		p.connectWithRetry(attempt + 1)
		return
	}

	maxIdleConns := p.Config.GetMaxIdleConns()
	maxOpenConns := p.Config.GetMaxOpenConns()
	connMaxLifetime := time.Duration(p.Config.GetConnMaxLifetime()) * time.Second

	p.Sql.SetMaxIdleConns(maxIdleConns)
	p.Sql.SetMaxOpenConns(maxOpenConns)
	p.Sql.SetConnMaxLifetime(connMaxLifetime)

	if err = p.Sql.Ping(); err != nil {
		backoff := time.Second * time.Duration(1<<uint(attempt))
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		lets.LogERL("postgres-ping-retry", "Postgres ping attempt %d failed: %v. Retrying in %v...", attempt+1, err, backoff)
		time.Sleep(backoff)
		p.connectWithRetry(attempt + 1)
		return
	}

	lets.LogI("Postgres Client Connected (MaxIdle: %d, MaxOpen: %d, Lifetime: %v)", maxIdleConns, maxOpenConns, connMaxLifetime)
}

func (p *postgresProvider) Disconnect() {
	lets.LogI("Postgres Stopping ...")
	err := p.Sql.Close()
	if err != nil {
		lets.LogE(err.Error())
		return
	}
	lets.LogI("Postgres Stopped ...")
}

func Postgres() (disconnectors []func()) {
	if PostgresConfig == nil {
		return
	}

	lets.LogI("Postgres Client Starting ...")

	for _, config := range PostgresConfig {
		pg := postgresProvider{Config: config}
		pg.Connect()
		disconnectors = append(disconnectors, pg.Disconnect)

		for _, repository := range config.GetRepositories() {
			repository.SetDriver(pg.Gorm, &pg.Mu)
		}

		if config.Migration() {
			err := pg.Gorm.AutoMigrate(&migration{})
			if err != nil {
				lets.LogE("Unable to run migration %w", err)
				return
			}
			MigratePostgres(pg.Gorm, pg.Sql)
		}
	}

	return
}

func MigratePostgres(g *gorm.DB, db *sql.DB) {
	var batch uint = 1
	lastMigration := &migration{}
	result := g.Last(lastMigration)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		lets.LogE("Unable to run migration %w", result.Error)
		return
	}

	batch = lastMigration.Batch + 1

	files, err := os.ReadDir("migrations")
	if err != nil {
		lets.LogE(err.Error())
		time.Sleep(time.Second * 3)
		MigratePostgres(g, db)
		return
	}

	for _, file := range files {
		name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

		search := &migration{Migration: name}
		result := g.Where("migration = ?", name).First(search)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			lets.LogE("Unable to run migration %w", result.Error)
			return
		}

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			lets.LogI("Migrating: %s", name)

			filePath := fmt.Sprintf("migrations/%s", file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				lets.LogE("Unable to run migration: %s", err.Error())
				return
			}

			err = g.Transaction(func(tx *gorm.DB) error {
				for _, query := range strings.Split(string(content), ";") {
					query := strings.TrimSpace(query)
					if query == "" {
						continue
					}

					result = g.Exec(query)
					if result.Error != nil {
						return result.Error
					}
				}

				return nil
			})

			if err != nil {
				lets.LogE("Unable to run migration %w", err.Error())
				return
			}

			m := &migration{Migration: name, Batch: batch}
			result = g.Create(m)
			if result.Error != nil {
				lets.LogE("Unable to run migration: %s", result.Error.Error())
				return
			}
		}
	}
}
