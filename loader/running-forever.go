package loader

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/natansdj/lets"
)

// List of stop function
var Stopper = []func(){}

// Hold the thread for exitting
func WaitForExitSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)
	sigTrigger := <-sig

	lets.LogD("Shutdown signal recived [%s], running close procedures.", sigTrigger.String())

	OnShutdown()

	os.Exit(0)
}

func OnShutdown() {
	for _, stop := range Stopper {
		stop()
	}
}
