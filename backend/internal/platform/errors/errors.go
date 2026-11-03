package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound                   = errors.New("resource not found")
	ErrUnauthorized               = errors.New("unauthorized")
	ErrForbidden                  = errors.New("forbidden: insufficient permissions")
	ErrConflict                   = errors.New("resource conflict")
	ErrValidation                 = errors.New("validation failed")
	ErrParentDueExceeded          = errors.New("child due date cannot exceed parent due date")
	ErrParentStartExceeded        = errors.New("child start date cannot precede parent start date")
	ErrCyclicDependency           = errors.New("cyclic dependency detected in task hierarchy")
	ErrTaskCompleted              = errors.New("cannot create child task or modify completed task")
	ErrReviewIncomplete           = errors.New("cannot approve task while required child tasks remain incomplete")
	ErrReopenUnauthorized         = errors.New("only the latest reviewer or higher manager may reopen this task")
	ErrCannotDeleteDispatchedTask = errors.New("cannot delete dispatched task; only drafts may be deleted")
	ErrTaskNotDraft               = errors.New("task is not in draft status")
	ErrTaskAlreadyAssigned        = errors.New("task is already assigned")
	ErrInvalidTaskState           = errors.New("invalid task state for this operation")
)

type AppError struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	HTTPStatus int         `json:"-"`
	Details    interface{} `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewAppError(httpStatus int, code, message string, details interface{}) *AppError {
	return &AppError{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    message,
		Details:    details,
	}
}
