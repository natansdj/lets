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
}

func (m *mongodbProvider) Connect() {
	clientOptions := options.Client()
	clientOptions.ApplyURI(m.dsn)

	var err error
	m.mongodb, err = mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		lets.LogE("MongoDB: %v", err)
		return
	}

	m.DB = m.mongodb.Database(m.database)
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

// Define MySQL service host and port
func MongoDB() (disconnectors []func()) {
	if MongoDBConfig == nil {
		return
	}

	lets.LogI("MongoDB Client Starting ...")

	mongodb := mongodbProvider{
		dsn:      MongoDBConfig.GetDsn(),
		database: MongoDBConfig.GetDatabase(),
	}
	mongodb.Connect()
	disconnectors = append(disconnectors, mongodb.Disconnect)

	// Inject Gorm into repository
	for _, repository := range MongoDBConfig.GetRepositories() {
		repository.SetDriver(mongodb.DB)
	}
 return
}
