package main

import (
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view"
	"github.com/nsf/termbox-go"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	config.Init()
	config.PrintConfig()
	dp.InitNetworkSettings()
	model.M.SetDpDomain(*config.DpDomain)
	if *config.DpUsername != "" {
		model.M.SetDpAppliance(config.PreviousAppliance)
	}

	setupCloseHandler()

	view.Init()
	// model.M.Print()
	logging.LogDebug("...dpcmder ending.")
}

// SetupCloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the OS - so we can cleanup/close termbox.
func setupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	signal.Notify(c, os.Interrupt, syscall.SIGKILL)
	signal.Notify(c, os.Interrupt, syscall.SIGINT)
	signal.Notify(c, os.Interrupt, syscall.SIGHUP)
	go func() {
		s := <-c
		logging.LogDebug("System interrupt signal received, dpcmder ending. s: ", s)
		termbox.Close()
		os.Exit(0)
	}()
}
