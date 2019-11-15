package view

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in"
	"github.com/croz-ltd/dpcmder/view/out"
)

func Init(updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("view/Init()")
	out.Init(updateViewEventChan)
}

func Start(keyPressedEventChan chan events.KeyPressedEvent) {
	logging.LogDebug("view/Start()")

	defer out.Stop()
	in.Start(keyPressedEventChan)
	logging.LogDebug("view/Start() end")
}

func Stop() {
	logging.LogDebug("view/Stop()")
	out.Stop()
	logging.LogDebug("view/Stop() end")
}
