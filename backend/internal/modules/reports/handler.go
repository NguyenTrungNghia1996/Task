package reports

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"taskmgmt/internal/platform/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetDashboardSummary godoc
// @Summary Tổng quan số liệu Dashboard
// @Description Thống kê chỉ số KPI tổng quan công việc theo phạm vi quyền của người dùng hiện tại
// @Tags Dashboard
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=DashboardSummaryResponse}
// @Router /dashboard/summary [get]
func (h *Handler) GetDashboardSummary(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	userID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	roles, _ := c.Locals("roles").([]string)

	summary, err := h.svc.GetDashboardSummary(orgID, userID, roles)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_SUMMARY_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, summary)
}

// GetMyTasks godoc
// @Summary Danh sách công việc cần xử lý của tôi
// @Description Lấy danh sách việc của tôi sắp xếp theo mức độ khẩn cấp (quá hạn, ưu tiên cao lên đầu)
// @Tags Dashboard
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Số lượng lấy tối đa (mặc định 10)"
// @Success 200 {object} response.SuccessResponse{data=[]MyTaskItem}
// @Router /dashboard/my-tasks [get]
func (h *Handler) GetMyTasks(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	userID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	tasks, err := h.svc.GetMyTasks(orgID, userID, limit)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_MY_TASKS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, tasks)
}

// GetTaskReport godoc
// @Summary Báo cáo thống kê tiến độ công việc
// @Description Thống kê tổng hợp số liệu công việc theo trạng thái, mức độ ưu tiên và thời gian
// @Tags Reports
// @Security BearerAuth
// @Produce json
// @Param from_date query string false "Từ ngày (ISO 8601)"
// @Param to_date query string false "Đến ngày (ISO 8601)"
// @Param department_id query string false "ID phòng ban lọc"
// @Success 200 {object} response.SuccessResponse{data=TaskReportResponse}
// @Router /reports/tasks [get]
func (h *Handler) GetTaskReport(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	var filter TaskReportFilter
	if err := c.QueryParser(&filter); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_QUERY", "Invalid query parameters", err.Error())
	}

	report, err := h.svc.GetTaskReport(orgID, filter)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_REPORT_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, report)
}

// GetDepartmentPerformance godoc
// @Summary Báo cáo hiệu suất theo phòng ban
// @Description Đánh giá tổng hợp khối lượng công việc, tỷ lệ hoàn thành và quá hạn theo từng phòng ban
// @Tags Reports
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]DepartmentPerformanceItem}
// @Router /reports/departments [get]
func (h *Handler) GetDepartmentPerformance(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing organization context", nil)
	}

	performance, err := h.svc.GetDepartmentPerformance(orgID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_PERFORMANCE_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, performance)
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	// Dashboard Group
	dash := router.Group("/dashboard", authMiddleware)
	dash.Get("/summary", h.GetDashboardSummary)
	dash.Get("/my-tasks", h.GetMyTasks)

	// Reports Group
	reps := router.Group("/reports", authMiddleware)
	reps.Get("/tasks", h.GetTaskReport)
	reps.Get("/departments", h.GetDepartmentPerformance)
}

