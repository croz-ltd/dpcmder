package view

import (
	"github.com/croz-ltd/dpcmder/events"
	"github.com/croz-ltd/dpcmder/utils/logging"
	"github.com/croz-ltd/dpcmder/view/in"
	"github.com/croz-ltd/dpcmder/view/out"
)

func Start(keyPressedEventChan chan events.KeyPressedEvent, updateViewEventChan chan events.UpdateViewEvent) {
	logging.LogDebug("view/Start()")

	out.Init(updateViewEventChan)
	defer out.Stop()
	in.Start(keyPressedEventChan)
}

func Stop() {
	out.Stop()
}
