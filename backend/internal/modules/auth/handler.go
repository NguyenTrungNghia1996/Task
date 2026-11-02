package auth

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

// Login godoc
// @Summary Đăng nhập hệ thống
// @Description Xác thực tài khoản bằng username/password và nhận JWT Access Token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Thông tin tài khoản"
// @Success 200 {object} response.SuccessResponse{data=LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /auth/login [post]
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Missing required credentials", err.Error())
	}

	res, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "AUTH_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, res)
}

// Refresh godoc
// @Summary Làm mới Access Token
// @Description Dùng Refresh Token hợp lệ để cấp mới Access Token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh Token"
// @Success 200 {object} response.SuccessResponse{data=LoginResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /auth/refresh [post]
func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Missing refresh token", err.Error())
	}

	res, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "REFRESH_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, res)
}

// Logout godoc
// @Summary Đăng xuất
// @Description Thu hồi Refresh Token hiện tại
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest false "Refresh Token"
// @Success 200 {object} response.SuccessResponse{data=LogoutResponse}
// @Router /auth/logout [post]
func (h *Handler) Logout(c *fiber.Ctx) error {
	var req RefreshRequest
	_ = c.BodyParser(&req)

	if req.RefreshToken != "" {
		_ = h.svc.Logout(req.RefreshToken)
	}

	return response.JSON(c, fiber.StatusOK, LogoutResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}

// Me godoc
// @Summary Thông tin tài khoản hiện tại
// @Description Lấy thông tin cá nhân và vai trò của người dùng đang đăng nhập
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=UserInfoResp}
// @Failure 401 {object} response.ErrorResponse
// @Router /auth/me [get]
func (h *Handler) Me(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	me, err := h.svc.GetMe(userID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
	}

	return response.JSON(c, fiber.StatusOK, me)
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	authGroup := router.Group("/auth")
	authGroup.Post("/login", h.Login)
	authGroup.Post("/refresh", h.Refresh)
	authGroup.Post("/logout", h.Logout)
	authGroup.Get("/me", authMiddleware, h.Me)
}

