package users_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/users"
	"taskmgmt/internal/testutil"
)

func setupUserService(t *testing.T) (*users.Service, *model.Organization, *model.Department, *model.Role) {
	db := testutil.SetupTestDB(t)

	org, _ := testutil.CreateTestOrgAndUser(t, db, "admin_user", "system_admin")

	dept := &model.Department{
		BaseModel:      model.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Code:           "DEPT_IT",
		Name:           "Phòng Công nghệ thông tin",
		DepartmentType: "department",
		IsActive:       true,
	}
	require.NoError(t, db.Create(dept).Error)

	role := &model.Role{
		BaseModel: model.BaseModel{ID: uuid.New()},
		Code:      "developer",
		Name:      "Lập trình viên",
	}
	require.NoError(t, db.Create(role).Error)

	repo := users.NewRepository(db)
	svc := users.NewService(repo)
	return svc, org, dept, role
}

func TestUser_Create_Success(t *testing.T) {
	svc, org, dept, role := setupUserService(t)

	req := users.CreateUserRequest{
		Username:      "nghia_dev",
		FullName:      "Nguyễn Trung Nghĩa",
		Email:         "nghia@example.com",
		Password:      "securepassword123",
		DepartmentID:  &dept.ID,
		PositionTitle: "Kỹ sư phần mềm",
		RoleCodes:     []string{role.Code},
	}

	user, err := svc.CreateUser(org.ID, req)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "nghia_dev", user.Username)
	assert.Equal(t, "Nguyễn Trung Nghĩa", user.FullName)
	assert.Equal(t, "nghia@example.com", user.Email)
	assert.True(t, user.IsActive)
	assert.Contains(t, user.Roles, "developer")
	assert.Len(t, user.Departments, 1)
	assert.Equal(t, dept.Name, user.Departments[0].Name)
	assert.Equal(t, "Kỹ sư phần mềm", user.Departments[0].PositionTitle)
}

func TestUser_Create_DuplicateUsername(t *testing.T) {
	svc, org, _, _ := setupUserService(t)

	req := users.CreateUserRequest{
		Username: "duplicate_user",
		FullName: "Người Dùng Mẫu",
		Password: "password123",
	}

	_, err := svc.CreateUser(org.ID, req)
	require.NoError(t, err)

	// Second attempt with same username must fail
	_, err = svc.CreateUser(org.ID, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tồn tại")
}

func TestUser_ListUsers_FilterAndSearch(t *testing.T) {
	svc, org, dept, _ := setupUserService(t)

	// Create 3 users
	_, err := svc.CreateUser(org.ID, users.CreateUserRequest{
		Username:     "user_alpha",
		FullName:     "Trần Văn Alpha",
		Email:        "alpha@test.com",
		Password:     "password123",
		DepartmentID: &dept.ID,
	})
	require.NoError(t, err)

	_, err = svc.CreateUser(org.ID, users.CreateUserRequest{
		Username: "user_beta",
		FullName: "Lê Thị Beta",
		Email:    "beta@test.com",
		Password: "password123",
	})
	require.NoError(t, err)

	// Test 1: Search query "Alpha"
	items, total, err := svc.ListUsers(org.ID, users.UserFilter{
		Query: "Alpha",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "user_alpha", items[0].Username)

	// Test 2: Filter by department
	items, total, err = svc.ListUsers(org.ID, users.UserFilter{
		DepartmentID: &dept.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "user_alpha", items[0].Username)
}

func TestUser_Update(t *testing.T) {
	svc, org, dept, _ := setupUserService(t)

	created, err := svc.CreateUser(org.ID, users.CreateUserRequest{
		Username: "update_target",
		FullName: "Tên Ban Đầu",
		Password: "password123",
	})
	require.NoError(t, err)

	newFullName := "Tên Đã Cập Nhật"
	newEmail := "updated@test.com"
	isActive := false
	newPos := "Trưởng nhóm"

	updated, err := svc.UpdateUser(org.ID, created.ID, users.UpdateUserRequest{
		FullName:      &newFullName,
		Email:         &newEmail,
		IsActive:      &isActive,
		DepartmentID:  &dept.ID,
		PositionTitle: &newPos,
	})
	require.NoError(t, err)
	assert.Equal(t, "Tên Đã Cập Nhật", updated.FullName)
	assert.Equal(t, "updated@test.com", updated.Email)
	assert.False(t, updated.IsActive)
	assert.Len(t, updated.Departments, 1)
	assert.Equal(t, "Trưởng nhóm", updated.Departments[0].PositionTitle)
}

func TestUser_SetRoles(t *testing.T) {
	svc, org, _, role := setupUserService(t)

	created, err := svc.CreateUser(org.ID, users.CreateUserRequest{
		Username: "role_user",
		FullName: "Nhân Sự Vai Trò",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Empty(t, created.Roles)

	// Assign role
	updated, err := svc.SetUserRoles(org.ID, created.ID, []string{role.Code})
	require.NoError(t, err)
	assert.Contains(t, updated.Roles, "developer")

	// Revoke role
	updated2, err := svc.SetUserRoles(org.ID, created.ID, []string{})
	require.NoError(t, err)
	assert.Empty(t, updated2.Roles)
}

func TestUser_ResetPassword(t *testing.T) {
	svc, org, _, _ := setupUserService(t)

	created, err := svc.CreateUser(org.ID, users.CreateUserRequest{
		Username: "pwd_reset_user",
		FullName: "Người Dùng Quên Mật Khẩu",
		Password: "oldpassword123",
	})
	require.NoError(t, err)

	err = svc.ResetPassword(org.ID, created.ID, "newsupersecretpassword")
	require.NoError(t, err)
}

func TestUser_GetUserOptions(t *testing.T) {
	svc, org, dept, _ := setupUserService(t)

	_, err := svc.CreateUser(org.ID, users.CreateUserRequest{
		Username:      "opt_user",
		FullName:      "Nhân Sự Lựa Chọn",
		Password:      "password123",
		DepartmentID:  &dept.ID,
		PositionTitle: "Chuyên viên",
	})
	require.NoError(t, err)

	options, err := svc.GetUserOptions(org.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, options)

	var found bool
	for _, o := range options {
		if o.Username == "opt_user" {
			found = true
			assert.Contains(t, o.Label, "Nhân Sự Lựa Chọn (opt_user)")
			assert.Contains(t, o.Label, dept.Name)
			assert.Equal(t, "Chuyên viên", o.PositionTitle)
		}
	}
	assert.True(t, found, "Newly created user should be in options list")
}
