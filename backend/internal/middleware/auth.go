package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"taskmgmt/internal/platform/auth"
	"taskmgmt/internal/platform/response"
)

func AuthRequired(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		var tokenStr string

		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			// Fallback to cookie if present
			tokenStr = c.Cookies("access_token")
		}

		if tokenStr == "" {
			// Fallback to query param for EventSource (SSE) and WebSocket clients
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing authentication token", nil)
		}

		claims, err := auth.ValidateAccessToken(jwtSecret, tokenStr)
		if err != nil {
			return response.Error(c, fiber.StatusUnauthorized, "INVALID_TOKEN", "Token is invalid or expired", err.Error())
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("org_id", claims.OrganizationID)
		c.Locals("username", claims.Username)
		c.Locals("roles", claims.Roles)

		return c.Next()
	}
}

func RequireRole(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRoles, ok := c.Locals("roles").([]string)
		if !ok {
			return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "Insufficient role permissions", nil)
		}

		roleMap := make(map[string]bool)
		for _, r := range userRoles {
			roleMap[r] = true
		}

		for _, allowed := range allowedRoles {
			if roleMap[allowed] {
				return c.Next()
			}
		}

		return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource", nil)
	}
}

