// Package main is entrypoint to DataPower commander (dpcmder) application.
package main

import (
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/ui"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	config.Init()
	config.PrintConfig()

	setupCloseHandler()

	ui.Start()
	logging.LogDebug("main/main() - ...dpcmder ending.")
}

// setupCloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the OS. Since tcell input catch Ctrl+C
// it probably comes from Ctrl+C combination sent to external program - ignoring it.
func setupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	signal.Notify(c, os.Interrupt, syscall.SIGKILL)
	signal.Notify(c, os.Interrupt, syscall.SIGINT)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP)
	go func() {
		s := <-c
		logging.LogDebug("main/setupCloseHandler() - System interrupt signal received from external program, ignoring it - s: ", s)
	}()
}
