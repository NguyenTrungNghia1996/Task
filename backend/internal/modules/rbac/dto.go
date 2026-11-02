package rbac

import (
	"taskmgmt/internal/model"
)

type PermissionOption struct {
	Value  string `json:"value"`  // vd: "tasks:view"
	Label  string `json:"label"`  // vd: "Xem danh sách công việc (tasks:view)"
	Module string `json:"module"` // vd: "Quản lý công việc"
}

type ModuleGroupedPermissions struct {
	Module      string             `json:"module"`
	Permissions []model.Permission `json:"permissions"`
}

type CreatePermissionRequest struct {
	Code        string `json:"code" validate:"required,min=3,max=100"`
	Name        string `json:"name" validate:"required,min=2,max=255"`
	Module      string `json:"module" validate:"required,min=2,max=50"`
	Description string `json:"description"`
}

type RoleWithPermissionsResponse struct {
	Role        model.Role         `json:"role"`
	Permissions []model.Permission `json:"permissions"`
}

type UpdateRolePermissionsRequest struct {
	PermissionCodes []string `json:"permission_codes" validate:"required"`
}
