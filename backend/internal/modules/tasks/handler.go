package tasks

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	_ "taskmgmt/internal/model"
	appErrors "taskmgmt/internal/platform/errors"
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

// CreateTask godoc
// @Summary Tạo nhiệm vụ gốc
// @Description Tạo một nhiệm vụ gốc mới và khởi tạo phân công ban đầu
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "Dữ liệu tạo nhiệm vụ"
// @Success 201 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /tasks [post]
func (h *Handler) CreateTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	var req CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid task fields", err.Error())
	}

	task, err := h.svc.CreateTask(orgID, actorID, req)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "CREATE_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, task)
}

// CreateChildTask godoc
// @Summary Tạo công việc con
// @Description Tạo công việc con theo cây phân cấp (ràng buộc thời hạn nằm trong hạn cha)
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc cha (UUID)"
// @Param request body CreateChildTaskRequest true "Dữ liệu công việc con"
// @Success 201 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 422 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /tasks/{taskId}/children [post]
func (h *Handler) CreateChildTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	parentID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req CreateChildTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid child task fields", err.Error())
	}

	child, err := h.svc.CreateChildTask(orgID, actorID, parentID, req)
	if err != nil {
		if errors.Is(err, appErrors.ErrParentDueExceeded) {
			return response.Error(c, fiber.StatusUnprocessableEntity, "TASK_CHILD_DUE_DATE_OUT_OF_RANGE",
				"Hạn hoàn thành công việc con không được vượt hạn công việc cha", nil)
		}
		if errors.Is(err, appErrors.ErrParentStartExceeded) {
			return response.Error(c, fiber.StatusUnprocessableEntity, "TASK_CHILD_START_DATE_OUT_OF_RANGE",
				"Thời gian bắt đầu công việc con không được trước công việc cha", nil)
		}
		if errors.Is(err, appErrors.ErrTaskCompleted) {
			return response.Error(c, fiber.StatusConflict, "PARENT_TASK_ALREADY_CLOSED",
				"Không thể tạo công việc con dưới công việc đã hoàn thành hoặc đã hủy", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "CREATE_CHILD_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, child)
}

// ListTasks godoc
// @Summary Danh sách công việc
// @Description Lấy danh sách công việc có lọc, phân trang và sắp xếp
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param page query int false "Trang hiện tại (mặc định 1)"
// @Param page_size query int false "Số lượng mỗi trang (mặc định 20, tối đa 100)"
// @Param q query string false "Tìm kiếm theo tiêu đề hoặc mã"
// @Param status query string false "Lọc theo trạng thái"
// @Param priority query int false "Lọc theo mức độ ưu tiên"
// @Param overdue query bool false "Chỉ lấy công việc quá hạn"
// @Param sort_by query string false "Sắp xếp theo trường (created_at, due_at, priority...)"
// @Param sort_order query string false "Thứ tự sắp xếp (asc hoặc desc)"
// @Success 200 {object} response.SuccessResponse{data=[]model.Task,meta=response.PaginationMeta}
// @Failure 401 {object} response.ErrorResponse
// @Router /tasks [get]
func (h *Handler) ListTasks(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	var filter TaskFilter
	if err := c.QueryParser(&filter); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_QUERY", "Invalid filter query parameters", err.Error())
	}

	list, total, err := h.svc.ListTasks(orgID, filter)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "LIST_TASKS_FAILED", err.Error(), nil)
	}

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	meta := response.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}

	return response.JSONWithMeta(c, fiber.StatusOK, list, meta)
}

// GetTask godoc
// @Summary Chi tiết công việc
// @Description Lấy thông tin chi tiết một công việc kèm phân công
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId} [get]
func (h *Handler) GetTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	task, err := h.svc.GetTaskByID(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// GetTaskChildren godoc
// @Summary Danh sách công việc con trực tiếp
// @Description Lấy danh sách các công việc con trực tiếp của một nhiệm vụ
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc cha (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.Task}
// @Router /tasks/{taskId}/children [get]
func (h *Handler) GetTaskChildren(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	children, err := h.svc.GetTaskChildren(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_CHILDREN_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, children)
}

// UpdateTask godoc
// @Summary Cập nhật công việc (Optimistic Locking)
// @Description Sửa thông tin công việc, bắt buộc truyền trường version để chống ghi đè đồng thời
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body UpdateTaskRequest true "Dữ liệu cập nhật kèm version"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /tasks/{taskId} [patch]
func (h *Handler) UpdateTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Version is required for update", err.Error())
	}

	task, err := h.svc.UpdateTask(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		if errors.Is(err, ErrVersionConflict) {
			return response.Error(c, fiber.StatusConflict, "CONFLICT_VERSION",
				"Dữ liệu công việc đã bị thay đổi bởi người khác, vui lòng tải lại", nil)
		}
		if errors.Is(err, appErrors.ErrParentDueExceeded) {
			return response.Error(c, fiber.StatusConflict, "TASK_DUE_CONFLICT",
				"Hạn hoàn thành mới làm ảnh hưởng hạn công việc con hoặc vượt hạn cha", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "UPDATE_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// SubmitProgress godoc
// @Summary Báo cáo tiến độ
// @Description Gửi báo cáo tiến độ và kích hoạt tự động tính toán tổng hợp tiến độ cha
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateTaskUpdateRequest true "Nội dung báo cáo tiến độ"
// @Success 201 {object} response.SuccessResponse{data=model.TaskUpdate}
// @Failure 409 {object} response.ErrorResponse
// @Router /tasks/{taskId}/updates [post]
func (h *Handler) SubmitProgress(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req CreateTaskUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Missing progress content or value", err.Error())
	}

	update, err := h.svc.SubmitProgress(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		if errors.Is(err, appErrors.ErrTaskCompleted) {
			return response.Error(c, fiber.StatusConflict, "TASK_ALREADY_CLOSED", "Task is already completed or cancelled", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "SUBMIT_PROGRESS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, update)
}

// ReviewTask godoc
// @Summary Nghiệm thu hoặc yêu cầu bổ sung
// @Description Đánh giá kết quả công việc (chỉ approved khi toàn bộ việc con bắt buộc đã hoàn thành)
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateTaskReviewRequest true "Quyết định nghiệm thu"
// @Success 201 {object} response.SuccessResponse{data=model.TaskReview}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 422 {object} response.ErrorResponse
// @Router /tasks/{taskId}/reviews [post]
func (h *Handler) ReviewTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req CreateTaskReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Decision must be approved or revision_required", err.Error())
	}

	review, err := h.svc.ReviewTask(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		if errors.Is(err, appErrors.ErrReviewIncomplete) {
			return response.Error(c, fiber.StatusUnprocessableEntity, "INCOMPLETE_CHILDREN",
				"Không thể nghiệm thu khi các công việc con bắt buộc chưa hoàn thành", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "REVIEW_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, review)
}

// ReopenTask godoc
// @Summary Mở lại công việc đã hoàn thành
// @Description Chỉ người nghiệm thu gần nhất hoặc cấp quản lý cao hơn được phép; bắt buộc có lý do
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body ReopenTaskRequest true "Lý do mở lại"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/reopen [post]
func (h *Handler) ReopenTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req ReopenTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Reason is required (min 5 characters)", err.Error())
	}

	task, err := h.svc.ReopenTask(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		if errors.Is(err, appErrors.ErrReopenUnauthorized) {
			return response.Error(c, fiber.StatusForbidden, "REOPEN_UNAUTHORIZED",
				"Chỉ người nghiệm thu gần nhất hoặc cấp quản lý cao hơn mới được mở lại", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "REOPEN_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// CancelTask godoc
// @Summary Hủy công việc
// @Description Hủy công việc theo thẩm quyền có kèm lý do
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CancelTaskRequest true "Lý do hủy"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/cancellations [post]
func (h *Handler) CancelTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req CancelTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Reason is required", err.Error())
	}

	task, err := h.svc.CancelTask(orgID, actorID, taskID, req.Reason)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "CANCEL_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// GetTaskTree godoc
// @Summary Cây phân cấp công việc
// @Description Lấy cây phân cấp hoàn chỉnh từ công việc hiện tại xuống các cấp con cháu
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Router /tasks/{taskId}/tree [get]
func (h *Handler) GetTaskTree(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	tree, err := h.svc.GetTaskTree(orgID, taskID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task tree not found", nil)
	}

	return response.JSON(c, fiber.StatusOK, tree)
}

// GetTaskUpdates godoc
// @Summary Lịch sử báo cáo tiến độ
// @Description Lấy toàn bộ các lần báo cáo tiến độ kèm người gửi và tệp đính kèm
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskUpdate}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/updates [get]
func (h *Handler) GetTaskUpdates(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	updates, err := h.svc.GetTaskUpdates(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_UPDATES_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, updates)
}

// GetTaskReviews godoc
// @Summary Lịch sử nghiệm thu
// @Description Lấy toàn bộ lịch sử các lần nghiệm thu hoặc yêu cầu bổ sung
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskReview}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/reviews [get]
func (h *Handler) GetTaskReviews(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	reviews, err := h.svc.GetTaskReviews(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_REVIEWS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, reviews)
}

// GetTaskEvents godoc
// @Summary Audit timeline
// @Description Lấy toàn bộ lịch sử sự kiện kiểm toán nghiệp vụ của công việc
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskEvent}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/events [get]
func (h *Handler) GetTaskEvents(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	events, err := h.svc.GetTaskEvents(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_EVENTS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, events)
}

// GetTaskComments godoc
// @Summary Danh sách bình luận
// @Description Lấy danh sách trao đổi, bình luận trong công việc
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskComment}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/comments [get]
func (h *Handler) GetTaskComments(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	comments, err := h.svc.GetTaskComments(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_COMMENTS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, comments)
}

// CreateComment godoc
// @Summary Thêm bình luận
// @Description Thêm bình luận mới vào công việc
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateCommentRequest true "Nội dung bình luận"
// @Success 201 {object} response.SuccessResponse{data=model.TaskComment}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/comments [post]
func (h *Handler) CreateComment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Comment content is required", err.Error())
	}

	comment, err := h.svc.AddComment(orgID, actorID, taskID, req.Content)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "CREATE_COMMENT_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, comment)
}

// DeleteTask godoc
// @Summary Xóa/Hủy công việc theo chính sách REST
// @Description Xóa mềm hoặc hủy công việc theo chính sách RESTful
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId} [delete]
func (h *Handler) DeleteTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	task, err := h.svc.CancelTask(orgID, actorID, taskID, "Deleted via DELETE /tasks/:taskId")
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "DELETE_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// GetAssignments godoc
// @Summary Danh sách phân công công việc
// @Description Lấy danh sách người hoặc phòng ban được giao việc
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskAssignment}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/assignments [get]
func (h *Handler) GetAssignments(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	assignments, err := h.svc.GetAssignments(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_ASSIGNMENTS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, assignments)
}

// AddAssignment godoc
// @Summary Phân công thêm người hoặc phòng ban
// @Description Giao công việc cho người hoặc phòng ban mới
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateAssignmentRequest true "Dữ liệu phân công"
// @Success 201 {object} response.SuccessResponse{data=model.TaskAssignment}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/assignments [post]
func (h *Handler) AddAssignment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req CreateAssignmentRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid assignment input", err.Error())
	}

	assignment, err := h.svc.AddAssignment(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "ADD_ASSIGNMENT_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, assignment)
}

// DeleteAssignment godoc
// @Summary Xóa phân công công việc
// @Description Gỡ bỏ một phân công khỏi công việc
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param assignmentId path string true "ID phân công (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/assignments/{assignmentId} [delete]
func (h *Handler) DeleteAssignment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	assignmentID, err := uuid.Parse(c.Params("assignmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ASSIGNMENT_ID", "Invalid assignment ID in URL", nil)
	}

	if err := h.svc.DeleteAssignment(orgID, taskID, assignmentID); err != nil {
		if errors.Is(err, ErrTaskNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Task or assignment not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "DELETE_ASSIGNMENT_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"message": "Assignment removed successfully",
	})
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	tasks := router.Group("/tasks", authMiddleware)

	tasks.Post("/", h.CreateTask)
	tasks.Get("/", h.ListTasks)
	tasks.Get("/:taskId", h.GetTask)
	tasks.Patch("/:taskId", h.UpdateTask)
	tasks.Delete("/:taskId", h.DeleteTask)

	tasks.Get("/:taskId/tree", h.GetTaskTree)
	tasks.Get("/:taskId/children", h.GetTaskChildren)
	tasks.Post("/:taskId/children", h.CreateChildTask)

	tasks.Get("/:taskId/assignments", h.GetAssignments)
	tasks.Post("/:taskId/assignments", h.AddAssignment)
	tasks.Delete("/:taskId/assignments/:assignmentId", h.DeleteAssignment)

	tasks.Get("/:taskId/updates", h.GetTaskUpdates)
	tasks.Post("/:taskId/updates", h.SubmitProgress)

	tasks.Get("/:taskId/reviews", h.GetTaskReviews)
	tasks.Post("/:taskId/reviews", h.ReviewTask)

	tasks.Get("/:taskId/events", h.GetTaskEvents)
	tasks.Get("/:taskId/comments", h.GetTaskComments)
	tasks.Post("/:taskId/comments", h.CreateComment)

	tasks.Post("/:taskId/reopen", h.ReopenTask)
	tasks.Post("/:taskId/cancellations", h.CancelTask)
}

