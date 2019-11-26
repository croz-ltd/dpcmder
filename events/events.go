package events

import (
	"github.com/croz-ltd/dpcmder/model"
)

// UpdateViewEventType is used for sending different messages to view to update.
type UpdateViewEventType int

// All available event types for updating screen.
const (
	UpdateViewRefresh    UpdateViewEventType = UpdateViewEventType(1)
	UpdateViewShowDialog UpdateViewEventType = UpdateViewEventType(2)
	UpdateViewShowStatus UpdateViewEventType = UpdateViewEventType(3)
)

// UpdateViewEvent contains information neccessary for all types of screen
// update events.
type UpdateViewEvent struct {
	Type                  UpdateViewEventType
	Model                 *model.Model
	DialogQuestion        string
	DialogAnswer          string
	DialogAnswerCursorIdx int
	Status                string
}
