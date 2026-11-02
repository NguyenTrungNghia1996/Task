package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type RefreshToken struct {
	BaseModel
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	User      *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

type IdempotencyKey struct {
	Key            string         `gorm:"size:100;primaryKey" json:"key"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	RequestMethod  string         `gorm:"size:10;not null" json:"request_method"`
	RequestPath    string         `gorm:"size:255;not null" json:"request_path"`
	ResponseCode   int            `gorm:"not null" json:"response_code"`
	ResponseBody   datatypes.JSON `gorm:"type:jsonb;not null" json:"response_body"`
	CreatedAt      time.Time      `gorm:"not null" json:"created_at"`
	ExpiresAt      time.Time      `gorm:"not null;index" json:"expires_at"`
}

type Notification struct {
	BaseModel
	OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
	RecipientID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"recipient_id"`
	TaskID         *uuid.UUID `gorm:"type:uuid;index" json:"task_id,omitempty"`
	Type           string     `gorm:"size:50;not null" json:"type"`
	Title          string     `gorm:"size:255;not null" json:"title"`
	Body           string     `gorm:"type:text" json:"body,omitempty"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	Recipient      *User      `gorm:"foreignKey:RecipientID" json:"recipient,omitempty"`
	Task           *Task      `gorm:"foreignKey:TaskID" json:"task,omitempty"`
}

