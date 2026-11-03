package departments

import (
	"time"

	"github.com/google/uuid"
)

type DepartmentLeaderDTO struct {
	UserID        uuid.UUID `json:"user_id"`
	FullName      string    `json:"full_name"`
	Username      string    `json:"username"`
	PositionTitle string    `json:"position_title"`
}

type DepartmentTreeNode struct {
	ID             uuid.UUID            `json:"id"`
	Key            string               `json:"key"`   // Ant Design Tree compatibility (ID string)
	Title          string               `json:"title"` // Ant Design Tree compatibility (Name)
	Value          string               `json:"value"` // Ant Design TreeSelect compatibility
	ParentID       *uuid.UUID           `json:"parent_id,omitempty"`
	Code           string               `json:"code"`
	Name           string               `json:"name"`
	DepartmentType string               `json:"department_type"`
	IsActive       bool                 `json:"is_active"`
	MemberCount    int                  `json:"member_count"`
	Leader         *DepartmentLeaderDTO `json:"leader,omitempty"`
	Children       []DepartmentTreeNode `json:"children"`
}

type DepartmentOption struct {
	Value          string     `json:"value"` // UUID as string
	Label          string     `json:"label"` // e.g. "Phòng Kế hoạch tác chiến (PKHTC)"
	Code           string     `json:"code"`
	DepartmentType string     `json:"department_type"`
	ParentID       *uuid.UUID `json:"parent_id,omitempty"`
}

type DepartmentDetailResponse struct {
	ID             uuid.UUID            `json:"id"`
	OrganizationID uuid.UUID            `json:"organization_id"`
	ParentID       *uuid.UUID           `json:"parent_id,omitempty"`
	ParentName     string               `json:"parent_name,omitempty"`
	Code           string               `json:"code"`
	Name           string               `json:"name"`
	DepartmentType string               `json:"department_type"`
	IsActive       bool                 `json:"is_active"`
	MemberCount    int                  `json:"member_count"`
	Leader         *DepartmentLeaderDTO `json:"leader,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type CreateDepartmentRequest struct {
	ParentID       *uuid.UUID `json:"parent_id,omitempty"`
	Code           string     `json:"code" validate:"required,min=2,max=50"`
	Name           string     `json:"name" validate:"required,min=2,max=255"`
	DepartmentType string     `json:"department_type" validate:"required,oneof=division department section team office other"`
	IsActive       *bool      `json:"is_active,omitempty"`
}

type UpdateDepartmentRequest struct {
	ParentID       *uuid.UUID `json:"parent_id,omitempty"`
	Code           *string    `json:"code,omitempty" validate:"omitempty,min=2,max=50"`
	Name           *string    `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	DepartmentType *string    `json:"department_type,omitempty" validate:"omitempty,oneof=division department section team office other"`
	IsActive       *bool      `json:"is_active,omitempty"`
}

type DepartmentMemberDTO struct {
	ID            uuid.UUID  `json:"id"`
	DepartmentID  uuid.UUID  `json:"department_id"`
	UserID        uuid.UUID  `json:"user_id"`
	Username      string     `json:"username"`
	FullName      string     `json:"full_name"`
	Email         string     `json:"email,omitempty"`
	ManagerUserID *uuid.UUID `json:"manager_user_id,omitempty"`
	ManagerName   string     `json:"manager_name,omitempty"`
	PositionTitle string     `json:"position_title"`
	IsPrimary     bool       `json:"is_primary"`
	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
}

type AddDepartmentMemberRequest struct {
	UserID        uuid.UUID  `json:"user_id" validate:"required"`
	PositionTitle string     `json:"position_title" validate:"required,max=100"`
	ManagerUserID *uuid.UUID `json:"manager_user_id,omitempty"`
	IsPrimary     bool       `json:"is_primary"`
	EffectiveFrom *time.Time `json:"effective_from,omitempty"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
}

type UpdateDepartmentMemberRequest struct {
	PositionTitle *string    `json:"position_title,omitempty" validate:"omitempty,max=100"`
	ManagerUserID *uuid.UUID `json:"manager_user_id,omitempty"`
	IsPrimary     *bool      `json:"is_primary,omitempty"`
	EffectiveFrom *time.Time `json:"effective_from,omitempty"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
}
