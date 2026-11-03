package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Task struct {
	BaseModel
	OrganizationID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
	ParentID         *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	RootTaskID       *uuid.UUID `gorm:"type:uuid;index" json:"root_task_id,omitempty"`
	Code             string     `gorm:"size:50;not null;index" json:"code"`
	Title            string     `gorm:"size:255;not null" json:"title"`
	Description      string     `gorm:"type:text" json:"description,omitempty"`
	Status           string     `gorm:"size:30;not null;index;default:assigned" json:"status"`
	Priority         int16      `gorm:"not null;default:2" json:"priority"`
	Progress         int16      `gorm:"not null;default:0" json:"progress"`
	StartAt          *time.Time `json:"start_at,omitempty"`
	DueAt            *time.Time `gorm:"index" json:"due_at,omitempty"`
	CreatedBy        uuid.UUID  `gorm:"type:uuid;not null;index" json:"created_by"`
	ApprovalRequired bool       `gorm:"not null;default:true" json:"approval_required"`
	Weight           int32      `gorm:"not null;default:1" json:"weight"`
	Version          int64      `gorm:"not null;default:1" json:"version"`
	SortOrder        int        `gorm:"not null;default:0" json:"sort_order"`
	BlockedReason    string     `gorm:"type:text" json:"blocked_reason,omitempty"`
	IsAssigned       bool       `gorm:"not null;default:false;index" json:"is_assigned"`

	Parent      *Task            `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children    []Task           `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Assignments []TaskAssignment `gorm:"foreignKey:TaskID" json:"assignments,omitempty"`
	Updates     []TaskUpdate     `gorm:"foreignKey:TaskID" json:"updates,omitempty"`
	Reviews     []TaskReview     `gorm:"foreignKey:TaskID" json:"reviews,omitempty"`
	Attachments []TaskAttachment `gorm:"foreignKey:TaskID" json:"attachments,omitempty"`
}

type TaskAssignment struct {
	BaseModel
	OrganizationID       uuid.UUID   `gorm:"type:uuid;not null;index" json:"organization_id"`
	TaskID               uuid.UUID   `gorm:"type:uuid;not null;index" json:"task_id"`
	AssigneeUserID       *uuid.UUID  `gorm:"type:uuid;index" json:"assignee_user_id,omitempty"`
	AssigneeDepartmentID *uuid.UUID  `gorm:"type:uuid;index" json:"assignee_department_id,omitempty"`
	ResponsibleUserID    *uuid.UUID  `gorm:"type:uuid;index" json:"responsible_user_id,omitempty"`
	IsPrimary            bool        `gorm:"not null;default:false" json:"is_primary"`
	AssignmentRole       string      `gorm:"size:30;not null;default:responsible" json:"assignment_role"`
	AssignedBy           uuid.UUID   `gorm:"type:uuid;not null" json:"assigned_by"`
	Note                 string      `gorm:"type:text" json:"note,omitempty"`
	AssigneeUser         *User       `gorm:"foreignKey:AssigneeUserID" json:"assignee_user,omitempty"`
	AssigneeDepartment   *Department `gorm:"foreignKey:AssigneeDepartmentID" json:"assignee_department,omitempty"`
	ResponsibleUser      *User       `gorm:"foreignKey:ResponsibleUserID" json:"responsible_user,omitempty"`
}

type TaskUpdate struct {
	BaseModel
	TaskID      uuid.UUID        `gorm:"type:uuid;not null;index" json:"task_id"`
	SubmittedBy uuid.UUID        `gorm:"type:uuid;not null" json:"submitted_by"`
	Progress    int16            `gorm:"not null" json:"progress"`
	Content     string           `gorm:"type:text;not null" json:"content"`
	SubmittedAt time.Time        `gorm:"not null" json:"submitted_at"`
	Submitter   *User            `gorm:"foreignKey:SubmittedBy" json:"submitter,omitempty"`
	Attachments []TaskAttachment `gorm:"foreignKey:UpdateID" json:"attachments,omitempty"`
}

type TaskReview struct {
	BaseModel
	TaskID     uuid.UUID   `gorm:"type:uuid;not null;index" json:"task_id"`
	UpdateID   uuid.UUID   `gorm:"type:uuid;not null" json:"update_id"`
	ReviewerID uuid.UUID   `gorm:"type:uuid;not null" json:"reviewer_id"`
	Decision   string      `gorm:"size:30;not null" json:"decision"`
	Comment    string      `gorm:"type:text" json:"comment,omitempty"`
	ReviewedAt time.Time   `gorm:"not null" json:"reviewed_at"`
	Reviewer   *User       `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
	Update     *TaskUpdate `gorm:"foreignKey:UpdateID" json:"update,omitempty"`
}

type TaskComment struct {
	BaseModel
	TaskID   uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	AuthorID uuid.UUID `gorm:"type:uuid;not null" json:"author_id"`
	Content  string    `gorm:"type:text;not null" json:"content"`
	Author   *User     `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
}

type TaskEvent struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	TaskID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"task_id"`
	ActorID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"actor_id"`
	EventType      string         `gorm:"size:50;not null" json:"event_type"`
	Data           datatypes.JSON `gorm:"type:jsonb;not null" json:"data"`
	CreatedAt      time.Time      `gorm:"not null;index" json:"created_at"`
	Actor          *User          `gorm:"foreignKey:ActorID" json:"actor,omitempty"`
}

type TaskAttachment struct {
	BaseModel
	TaskID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"task_id"`
	UpdateID    *uuid.UUID `gorm:"type:uuid;index" json:"update_id,omitempty"`
	UploadedBy  uuid.UUID  `gorm:"type:uuid;not null" json:"uploaded_by"`
	FileName    string     `gorm:"size:255;not null" json:"file_name"`
	ObjectKey   string     `gorm:"size:1024;not null" json:"-"`
	ContentType string     `gorm:"size:150" json:"content_type,omitempty"`
	SizeBytes   int64      `gorm:"not null" json:"size_bytes"`
	Checksum    string     `gorm:"size:128" json:"checksum,omitempty"`
	Status      string     `gorm:"size:20;not null;default:pending" json:"status"` // pending -> active
	Uploader    *User      `gorm:"foreignKey:UploadedBy" json:"uploader,omitempty"`
}

