package menus

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	_ "taskmgmt/internal/model"
	"taskmgmt/internal/modules/rbac"
	"taskmgmt/internal/platform/response"
)

type Handler struct {
	svc      *Service
	rbacSvc  *rbac.Service
	validate *validator.Validate
}

func NewHandler(svc *Service, rbacSvc *rbac.Service) *Handler {
	return &Handler{
		svc:      svc,
		rbacSvc:  rbacSvc,
		validate: validator.New(),
	}
}

// GetUserMenus godoc
// @Summary Lấy cây Menu động cho người dùng hiện tại (Chuẩn Ant Design)
// @Description Lấy danh sách menu dạng cây phân cấp đã được lọc theo quyền của user, định dạng chuẩn :items của Ant Design a-menu
// @Tags Menus
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]AntdMenuItem}
// @Router /menus/user-menus [get]
func (h *Handler) GetUserMenus(c *fiber.Ctx) error {
	userRoles, _ := c.Locals("roles").([]string)
	isSA := false
	for _, r := range userRoles {
		if r == "system_admin" {
			isSA = true
			break
		}
	}

	perms, err := h.rbacSvc.GetPermissionsForRoles(userRoles)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_PERMS_FAILED", err.Error(), nil)
	}

	items, err := h.svc.GetUserMenus(perms, isSA)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_MENUS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, items)
}

// GetAllMenus godoc
// @Summary Danh sách toàn bộ Menu hệ thống (Dành cho Quản trị viên)
// @Description Lấy toàn bộ cây menu không lọc quyền để phục vụ màn hình cấu hình Menu
// @Tags Menus
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]model.Menu}
// @Router /menus [get]
func (h *Handler) GetAllMenus(c *fiber.Ctx) error {
	menus, err := h.svc.GetAllMenus()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_ALL_MENUS_FAILED", err.Error(), nil)
	}
	return response.JSON(c, fiber.StatusOK, menus)
}

// CreateMenu godoc
// @Summary Thêm mới mục Menu
// @Description Quản trị viên thêm menu mới vào cây điều hướng
// @Tags Menus
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateMenuRequest true "Thông tin menu mới"
// @Success 201 {object} response.SuccessResponse{data=model.Menu}
// @Failure 400 {object} response.ErrorResponse
// @Router /menus [post]
func (h *Handler) CreateMenu(c *fiber.Ctx) error {
	var req CreateMenuRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid fields", err.Error())
	}

	menu, err := h.svc.CreateMenu(req)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "CREATE_MENU_FAILED", err.Error(), nil)
	}
	return response.JSON(c, fiber.StatusCreated, menu)
}

// UpdateMenu godoc
// @Summary Cập nhật mục Menu
// @Description Sửa tên, icon, đường dẫn, quyền yêu cầu hoặc vị trí của menu
// @Tags Menus
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param menuId path string true "ID Menu (UUID)"
// @Param request body UpdateMenuRequest true "Thông tin cập nhật"
// @Success 200 {object} response.SuccessResponse{data=model.Menu}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /menus/{menuId} [patch]
func (h *Handler) UpdateMenu(c *fiber.Ctx) error {
	menuID, err := uuid.Parse(c.Params("menuId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_MENU_ID", "Invalid menu ID in URL", nil)
	}

	var req UpdateMenuRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid fields", err.Error())
	}

	menu, err := h.svc.UpdateMenu(menuID, req)
	if err != nil {
		if errors.Is(err, ErrMenuNotFound) {
			return response.Error(c, fiber.StatusNotFound, "MENU_NOT_FOUND", "Menu not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "UPDATE_MENU_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, menu)
}

// DeleteMenu godoc
// @Summary Xóa mục Menu
// @Description Xóa menu khỏi cây điều hướng (chỉ xóa được menu lá không có menu con)
// @Tags Menus
// @Security BearerAuth
// @Produce json
// @Param menuId path string true "ID Menu (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /menus/{menuId} [delete]
func (h *Handler) DeleteMenu(c *fiber.Ctx) error {
	menuID, err := uuid.Parse(c.Params("menuId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_MENU_ID", "Invalid menu ID in URL", nil)
	}

	if err := h.svc.DeleteMenu(menuID); err != nil {
		if errors.Is(err, ErrMenuNotFound) {
			return response.Error(c, fiber.StatusNotFound, "MENU_NOT_FOUND", "Menu not found", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "DELETE_MENU_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"success": true,
		"message": "Menu deleted successfully",
	})
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	menuGroup := router.Group("/menus", authMiddleware)
	menuGroup.Get("/user-menus", h.GetUserMenus)
	menuGroup.Get("/", h.GetAllMenus)
	menuGroup.Post("/", h.CreateMenu)
	menuGroup.Patch("/:menuId", h.UpdateMenu)
	menuGroup.Delete("/:menuId", h.DeleteMenu)
}

