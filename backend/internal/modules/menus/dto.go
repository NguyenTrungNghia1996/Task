package menus

import (
	"github.com/google/uuid"
)

// AntdMenuItem cấu trúc chuẩn tương thích trực tiếp với thuộc tính :items của a-menu trong Ant Design
type AntdMenuItem struct {
	Key      string         `json:"key"`                // vd: "/tasks/all"
	Label    string         `json:"label"`              // vd: "Tất cả công việc"
	Icon     string         `json:"icon,omitempty"`     // vd: "ProjectOutlined"
	Path     string         `json:"path,omitempty"`     // vd: "/tasks/all"
	Children []AntdMenuItem `json:"children,omitempty"` // Menu con
}

type CreateMenuRequest struct {
	ParentID           *uuid.UUID `json:"parent_id,omitempty"`
	Code               string     `json:"code" validate:"required,min=2,max=100"`
	Title              string     `json:"title" validate:"required,min=2,max=255"`
	Icon               string     `json:"icon" validate:"omitempty,max=100"`
	Path               string     `json:"path" validate:"omitempty,max=255"`
	RequiredPermission string     `json:"required_permission" validate:"omitempty,max=100"`
	SortOrder          int        `json:"sort_order"`
	IsActive           *bool      `json:"is_active"`
	IsHidden           *bool      `json:"is_hidden"`
}

type UpdateMenuRequest struct {
	ParentID           *uuid.UUID `json:"parent_id,omitempty"`
	Title              *string    `json:"title,omitempty" validate:"omitempty,min=2,max=255"`
	Icon               *string    `json:"icon,omitempty" validate:"omitempty,max=100"`
	Path               *string    `json:"path,omitempty" validate:"omitempty,max=255"`
	RequiredPermission *string    `json:"required_permission,omitempty" validate:"omitempty,max=100"`
	SortOrder          *int       `json:"sort_order,omitempty"`
	IsActive           *bool      `json:"is_active,omitempty"`
	IsHidden           *bool      `json:"is_hidden,omitempty"`
}

