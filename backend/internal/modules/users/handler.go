package users

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

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

// ListUsers godoc
// @Summary Danh sách người dùng
// @Description Lấy danh sách người dùng trong đơn vị có phân trang, tìm kiếm và lọc
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param page query int false "Trang hiện tại (mặc định 1)"
// @Param page_size query int false "Số lượng mỗi trang (mặc định 20)"
// @Param q query string false "Từ khóa tìm kiếm (tên đăng nhập, họ tên, email)"
// @Param department_id query string false "Lọc theo phòng ban"
// @Param role query string false "Lọc theo mã vai trò"
// @Param is_active query bool false "Lọc theo trạng thái hoạt động"
// @Success 200 {object} response.SuccessResponse{data=[]UserListItem}
// @Router /users [get]
func (h *Handler) ListUsers(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	var filter UserFilter
	if err := c.QueryParser(&filter); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_QUERY", "Invalid query parameters", err.Error())
	}

	items, total, err := h.svc.ListUsers(orgID, filter)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "LIST_USERS_FAILED", err.Error(), nil)
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return response.JSONWithMeta(c, fiber.StatusOK, items, response.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetUserOptions godoc
// @Summary Danh sách người dùng rút gọn cho Select
// @Description Lấy danh sách người dùng đang hoạt động để gán việc trong thẻ a-select của Ant Design
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]UserOption}
// @Router /users/options [get]
func (h *Handler) GetUserOptions(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	options, err := h.svc.GetUserOptions(orgID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_USER_OPTIONS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, options)
}

// GetUserDetail godoc
// @Summary Chi tiết người dùng
// @Description Lấy thông tin chi tiết của người dùng kèm danh sách vai trò và phòng ban
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} response.SuccessResponse{data=UserDetailResponse}
// @Router /users/{userId} [get]
func (h *Handler) GetUserDetail(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Invalid user ID format", err.Error())
	}

	user, err := h.svc.GetUserDetail(orgID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "USER_NOT_FOUND", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, user)
}

// CreateUser godoc
// @Summary Tạo người dùng mới
// @Description Quản trị viên thêm nhân sự mới vào hệ thống kèm vai trò và phòng ban
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "Thông tin người dùng mới"
// @Success 201 {object} response.SuccessResponse{data=UserDetailResponse}
// @Router /users [post]
func (h *Handler) CreateUser(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Thông tin người dùng không hợp lệ", err.Error())
	}

	user, err := h.svc.CreateUser(orgID, req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "CREATE_USER_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, user)
}

// UpdateUser godoc
// @Summary Cập nhật thông tin người dùng
// @Description Chỉnh sửa họ tên, email, trạng thái hoạt động hoặc phòng ban
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Param request body UpdateUserRequest true "Thông tin cần sửa"
// @Success 200 {object} response.SuccessResponse{data=UserDetailResponse}
// @Router /users/{userId} [patch]
func (h *Handler) UpdateUser(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Invalid user ID format", err.Error())
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}

	user, err := h.svc.UpdateUser(orgID, userID, req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "UPDATE_USER_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, user)
}

// ResetPassword godoc
// @Summary Đặt lại mật khẩu người dùng
// @Description Quản trị viên cấp lại mật khẩu mới cho người dùng
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Param request body ResetPasswordRequest true "Mật khẩu mới"
// @Success 200 {object} response.SuccessResponse
// @Router /users/{userId}/reset-password [post]
func (h *Handler) ResetPassword(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Invalid user ID format", err.Error())
	}

	var req ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Mật khẩu phải từ 6 ký tự", err.Error())
	}

	if err := h.svc.ResetPassword(orgID, userID, req.NewPassword); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "RESET_PASSWORD_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"success": true,
		"message": "Đặt lại mật khẩu thành công",
	})
}

// GetUserRoles godoc
// @Summary Xem danh sách vai trò của người dùng
// @Tags Users
// @Security BearerAuth
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {object} response.SuccessResponse{data=[]string}
// @Router /users/{userId}/roles [get]
func (h *Handler) GetUserRoles(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Invalid user ID format", err.Error())
	}

	user, err := h.svc.GetUserDetail(orgID, userID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "USER_NOT_FOUND", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, user.Roles)
}

// SetUserRoles godoc
// @Summary Cập nhật danh sách vai trò của người dùng
// @Description Thay thế toàn bộ vai trò hiện tại bằng danh sách vai trò mới
// @Tags Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param userId path string true "User ID"
// @Param request body SetUserRolesRequest true "Danh sách mã vai trò"
// @Success 200 {object} response.SuccessResponse{data=UserDetailResponse}
// @Router /users/{userId}/roles [put]
func (h *Handler) SetUserRoles(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	userID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Invalid user ID format", err.Error())
	}

	var req SetUserRolesRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Danh sách vai trò không hợp lệ", err.Error())
	}

	user, err := h.svc.SetUserRoles(orgID, userID, req.RoleCodes)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "SET_ROLES_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, user)
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	group := router.Group("/users", authMiddleware)
	group.Get("/options", h.GetUserOptions)
	group.Get("/", h.ListUsers)
	group.Post("/", h.CreateUser)
	group.Get("/:userId", h.GetUserDetail)
	group.Patch("/:userId", h.UpdateUser)
	group.Post("/:userId/reset-password", h.ResetPassword)
	group.Get("/:userId/roles", h.GetUserRoles)
	group.Put("/:userId/roles", h.SetUserRoles)
}
