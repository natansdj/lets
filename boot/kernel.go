package boot

import (
	"github.com/natansdj/lets/drivers"
	"github.com/natansdj/lets/frameworks"
	"github.com/natansdj/lets/loader"
)

// List of initializer
var Initializer = []func(){
	loader.Environment,
	loader.Logger,
	loader.Network,
	loader.Replica,
}

// List of framework that start on lets
var Servers = []func() []func(){
	drivers.MySQL,
	drivers.SqLiteClient,
	drivers.Redis,
	drivers.MongoDB,
	frameworks.Grpc,
	frameworks.RabbitMQ,
	frameworks.Tcp,
	frameworks.Http,
	frameworks.WebSocket,
}

// Add initialization function and run before application starting
func AddInitializer(init func()) {
	Initializer = append(Initializer, init)
}

// Add initialization function and run before application starting
func AddServers(server func() (disconectors []func())) {
	Servers = append(Servers, server)
}

func AddStopper(stopper func()) {
	loader.Stopper = append(loader.Stopper, stopper)
}

// Bootstrap vars and configuration
func OnInit() {
	Initializer = append(Initializer, loader.Launching)
	for _, initializer := range Initializer {
		initializer()
	}
}

// Bootstrap frameworks
func OnMain(waiter ...chan<- int) {
	for _, runner := range Servers {
		stopper := runner()

		for _, stopFunc := range stopper {
			loader.Stopper = append(loader.Stopper, stopFunc)
		}
	}

	for _, wait := range waiter {
		wait <- int(1)
	}

	loader.WaitForExitSignal()
}
