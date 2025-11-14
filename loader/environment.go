package loader

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/natansdj/lets/types"
)

var EnvFile string
var SystemInfoConfig types.ISystemInfo

// SystemInfo provides default configuration for system information display
type SystemInfo struct {
	networkInfo        bool
	replicaInfo        bool
	startupBanner      bool
	verboseNetworkInfo bool
}

// NewSystemInfoFromEnv creates SystemInfo from environment variables
func NewSystemInfoFromEnv() *SystemInfo {
	return &SystemInfo{
		networkInfo:        getEnvBool("LETS_ENABLE_NETWORK_INFO", true),
		replicaInfo:        getEnvBool("LETS_ENABLE_REPLICA_INFO", true),
		startupBanner:      getEnvBool("LETS_ENABLE_STARTUP_BANNER", true),
		verboseNetworkInfo: getEnvBool("LETS_VERBOSE_NETWORK_INFO", false),
	}
}

func (s *SystemInfo) EnableNetworkInfo() bool        { return s.networkInfo }
func (s *SystemInfo) EnableReplicaInfo() bool        { return s.replicaInfo }
func (s *SystemInfo) EnableStartupBanner() bool      { return s.startupBanner }
func (s *SystemInfo) EnableVerboseNetworkInfo() bool { return s.verboseNetworkInfo }

// getEnvBool retrieves boolean from environment variable with default fallback
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return result
}

// Loading .env environment variable into memory.
func Environment() {
	if EnvFile != "" {
		err := godotenv.Load(EnvFile)
		if err != nil {
			log.Fatalln("Error loading .env file")
		}
	} else {
		err := godotenv.Load()
		if err != nil {
			log.Fatalln("Error loading .env file")
		}
	}

	// Initialize SystemInfoConfig if not already set
	if SystemInfoConfig == nil {
		SystemInfoConfig = NewSystemInfoFromEnv()
	}
}
