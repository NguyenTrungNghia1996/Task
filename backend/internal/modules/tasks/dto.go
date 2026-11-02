package tasks

import (
	"time"

	"github.com/google/uuid"
)

type AssignmentInput struct {
	Type              string     `json:"type" validate:"required,oneof=user department"`
	ID                uuid.UUID  `json:"id" validate:"required"`
	ResponsibleUserID *uuid.UUID `json:"responsible_user_id,omitempty"`
}

type CreateTaskRequest struct {
	Title             string          `json:"title" validate:"required,max=255"`
	Description       string          `json:"description"`
	Priority          int16           `json:"priority" validate:"gte=1,lte=3"`
	StartAt           *time.Time      `json:"start_at"`
	DueAt             *time.Time      `json:"due_at"`
	ApprovalRequired  *bool           `json:"approval_required"`
	InitialAssignment AssignmentInput `json:"initial_assignment" validate:"required"`
}

type CreateChildTaskRequest struct {
	Title             string          `json:"title" validate:"required,max=255"`
	Description       string          `json:"description"`
	Priority          int16           `json:"priority" validate:"gte=1,lte=3"`
	StartAt           *time.Time      `json:"start_at"`
	DueAt             *time.Time      `json:"due_at"`
	Weight            int32           `json:"weight" validate:"gte=1"`
	ApprovalRequired  *bool           `json:"approval_required"`
	InitialAssignment AssignmentInput `json:"initial_assignment" validate:"required"`
}

type UpdateTaskRequest struct {
	Title       *string    `json:"title,omitempty" validate:"omitempty,max=255"`
	Description *string    `json:"description,omitempty"`
	Priority    *int16     `json:"priority,omitempty" validate:"omitempty,gte=1,lte=3"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	Version     int64      `json:"version" validate:"required,gte=1"` // Optimistic Locking
}

type CreateTaskUpdateRequest struct {
	Progress      int16       `json:"progress" validate:"gte=0,lte=100"`
	Content       string      `json:"content" validate:"required"`
	AttachmentIDs []uuid.UUID `json:"attachment_ids"`
}

type CreateTaskReviewRequest struct {
	UpdateID uuid.UUID `json:"update_id" validate:"required"`
	Decision string    `json:"decision" validate:"required,oneof=approved revision_required"`
	Comment  string    `json:"comment"`
}

type ReopenTaskRequest struct {
	Reason string `json:"reason" validate:"required,min=5,max=2000"`
}

type CancelTaskRequest struct {
	Reason string `json:"reason" validate:"required,min=5,max=2000"`
}

type CreateCommentRequest struct {
	Content string `json:"content" validate:"required"`
}

type PresignUploadRequest struct {
	FileName    string `json:"file_name" validate:"required,max=255"`
	ContentType string `json:"content_type" validate:"required"`
	SizeBytes   int64  `json:"size_bytes" validate:"required,gt=0"`
}

type PresignUploadResponse struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
	UploadURL    string    `json:"upload_url"`
	ObjectKey    string    `json:"object_key"`
	ExpiresInSec int       `json:"expires_in_sec"`
}

type ConfirmUploadRequest struct {
	AttachmentID uuid.UUID `json:"attachment_id" validate:"required"`
	Checksum     string    `json:"checksum,omitempty"`
}

type TaskFilter struct {
	Page         int        `query:"page"`
	PageSize     int        `query:"page_size"`
	Query        string     `query:"q"`
	Status       string     `query:"status"`
	Priority     int16      `query:"priority"`
	AssigneeID   *uuid.UUID `query:"assignee_id"`
	DepartmentID *uuid.UUID `query:"department_id"`
	ParentID     *uuid.UUID `query:"parent_id"`
	RootTaskID   *uuid.UUID `query:"root_task_id"`
	DueBefore    *time.Time `query:"due_before"`
	DueAfter     *time.Time `query:"due_after"`
	Overdue      bool       `query:"overdue"`
	SortBy       string     `query:"sort_by"`
	SortOrder    string     `query:"sort_order"`
}

type CreateAssignmentRequest struct {
	Type              string     `json:"type" validate:"required,oneof=user department"`
	ID                uuid.UUID  `json:"id" validate:"required"`
	ResponsibleUserID *uuid.UUID `json:"responsible_user_id,omitempty"`
	AssignmentRole    string     `json:"assignment_role" validate:"omitempty,oneof=responsible supporting observer"`
	IsPrimary         bool       `json:"is_primary"`
	Note              string     `json:"note,omitempty"`
}


