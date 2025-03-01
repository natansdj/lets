package drivers

import (
	"database/sql"
	"sync"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var SqLiteConfig *[]*types.SqLite

type SqLiteProvider struct {
	Config *types.SqLite
	// debug bool
	DSN  string
	Gorm *gorm.DB
	Sql  *sql.DB
	Mu   sync.RWMutex
}

func (m *SqLiteProvider) Connect() {
	var logType logger.Interface = logger.Default.LogMode(logger.Warn)
	if m.Config.Debug {
		logType = logger.Default.LogMode(logger.Info)
	}

	var err error
	// _, err = os.Stat(m.DSN)
	// if err == nil {
	// 	return
	// }

	// if os.IsNotExist(err) {
	// 	if _, err = os.Create(m.DSN); err != nil {
	// 		panic(err)
	// 	}
	// }

	m.Gorm, err = gorm.Open(sqlite.Open(m.DSN), &gorm.Config{
		Logger:      logType,
		QueryFields: m.Config.QueryFields,
		NamingStrategy: schema.NamingStrategy{
			NoLowerCase:   true,
			SingularTable: true,
		},
		PrepareStmt:              false,
		DisableNestedTransaction: m.Config.DisableNestedTransaction,
	})

	if err != nil {
		lets.LogE("%s", err.Error())
		return
	}

	m.Sql, err = m.Gorm.DB()
	if err != nil {
		lets.LogE(err.Error())
		return
	}

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	m.Sql.SetMaxIdleConns(10)

	// SetMaxOpenConns sets the maximum number of open connections to the database.
	m.Sql.SetMaxOpenConns(100)

	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	m.Sql.SetConnMaxLifetime(time.Hour)
}

func (m *SqLiteProvider) Disconnect() {
	lets.LogI("SqLite Stopping ...")
	err := m.Sql.Close()
	if err != nil {
		lets.LogE(err.Error())
		return
	}
	lets.LogI("SqLite Stopped ...")
}

// Define MySQL service host and port
func SqLiteClient() (disconnectors []func()) {
	if SqLiteConfig == nil {
		return
	} else if len(*SqLiteConfig) == 0 {
		return
	}

	lets.LogI("SqLite Starting ...")

	for _, config := range *SqLiteConfig {
		sqlLite := SqLiteProvider{
			Config: config,
		}
		sqlLite.DSN = "storage/db/" + config.DBPath
		sqlLite.Connect()
		disconnectors = append(disconnectors, sqlLite.Disconnect)

		// Inject Gorm into repository
		for _, repository := range config.Repositories {
			repository.SetDriver(sqlLite.Gorm, &sqlLite.Mu)
		}

		// Migration
		if config.EnableMigration {
			err := sqlLite.Gorm.AutoMigrate(&migration{})
			if err != nil {
				lets.LogE("Unable to run migration %w", err)
				return
			}
			Migrate(sqlLite.Gorm, sqlLite.Sql)
		}
	}

	return
}

// type migration struct {
// 	ID        uint   `gorm:"primaryKey,column:id"`
// 	Migration string `gorm:"column:migration"`
// 	Batch     uint   `gorm:"column:batch"`
// }

// func Migrate(g *gorm.DB, db *sql.DB) {
// 	// Define batch number
// 	var batch uint = 1
// 	lastMigration := &migration{}
// 	result := g.Last(lastMigration)
// 	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 		lets.LogE("Unable to run migration %w", result.Error)
// 		return
// 	}

// 	batch = lastMigration.Batch + 1

// 	// Get migration files
// 	files, err := os.ReadDir("migrations")
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	for _, file := range files {
// 		name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

// 		// Search migration
// 		search := &migration{
// 			Migration: name,
// 		}

// 		result := g.Where("migration = ?", name).First(search)
// 		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 			lets.LogE("Unable to run migration %w", result.Error)
// 			return
// 		}

// 		// Execute
// 		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 			lets.LogI("Migrating: %s", name)

// 			// Read file content
// 			filePath := fmt.Sprintf("migrations/%s", file.Name())
// 			content, err := os.ReadFile(filePath)
// 			if err != nil {
// 				lets.LogE("Unable to run migration: %s", err.Error())
// 				return
// 			}

// 			err = g.Transaction(func(tx *gorm.DB) error {
// 				for _, query := range strings.Split(string(content), ";") {
// 					query := strings.TrimSpace(query)
// 					if query == "" {
// 						continue
// 					}

// 					result = g.Exec(query)
// 					if result.Error != nil {
// 						return result.Error
// 					}
// 				}

// 				return nil
// 			})

// 			if err != nil {
// 				lets.LogE("Unable to run migration %w", err.Error())
// 				return
// 			}

// 			// Insert migration record
// 			m := &migration{
// 				Migration: name,
// 				Batch:     batch,
// 			}

// 			result = g.Create(m)
// 			if result.Error != nil {
// 				lets.LogE("Unable to run migration: %s", result.Error.Error())
// 				return
// 			}
// 		}

// 	}
// }
