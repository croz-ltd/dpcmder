package main

import (
	"github.com/croz-ltd/dpcmder/config"
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/repo/dp"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view"
)

func main() {
	config.Init()
	config.PrintConfig()
	dp.InitNetworkSettings()
	model.M.SetDpDomain(*config.DpDomain)
	if *config.DpUsername != "" {
		model.M.SetDpAppliance(config.PreviousAppliance)
	}

	view.Init()
	// model.M.Print()
	logging.LogDebug("...dpcmder ending.")
}
