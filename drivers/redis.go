package drivers

import (
	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"

	"github.com/go-redis/redis/v8"
)

var RedisConfig types.IRedis

type redisProvider struct {
	dsn      string
	username string
	password string
	database int
	redis    *redis.Client
}

func (m *redisProvider) Connect() {
	m.redis = redis.NewClient(&redis.Options{
		Addr:     m.dsn,
		Username: m.username,
		Password: m.password,
		DB:       m.database,
	})
}

func (m *redisProvider) Disconnect() {
	lets.LogI("SqLite Stopping ...")
	err := m.redis.Close()
	if err != nil {
		lets.LogErr(err)
		return
	}
	lets.LogI("SqLite Stopped ...")
}

// Define MySQL service host and port
func Redis() (disconnectors []func()) {
	if RedisConfig == nil {
		return
	}

	lets.LogI("Redis Client Starting ...")

	redis := redisProvider{
		dsn:      RedisConfig.GetDsn(),
		username: RedisConfig.GetUsername(),
		password: RedisConfig.GetPassword(),
		database: RedisConfig.GetDatabase(),
	}
	redis.Connect()
	disconnectors = append(disconnectors, redis.Disconnect)

	// Inject Gorm into repository
	for _, repository := range RedisConfig.GetRepositories() {
		repository.SetDriver(redis.redis)
	}
	return
}
