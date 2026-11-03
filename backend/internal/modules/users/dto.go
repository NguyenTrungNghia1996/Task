package users

import (
	"time"

	"github.com/google/uuid"
)

type UserFilter struct {
	Page         int        `query:"page"`
	PageSize     int        `query:"page_size"`
	Query        string     `query:"q"`
	DepartmentID *uuid.UUID `query:"department_id"`
	RoleCode     string     `query:"role"`
	IsActive     *bool      `query:"is_active"`
}

type DepartmentInfo struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	PositionTitle string    `json:"position_title"`
	IsPrimary     bool      `json:"is_primary"`
}

type UserListItem struct {
	ID         uuid.UUID       `json:"id"`
	Username   string          `json:"username"`
	FullName   string          `json:"full_name"`
	Email      string          `json:"email,omitempty"`
	IsActive   bool            `json:"is_active"`
	Roles      []string        `json:"roles"`
	Department *DepartmentInfo `json:"department,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type UserDetailResponse struct {
	ID          uuid.UUID        `json:"id"`
	Username    string           `json:"username"`
	FullName    string           `json:"full_name"`
	Email       string           `json:"email,omitempty"`
	IsActive    bool             `json:"is_active"`
	Roles       []string         `json:"roles"`
	Departments []DepartmentInfo `json:"departments"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type CreateUserRequest struct {
	Username      string     `json:"username" validate:"required,min=3,max=50"`
	FullName      string     `json:"full_name" validate:"required,max=255"`
	Email         string     `json:"email" validate:"omitempty,email,max=255"`
	Password      string     `json:"password" validate:"required,min=6,max=100"`
	DepartmentID  *uuid.UUID `json:"department_id,omitempty"`
	PositionTitle string     `json:"position_title,omitempty" validate:"omitempty,max=100"`
	RoleCodes     []string   `json:"role_codes,omitempty"`
}

type UpdateUserRequest struct {
	FullName      *string    `json:"full_name,omitempty" validate:"omitempty,max=255"`
	Email         *string    `json:"email,omitempty" validate:"omitempty,email,max=255"`
	IsActive      *bool      `json:"is_active,omitempty"`
	DepartmentID  *uuid.UUID `json:"department_id,omitempty"`
	PositionTitle *string    `json:"position_title,omitempty" validate:"omitempty,max=100"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required,min=6,max=100"`
}

type SetUserRolesRequest struct {
	RoleCodes []string `json:"role_codes" validate:"required"`
}

type UserOption struct {
	Value          uuid.UUID `json:"value"` // ID dùng cho <a-select>
	Label          string    `json:"label"` // Họ tên (Username)
	Username       string    `json:"username"`
	FullName       string    `json:"full_name"`
	DepartmentName string    `json:"department_name,omitempty"`
	PositionTitle  string    `json:"position_title,omitempty"`
}
