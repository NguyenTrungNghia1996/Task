package rbac_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/rbac"
	"taskmgmt/internal/testutil"
)

func setupRBACService(t *testing.T) (*rbac.Service, *model.Role) {
	db := testutil.SetupTestDB(t)
	repo := rbac.NewRepository(db)
	svc := rbac.NewService(repo)

	role := &model.Role{
		BaseModel: model.BaseModel{ID: uuid.New()},
		Code:      "department_head",
		Name:      "Trưởng phòng",
	}
	require.NoError(t, db.Create(role).Error)

	adminRole := &model.Role{
		BaseModel: model.BaseModel{ID: uuid.New()},
		Code:      "system_admin",
		Name:      "Quản trị hệ thống",
	}
	require.NoError(t, db.Create(adminRole).Error)

	return svc, role
}

func TestRBAC_CreateAndListPermissions(t *testing.T) {
	svc, _ := setupRBACService(t)

	// Create 3 permissions across 2 modules
	p1, err := svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "tasks:view",
		Name:   "Xem công việc",
		Module: "tasks",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, p1.ID)

	_, err = svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "tasks:create",
		Name:   "Tạo công việc",
		Module: "tasks",
	})
	require.NoError(t, err)

	_, err = svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "users:manage",
		Name:   "Quản lý nhân sự",
		Module: "users",
	})
	require.NoError(t, err)

	// List grouped
	grouped, err := svc.ListPermissions()
	require.NoError(t, err)
	assert.Len(t, grouped, 2, "Should have 2 modules: tasks and users")

	for _, g := range grouped {
		if g.Module == "tasks" {
			assert.Len(t, g.Permissions, 2)
		} else if g.Module == "users" {
			assert.Len(t, g.Permissions, 1)
		}
	}
}

func TestRBAC_GetPermissionOptions(t *testing.T) {
	svc, _ := setupRBACService(t)

	_, err := svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "tasks:view",
		Name:   "Xem công việc",
		Module: "tasks",
	})
	require.NoError(t, err)

	options, err := svc.GetPermissionOptions()
	require.NoError(t, err)
	assert.NotEmpty(t, options)
	assert.Equal(t, "tasks:view", options[0].Value)
	assert.Contains(t, options[0].Label, "Xem công việc")
	assert.Equal(t, "tasks", options[0].Module)
}

func TestRBAC_SetAndGetRolePermissions(t *testing.T) {
	svc, role := setupRBACService(t)

	_, err := svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "tasks:view",
		Name:   "Xem công việc",
		Module: "tasks",
	})
	require.NoError(t, err)

	_, err = svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "tasks:create",
		Name:   "Tạo công việc",
		Module: "tasks",
	})
	require.NoError(t, err)

	// Set permissions for role
	err = svc.SetRolePermissions(role.ID, []string{"tasks:view", "tasks:create"})
	require.NoError(t, err)

	// Verify permissions
	res, err := svc.GetRoleWithPermissions(role.ID)
	require.NoError(t, err)
	assert.Equal(t, role.Code, res.Role.Code)
	assert.Len(t, res.Permissions, 2)

	codes := []string{res.Permissions[0].Code, res.Permissions[1].Code}
	assert.Contains(t, codes, "tasks:view")
	assert.Contains(t, codes, "tasks:create")

	// Update permissions (remove tasks:create, only keep tasks:view)
	err = svc.SetRolePermissions(role.ID, []string{"tasks:view"})
	require.NoError(t, err)

	res2, err := svc.GetRoleWithPermissions(role.ID)
	require.NoError(t, err)
	assert.Len(t, res2.Permissions, 1)
	assert.Equal(t, "tasks:view", res2.Permissions[0].Code)
}

func TestRBAC_SystemAdmin_Bypass(t *testing.T) {
	svc, _ := setupRBACService(t)

	_, err := svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "tasks:view",
		Name:   "Xem công việc",
		Module: "tasks",
	})
	require.NoError(t, err)

	_, err = svc.CreatePermission(rbac.CreatePermissionRequest{
		Code:   "system:settings",
		Name:   "Cài đặt",
		Module: "system",
	})
	require.NoError(t, err)

	// system_admin should receive all permissions in the system automatically
	perms, err := svc.GetPermissionsForRoles([]string{"system_admin"})
	require.NoError(t, err)
	assert.Len(t, perms, 2)
	assert.Contains(t, perms, "tasks:view")
	assert.Contains(t, perms, "system:settings")
}
