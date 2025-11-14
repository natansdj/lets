package drivers

import (
	"context"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var MongoDBConfig types.IMongoDB

type mongodbProvider struct {
	dsn      string
	database string
	mongodb  *mongo.Client
	DB       *mongo.Database
	config   types.IMongoDB
}

func (m *mongodbProvider) Connect() {
	m.connectWithRetry(0)
}

func (m *mongodbProvider) connectWithRetry(attempt int) {
	maxRetries := m.config.GetMaxRetries()
	if attempt >= maxRetries {
		lets.LogF("MongoDB: Max connection retries (%d) exceeded", maxRetries)
		return
	}

	clientOptions := options.Client()
	clientOptions.ApplyURI(m.dsn)

	// Connection pool settings
	maxPoolSize := m.config.GetMaxPoolSize()
	minPoolSize := m.config.GetMinPoolSize()
	maxConnIdleTime := time.Duration(m.config.GetMaxConnIdleTime()) * time.Second

	clientOptions.SetMaxPoolSize(maxPoolSize)
	clientOptions.SetMinPoolSize(minPoolSize)
	clientOptions.SetMaxConnIdleTime(maxConnIdleTime)

	// Timeout settings
	clientOptions.SetConnectTimeout(time.Duration(m.config.GetConnectTimeout()) * time.Second)
	clientOptions.SetServerSelectionTimeout(time.Duration(m.config.GetServerSelectionTimeout()) * time.Second)
	clientOptions.SetSocketTimeout(time.Duration(m.config.GetSocketTimeout()) * time.Second)

	// Retry settings
	clientOptions.SetRetryWrites(true)
	clientOptions.SetRetryReads(true)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(m.config.GetConnectTimeout())*time.Second)
	defer cancel()

	var err error
	m.mongodb, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		// Calculate exponential backoff
		backoff := time.Second * time.Duration(1<<uint(attempt))
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		lets.LogERL("mongodb-connect-retry", "MongoDB connection attempt %d failed: %v. Retrying in %v...", attempt+1, err, backoff)
		time.Sleep(backoff)
		m.connectWithRetry(attempt + 1)
		return
	}

	// Verify connection with ping
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if err = m.mongodb.Ping(pingCtx, nil); err != nil {
		// Calculate exponential backoff
		backoff := time.Second * time.Duration(1<<uint(attempt))
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		lets.LogERL("mongodb-ping-retry", "MongoDB ping attempt %d failed: %v. Retrying in %v...", attempt+1, err, backoff)
		time.Sleep(backoff)
		m.connectWithRetry(attempt + 1)
		return
	}

	m.DB = m.mongodb.Database(m.database)
	lets.LogI("MongoDB Client Connected (Pool: %d-%d, IdleTime: %v)", minPoolSize, maxPoolSize, maxConnIdleTime)
}

func (m *mongodbProvider) Disconnect() {
	lets.LogI("MongoDB Stopping ...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	err := m.mongodb.Disconnect(ctx)
	if err != nil {
		lets.LogErr(err)
		return
	}
	lets.LogI("MongoDB Stopped ...")
}

// MongoDB initializes and returns MongoDB client with connection pooling and retry logic
func MongoDB() (disconnectors []func()) {
	if MongoDBConfig == nil {
		return
	}

	lets.LogI("MongoDB Client Starting ...")

	mongodb := mongodbProvider{
		dsn:      MongoDBConfig.GetDsn(),
		database: MongoDBConfig.GetDatabase(),
		config:   MongoDBConfig,
	}
	mongodb.Connect()
	disconnectors = append(disconnectors, mongodb.Disconnect)

	// Inject MongoDB database into repository
	for _, repository := range MongoDBConfig.GetRepositories() {
		repository.SetDriver(mongodb.DB)
	}
	return
}
