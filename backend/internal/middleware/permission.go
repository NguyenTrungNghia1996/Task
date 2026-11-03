package middleware

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"taskmgmt/internal/platform/response"
)

// RequirePermission bảo vệ endpoint theo mã quyền chức năng (hỗ trợ phân quyền động)
func RequirePermission(db *gorm.DB, requiredPerms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRoles, ok := c.Locals("roles").([]string)
		if !ok || len(userRoles) == 0 {
			return response.Error(c, fiber.StatusForbidden, "FORBIDDEN", "Không có quyền truy cập", nil)
		}

		// system_admin luôn có toàn quyền
		for _, r := range userRoles {
			if r == "system_admin" {
				return c.Next()
			}
		}

		var count int64
		err := db.Table("role_permissions").
			Joins("JOIN roles ON roles.id = role_permissions.role_id").
			Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
			Where("roles.code IN ? AND permissions.code IN ? AND role_permissions.deleted_at IS NULL AND roles.deleted_at IS NULL AND permissions.deleted_at IS NULL", userRoles, requiredPerms).
			Count(&count).Error

		if err != nil || count == 0 {
			return response.Error(c, fiber.StatusForbidden, "PERMISSION_DENIED",
				"Bạn không có quyền thực hiện chức năng này", nil)
		}

		return c.Next()
	}
}

