package menus_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/modules/menus"
	"taskmgmt/internal/testutil"
)

func setupMenuService(t *testing.T) *menus.Service {
	db := testutil.SetupTestDB(t)
	repo := menus.NewRepository(db)
	return menus.NewService(repo)
}

func TestMenus_UserMenus_Filtering(t *testing.T) {
	svc := setupMenuService(t)

	// 1. Dashboard (public / no permission)
	_, err := svc.CreateMenu(menus.CreateMenuRequest{
		Code:      "dashboard",
		Title:     "Bảng điều khiển",
		Path:      "/dashboard",
		SortOrder: 1,
	})
	require.NoError(t, err)

	// 2. Tasks parent menu
	tasksParent, err := svc.CreateMenu(menus.CreateMenuRequest{
		Code:      "tasks",
		Title:     "Quản lý công việc",
		SortOrder: 2,
	})
	require.NoError(t, err)

	// 2a. Child 1: tasks:view
	_, err = svc.CreateMenu(menus.CreateMenuRequest{
		ParentID:           &tasksParent.ID,
		Code:               "tasks_my",
		Title:              "Việc của tôi",
		Path:               "/tasks/my-tasks",
		RequiredPermission: "tasks:view",
		SortOrder:          1,
	})
	require.NoError(t, err)

	// 2b. Child 2: tasks:view_all
	_, err = svc.CreateMenu(menus.CreateMenuRequest{
		ParentID:           &tasksParent.ID,
		Code:               "tasks_all",
		Title:              "Tất cả công việc",
		Path:               "/tasks/all",
		RequiredPermission: "tasks:view_all",
		SortOrder:          2,
	})
	require.NoError(t, err)

	// 3. Admin root menu
	_, err = svc.CreateMenu(menus.CreateMenuRequest{
		Code:               "admin",
		Title:              "Quản trị",
		Path:               "/admin",
		RequiredPermission: "system:settings",
		SortOrder:          3,
	})
	require.NoError(t, err)

	// Case 1: Employee with only "tasks:view"
	empMenus, err := svc.GetUserMenus([]string{"tasks:view"}, false)
	require.NoError(t, err)

	assert.Len(t, empMenus, 2, "Employee should see Dashboard and Tasks only")
	assert.Equal(t, "/dashboard", empMenus[0].Key)
	assert.Equal(t, "tasks", empMenus[1].Key)
	assert.Len(t, empMenus[1].Children, 1, "Employee should only see 1 child under tasks")
	assert.Equal(t, "/tasks/my-tasks", empMenus[1].Children[0].Path)

	// Case 2: System Admin (bypass)
	adminMenus, err := svc.GetUserMenus([]string{}, true)
	require.NoError(t, err)

	assert.Len(t, adminMenus, 3, "Admin should see all 3 root menus")
	assert.Equal(t, "/dashboard", adminMenus[0].Key)
	assert.Equal(t, "tasks", adminMenus[1].Key)
	assert.Len(t, adminMenus[1].Children, 2, "Admin should see both children under tasks")
	assert.Equal(t, "/admin", adminMenus[2].Key)
}

func TestMenus_CRUD(t *testing.T) {
	svc := setupMenuService(t)

	// 1. Create root menu
	m, err := svc.CreateMenu(menus.CreateMenuRequest{
		Code:      "reports",
		Title:     "Báo cáo",
		Path:      "/reports",
		SortOrder: 1,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, m.ID)

	// 2. Create child menu
	child, err := svc.CreateMenu(menus.CreateMenuRequest{
		ParentID:  &m.ID,
		Code:      "reports_export",
		Title:     "Xuất dữ liệu",
		Path:      "/reports/export",
		SortOrder: 1,
	})
	require.NoError(t, err)

	// 3. Try to delete parent (should fail because child exists)
	err = svc.DeleteMenu(m.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sub-menus")

	// 4. Update child
	newTitle := "Báo cáo thống kê"
	updated, err := svc.UpdateMenu(child.ID, menus.UpdateMenuRequest{
		Title: &newTitle,
	})
	require.NoError(t, err)
	assert.Equal(t, "Báo cáo thống kê", updated.Title)

	// 5. Delete child first, then delete parent
	err = svc.DeleteMenu(child.ID)
	require.NoError(t, err)

	err = svc.DeleteMenu(m.ID)
	require.NoError(t, err)
}

