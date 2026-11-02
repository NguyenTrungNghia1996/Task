package model

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	BaseModel
	Code     string `gorm:"size:50;not null;uniqueIndex" json:"code"`
	Name     string `gorm:"size:255;not null" json:"name"`
	IsActive bool   `gorm:"not null;default:true" json:"is_active"`
}

type Department struct {
	BaseModel
	OrganizationID uuid.UUID   `gorm:"type:uuid;not null;index" json:"organization_id"`
	ParentID       *uuid.UUID  `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Code           string      `gorm:"size:50;not null" json:"code"`
	Name           string      `gorm:"size:255;not null" json:"name"`
	DepartmentType string      `gorm:"size:30;not null" json:"department_type"`
	IsActive       bool        `gorm:"not null;default:true" json:"is_active"`
	Children       []Department `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

type User struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index" json:"organization_id"`
	Username       string    `gorm:"size:100;not null" json:"username"`
	Email          string    `gorm:"size:255" json:"email,omitempty"`
	PasswordHash   string    `gorm:"size:255;not null" json:"-"`
	FullName       string    `gorm:"size:255;not null" json:"full_name"`
	IsActive       bool      `gorm:"not null;default:true" json:"is_active"`
}

type Role struct {
	BaseModel
	Code        string `gorm:"size:50;not null;uniqueIndex" json:"code"`
	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `gorm:"type:text" json:"description,omitempty"`
}

type UserRole struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index" json:"organization_id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	RoleID         uuid.UUID `gorm:"type:uuid;not null;index" json:"role_id"`
	Role           *Role     `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

type DepartmentMember struct {
	BaseModel
	OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
	DepartmentID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"department_id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	ManagerUserID  *uuid.UUID `gorm:"type:uuid;index" json:"manager_user_id,omitempty"`
	PositionTitle  string     `gorm:"size:100" json:"position_title,omitempty"`
	IsPrimary      bool       `gorm:"not null;default:false" json:"is_primary"`
	EffectiveFrom  time.Time  `gorm:"not null" json:"effective_from"`
	EffectiveTo    *time.Time `json:"effective_to,omitempty"`
	User           *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Department     *Department `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
}

