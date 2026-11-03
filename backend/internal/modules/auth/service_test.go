package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/config"
	"taskmgmt/internal/model"
	"taskmgmt/internal/modules/auth"
	"taskmgmt/internal/testutil"
)

func setupAuthService(t *testing.T) (*auth.Service, *model.Organization, *model.User) {
	db := testutil.SetupTestDB(t)
	cfg := &config.Config{
		JWTSecret:         "test-jwt-secret-very-long-and-secure-key",
		JWTAccessTTLMin:   15,
		JWTRefreshTTLDays: 7,
	}

	// Create test org and user
	org, user := testutil.CreateTestOrgAndUser(t, db, "testuser", "employee")

	// Seed some permissions for employee
	perm := &model.Permission{
		Code:   "tasks:view",
		Name:   "Xem công việc",
		Module: "tasks",
	}
	db.Create(perm)

	var role model.Role
	db.Where("code = ?", "employee").First(&role)
	db.Create(&model.RolePermission{
		RoleID:       role.ID,
		PermissionID: perm.ID,
	})

	svc := auth.NewService(db, cfg)
	return svc, org, user
}

func TestAuth_Login_Success(t *testing.T) {
	svc, _, user := setupAuthService(t)

	res, err := svc.Login(user.Username, "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.NotEmpty(t, res.RefreshToken)
	assert.Equal(t, user.Username, res.User.Username)
	assert.Contains(t, res.User.Roles, "employee")
	assert.Contains(t, res.User.Permissions, "tasks:view")
}

func TestAuth_Login_InvalidPassword(t *testing.T) {
	svc, _, user := setupAuthService(t)

	res, err := svc.Login(user.Username, "wrongpassword")
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "invalid username or password")
}

func TestAuth_Login_NonExistentUser(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	res, err := svc.Login("nonexistent", "password123")
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "invalid username or password")
}

func TestAuth_Refresh_SuccessAndRotation(t *testing.T) {
	svc, _, user := setupAuthService(t)

	loginRes, err := svc.Login(user.Username, "password123")
	require.NoError(t, err)
	oldRefreshToken := loginRes.RefreshToken

	// First refresh: should succeed
	refreshRes, err := svc.Refresh(oldRefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshRes.AccessToken)
	assert.NotEmpty(t, refreshRes.RefreshToken)
	assert.NotEqual(t, oldRefreshToken, refreshRes.RefreshToken, "Refresh token should be rotated")

	// Second refresh with OLD token: must fail because old token is revoked
	reusedRes, err := svc.Refresh(oldRefreshToken)
	assert.Error(t, err)
	assert.Nil(t, reusedRes)
	assert.Contains(t, err.Error(), "invalid or expired refresh token")

	// Refresh with NEW token: should succeed
	secondRefresh, err := svc.Refresh(refreshRes.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, secondRefresh.AccessToken)
}

func TestAuth_Refresh_InvalidOrExpired(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	// Completely bogus token
	res, err := svc.Refresh("invalid-random-token-hash-123456")
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "invalid or expired refresh token")
}

func TestAuth_Logout(t *testing.T) {
	svc, _, user := setupAuthService(t)

	loginRes, err := svc.Login(user.Username, "password123")
	require.NoError(t, err)

	err = svc.Logout(loginRes.RefreshToken)
	require.NoError(t, err)

	// Once logged out, the refresh token cannot be used
	refreshRes, err := svc.Refresh(loginRes.RefreshToken)
	assert.Error(t, err)
	assert.Nil(t, refreshRes)
}

func TestAuth_GetMe(t *testing.T) {
	svc, _, user := setupAuthService(t)

	info, err := svc.GetMe(user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, info.ID)
	assert.Equal(t, user.Username, info.Username)
	assert.Contains(t, info.Roles, "employee")
	assert.Contains(t, info.Permissions, "tasks:view")
}
