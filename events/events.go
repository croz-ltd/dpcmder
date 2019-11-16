package events

import (
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/ui/in/key"
)

type KeyPressedEvent struct {
	KeyCode key.KeyCode
}

type UpdateViewEventType int

const (
	UpdateViewScreenSize UpdateViewEventType = UpdateViewEventType(0)
	UpdateViewRefresh    UpdateViewEventType = UpdateViewEventType(1)
	UpdateViewShowDialog UpdateViewEventType = UpdateViewEventType(2)
	UpdateViewShowStatus UpdateViewEventType = UpdateViewEventType(3)
	UpdateViewQuit       UpdateViewEventType = UpdateViewEventType(99)
)

type UpdateViewEvent struct {
	Type                  UpdateViewEventType
	Model                 *model.Model
	DialogQuestion        string
	DialogAnswer          string
	DialogAnswerCursorIdx int
	Status                string
}
