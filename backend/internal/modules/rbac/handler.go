package rbac

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	_ "taskmgmt/internal/model"
	"taskmgmt/internal/platform/response"
)

type Handler struct {
	svc      *Service
	validate *validator.Validate
}

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc:      svc,
		validate: validator.New(),
	}
}

// ListPermissions godoc
// @Summary Danh sách tất cả quyền hệ thống
// @Description Lấy danh sách quyền được gom nhóm theo phân hệ (module) cho bảng ma trận phân quyền
// @Tags RBAC
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]ModuleGroupedPermissions}
// @Failure 401 {object} response.ErrorResponse
// @Router /permissions [get]
func (h *Handler) ListPermissions(c *fiber.Ctx) error {
	list, err := h.svc.ListPermissions()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "LIST_PERMISSIONS_FAILED", err.Error(), nil)
	}
	return response.JSON(c, fiber.StatusOK, list)
}

// GetPermissionOptions godoc
// @Summary Danh sách quyền dạng Options cho Ant Design Select
// @Description Lấy danh sách quyền {value, label, module} cho component a-select khi tạo/sửa Menu
// @Tags RBAC
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]PermissionOption}
// @Router /permissions/options [get]
func (h *Handler) GetPermissionOptions(c *fiber.Ctx) error {
	options, err := h.svc.GetPermissionOptions()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_OPTIONS_FAILED", err.Error(), nil)
	}
	return response.JSON(c, fiber.StatusOK, options)
}

// CreatePermission godoc
// @Summary Thêm quyền chức năng mới
// @Description Quản trị viên đăng ký thêm mã quyền mới vào hệ thống
// @Tags RBAC
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreatePermissionRequest true "Thông tin quyền mới"
// @Success 201 {object} response.SuccessResponse{data=model.Permission}
// @Failure 400 {object} response.ErrorResponse
// @Router /permissions [post]
func (h *Handler) CreatePermission(c *fiber.Ctx) error {
	var req CreatePermissionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid fields", err.Error())
	}

	perm, err := h.svc.CreatePermission(req)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "CREATE_PERMISSION_FAILED", err.Error(), nil)
	}
	return response.JSON(c, fiber.StatusCreated, perm)
}

// ListRoles godoc
// @Summary Danh sách vai trò hệ thống
// @Description Lấy danh sách các vai trò để quản lý phân quyền
// @Tags RBAC
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]model.Role}
// @Router /roles [get]
func (h *Handler) ListRoles(c *fiber.Ctx) error {
	roles, err := h.svc.ListRoles()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "LIST_ROLES_FAILED", err.Error(), nil)
	}
	return response.JSON(c, fiber.StatusOK, roles)
}

// GetRolePermissions godoc
// @Summary Lấy danh sách quyền của một vai trò
// @Description Lấy chi tiết vai trò kèm toàn bộ các quyền chức năng đang được gán
// @Tags RBAC
// @Security BearerAuth
// @Produce json
// @Param roleId path string true "ID vai trò (UUID)"
// @Success 200 {object} response.SuccessResponse{data=RoleWithPermissionsResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /roles/{roleId}/permissions [get]
func (h *Handler) GetRolePermissions(c *fiber.Ctx) error {
	roleID, err := uuid.Parse(c.Params("roleId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ROLE_ID", "Invalid role ID in URL", nil)
	}

	res, err := h.svc.GetRoleWithPermissions(roleID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "ROLE_NOT_FOUND", "Role not found", nil)
	}
	return response.JSON(c, fiber.StatusOK, res)
}

// SetRolePermissions godoc
// @Summary Cập nhật tập quyền cho một vai trò
// @Description Thay thế toàn bộ danh sách quyền của một vai trò bằng danh sách mã quyền mới
// @Tags RBAC
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param roleId path string true "ID vai trò (UUID)"
// @Param request body UpdateRolePermissionsRequest true "Danh sách mã quyền mới"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /roles/{roleId}/permissions [put]
func (h *Handler) SetRolePermissions(c *fiber.Ctx) error {
	roleID, err := uuid.Parse(c.Params("roleId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ROLE_ID", "Invalid role ID in URL", nil)
	}

	var req UpdateRolePermissionsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}

	if err := h.svc.SetRolePermissions(roleID, req.PermissionCodes); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "SET_PERMISSIONS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"success": true,
		"message": "Role permissions updated successfully",
	})
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	// Permissions
	perms := router.Group("/permissions", authMiddleware)
	perms.Get("/", h.ListPermissions)
	perms.Get("/options", h.GetPermissionOptions)
	perms.Post("/", h.CreatePermission)

	// Roles
	roles := router.Group("/roles", authMiddleware)
	roles.Get("/", h.ListRoles)
	roles.Get("/:roleId/permissions", h.GetRolePermissions)
	roles.Put("/:roleId/permissions", h.SetRolePermissions)
}

