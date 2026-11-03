package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/middleware"
	"taskmgmt/internal/model"
	platformAuth "taskmgmt/internal/platform/auth"
	"taskmgmt/internal/testutil"
)

func TestMiddleware_RequestID(t *testing.T) {
	app := fiber.New()
	app.Use(middleware.RequestID())
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})

	req := httptest.NewRequest("GET", "/ping", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	reqID := resp.Header.Get("X-Request-ID")
	assert.NotEmpty(t, reqID, "X-Request-ID header should be set")
}

func TestMiddleware_AuthRequired(t *testing.T) {
	secret := "test-jwt-secret-very-long-and-secure-key"
	app := fiber.New()
	app.Get("/protected", middleware.AuthRequired(secret), func(c *fiber.Ctx) error {
		username := c.Locals("username").(string)
		return c.SendString("hello " + username)
	})

	// Case 1: Missing Authorization header -> 401
	req1 := httptest.NewRequest("GET", "/protected", nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp1.StatusCode)

	// Case 2: Invalid token -> 401
	req2 := httptest.NewRequest("GET", "/protected", nil)
	req2.Header.Set("Authorization", "Bearer invalid-token-string")
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)

	// Case 3: Valid token -> 200
	token, _, err := platformAuth.GenerateAccessToken(secret, uuid.New(), uuid.New(), "nghia", []string{"employee"}, 15)
	require.NoError(t, err)

	req3 := httptest.NewRequest("GET", "/protected", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	resp3, err := app.Test(req3)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
}

func TestMiddleware_RequirePermission(t *testing.T) {
	db := testutil.SetupTestDB(t)

	// Seed roles and permissions
	roleEmp := &model.Role{BaseModel: model.BaseModel{ID: uuid.New()}, Code: "employee", Name: "Employee"}
	db.Create(roleEmp)

	permView := &model.Permission{BaseModel: model.BaseModel{ID: uuid.New()}, Code: "tasks:view", Name: "View", Module: "tasks"}
	db.Create(permView)

	db.Create(&model.RolePermission{
		RoleID:       roleEmp.ID,
		PermissionID: permView.ID,
	})

	app := fiber.New()
	app.Get("/tasks",
		func(c *fiber.Ctx) error {
			// Mock user role injected by AuthRequired
			roles := c.Get("X-Mock-Roles")
			if roles != "" {
				c.Locals("roles", []string{roles})
			}
			return c.Next()
		},
		middleware.RequirePermission(db, "tasks:view"),
		func(c *fiber.Ctx) error {
			return c.SendString("tasks list")
		},
	)

	// Case 1: No roles in context -> 403
	req1 := httptest.NewRequest("GET", "/tasks", nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp1.StatusCode)

	// Case 2: Role without permission -> 403
	req2 := httptest.NewRequest("GET", "/tasks", nil)
	req2.Header.Set("X-Mock-Roles", "guest")
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)

	// Case 3: Role WITH permission -> 200
	req3 := httptest.NewRequest("GET", "/tasks", nil)
	req3.Header.Set("X-Mock-Roles", "employee")
	resp3, err := app.Test(req3)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)

	// Case 4: system_admin role -> bypasses check -> 200
	req4 := httptest.NewRequest("GET", "/tasks", nil)
	req4.Header.Set("X-Mock-Roles", "system_admin")
	resp4, err := app.Test(req4)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp4.StatusCode)
}
