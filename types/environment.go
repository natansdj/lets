package types

type IEnvironment interface {
	GetName() string
	GetDebug() string
}

// ISystemInfo provides configuration for system information display
type ISystemInfo interface {
	// EnableNetworkInfo controls whether to collect and display network information
	EnableNetworkInfo() bool
	// EnableReplicaInfo controls whether to collect and display replica information
	EnableReplicaInfo() bool
	// EnableStartupBanner controls whether to display the startup banner
	EnableStartupBanner() bool
	// EnableVerboseNetworkInfo controls whether to display verbose network details
	EnableVerboseNetworkInfo() bool
}

// Serve information
type Environment struct {
	Name  string
	Debug string
}

func (e *Environment) GetName() string {
	return e.Name
}

func (e *Environment) GetDebug() string {
	return e.Debug
}
