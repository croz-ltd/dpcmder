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
)

type UpdateViewEvent struct {
	Type  UpdateViewEventType
	Model model.Model
}

type ActionEvent struct {
}
