package ui

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/ui/in"
	"github.com/croz-ltd/dpcmder/ui/out"
	"github.com/croz-ltd/dpcmder/utils/logging"
)

func Init(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("ui/Init()")
	out.Init(updateViewEventChan)
}

func Start(keyPressedEventChan chan events.KeyPressedEvent) {
	logging.LogDebug("ui/Start()")

	defer out.Stop()
	in.Start(keyPressedEventChan)
	logging.LogDebug("ui/Start() end")
}

func Stop() {
	logging.LogDebug("ui/Stop()")
	out.Stop()
	logging.LogDebug("ui/Stop() end")
}
