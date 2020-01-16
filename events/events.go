// Package events contains model for events causing screen update.
package events

import (
	"github.com/croz-ltd/dpcmder/model"
)

// UpdateViewEventType is used for sending different messages to view to update.
type UpdateViewEventType int

// All available event types for updating screen.
const (
	UpdateViewRefresh                 UpdateViewEventType = UpdateViewEventType(1)
	UpdateViewShowQuestionDialog      UpdateViewEventType = UpdateViewEventType(2)
	UpdateViewShowListSelectionDialog UpdateViewEventType = UpdateViewEventType(3)
	UpdateViewShowStatus              UpdateViewEventType = UpdateViewEventType(4)
	UpdateViewShowProgress            UpdateViewEventType = UpdateViewEventType(5)
)

// UpdateViewEvent contains information neccessary for all types of screen
// update events.
type UpdateViewEvent struct {
	Type                     UpdateViewEventType
	Model                    *model.Model
	DialogQuestion           string
	DialogAnswer             string
	DialogAnswerCursorIdx    int
	Status                   string
	Message                  string
	Progress                 int
	ListSelectionMessage     string
	ListSelectionList        []string
	ListSelectionSelectedIdx int
}
