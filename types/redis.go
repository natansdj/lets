package types

import (
	"fmt"

	"github.com/natansdj/lets"
)

const (
	REDIS_HOST     = "localhost"
	REDIS_PORT     = "6379"
	REDIS_USERNAME = ""
	REDIS_PASSWORD = ""
	REDIS_DATABASE = 0
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
