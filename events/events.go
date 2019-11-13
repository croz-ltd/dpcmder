package events

import (
	"github.com/croz-ltd/dpcmder/model"
	"github.com/croz-ltd/dpcmder/view/in/key"
)

type KeyPressedEvent struct {
	KeyCode key.KeyCode
}

type UpdateViewEventType int

const (
	UpdateViewScreenSize UpdateViewEventType = UpdateViewEventType(0)
	UpdateViewRefresh    UpdateViewEventType = UpdateViewEventType(1)
	UpdateViewShowDialog UpdateViewEventType = UpdateViewEventType(2)
)

type UpdateViewEvent struct {
	Type           UpdateViewEventType
	Model          *model.Model
	DialogQuestion string
}

type ActionEvent struct {
}
