package model

import (
	"github.com/google/uuid"
)

// Permission đại diện cho một quyền chức năng cụ thể trong hệ thống
type Permission struct {
	BaseModel
	Code        string `gorm:"size:100;not null;uniqueIndex" json:"code"`
	Name        string `gorm:"size:255;not null" json:"name"`
	Module      string `gorm:"size:50;not null;index" json:"module"`
	Description string `gorm:"type:text" json:"description,omitempty"`
}

// RolePermission liên kết n-n giữa vai trò và quyền chức năng
type RolePermission struct {
	BaseModel
	RoleID       uuid.UUID   `gorm:"type:uuid;not null;index" json:"role_id"`
	PermissionID uuid.UUID   `gorm:"type:uuid;not null;index" json:"permission_id"`
	Role         *Role       `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Permission   *Permission `gorm:"foreignKey:PermissionID" json:"permission,omitempty"`
}

// Menu đại diện cho một mục menu điều hướng phía Frontend (tương thích Ant Design)
type Menu struct {
	BaseModel
	ParentID           *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Code               string     `gorm:"size:100;not null;uniqueIndex" json:"code"`
	Title              string     `gorm:"size:255;not null" json:"title"`
	Icon               string     `gorm:"size:100" json:"icon,omitempty"`
	Path               string     `gorm:"size:255" json:"path,omitempty"`
	RequiredPermission string     `gorm:"size:100;index" json:"required_permission,omitempty"`
	SortOrder          int        `gorm:"not null;default:0" json:"sort_order"`
	IsActive           bool       `gorm:"not null;default:true" json:"is_active"`
	IsHidden           bool       `gorm:"not null;default:false" json:"is_hidden"`
	Children           []Menu     `gorm:"foreignKey:ParentID" json:"children,omitempty"`
}

