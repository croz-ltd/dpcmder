package ui

import (
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/logging"
)

func Start() {
	logging.LogDebug("ui/Start()")

	out.Init()
	defer out.Stop()
	InitialLoad()
	StartReadingKeys()
	logging.LogDebug("ui/Start() end")
}

func Stop() {
	logging.LogDebug("ui/Stop()")
	out.Stop()
	logging.LogDebug("ui/Stop() end")
}
