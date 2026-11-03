package tasks

import (
	"errors"
	"fmt"

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
// @Description Tạo một nhiệm vụ gốc mới (hỗ trợ lưu bản nháp hoặc giao việc ngay, tự nhận việc)
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
// @Description Lấy danh sách công việc có lọc (trạng thái, đã gắn người, phân trang, sắp xếp)
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param page query int false "Trang hiện tại (mặc định 1)"
// @Param page_size query int false "Số lượng mỗi trang (mặc định 20, tối đa 100)"
// @Param q query string false "Tìm kiếm theo tiêu đề hoặc mã"
// @Param status query string false "Lọc theo trạng thái (draft, unassigned, assigned, in_progress, blocked, submitted, completed, cancelled)"
// @Param is_assigned query bool false "Lọc công việc đã gắn người (true) hoặc chưa gắn (false)"
// @Param priority query int false "Lọc theo mức độ ưu tiên"
// @Param overdue query bool false "Chỉ lấy công việc quá hạn"
// @Param sort_by query string false "Sắp xếp theo trường (created_at, due_at, priority, sort_order...)"
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

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return response.JSONWithMeta(c, fiber.StatusOK, list, response.PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetTask godoc
// @Summary Chi tiết công việc
// @Description Lấy thông tin chi tiết một công việc theo ID
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 401 {object} response.ErrorResponse
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

// UpdateTask godoc
// @Summary Cập nhật công việc (Optimistic Locking)
// @Description Cập nhật tiêu đề, mô tả, ưu tiên hoặc hạn chót
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body UpdateTaskRequest true "Dữ liệu cập nhật kèm Version"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 422 {object} response.ErrorResponse
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
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid task update fields", err.Error())
	}

	task, err := h.svc.UpdateTask(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return response.Error(c, fiber.StatusConflict, "TASK_VERSION_CONFLICT",
				"Dữ liệu công việc đã bị thay đổi bởi người khác, vui lòng tải lại trước khi lưu", nil)
		}
		if errors.Is(err, appErrors.ErrParentDueExceeded) {
			return response.Error(c, fiber.StatusUnprocessableEntity, "TASK_DUE_DATE_OUT_OF_RANGE",
				"Hạn hoàn thành công việc không hợp lệ so với cây phân cấp", nil)
		}
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "UPDATE_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// DeleteTask godoc
// @Summary Xóa bản nháp công việc
// @Description Chỉ cho phép xóa nếu task ở trạng thái bản nháp (draft). Đã giao việc phải dùng chức năng hủy.
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
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

	err = h.svc.DeleteTask(orgID, actorID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		if errors.Is(err, appErrors.ErrCannotDeleteDispatchedTask) {
			return response.Error(c, fiber.StatusBadRequest, "CANNOT_DELETE_DISPATCHED_TASK",
				"Chỉ được xóa công việc ở trạng thái bản nháp (draft). Công việc đã giao vui lòng dùng chức năng hủy (cancel).", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "DELETE_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"message": "Task draft deleted successfully",
	})
}

// PublishTask godoc
// @Summary Phát hành công việc từ bản nháp
// @Description Chuyển trạng thái từ draft sang assigned (nếu có người) hoặc unassigned (chưa có người)
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/publish [post]
func (h *Handler) PublishTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	task, err := h.svc.PublishTask(orgID, actorID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		if errors.Is(err, appErrors.ErrTaskNotDraft) {
			return response.Error(c, fiber.StatusBadRequest, "TASK_NOT_DRAFT", "Công việc không ở trạng thái bản nháp (draft)", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "PUBLISH_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// ClaimTask godoc
// @Summary Tự nhận công việc chưa gán
// @Description Người dùng tự nhận công việc (chuyển sang assigned và tạo phân công chính)
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Router /tasks/{taskId}/claim [post]
func (h *Handler) ClaimTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	task, err := h.svc.ClaimTask(orgID, actorID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		if errors.Is(err, appErrors.ErrTaskAlreadyAssigned) {
			return response.Error(c, fiber.StatusConflict, "TASK_ALREADY_ASSIGNED", "Công việc đã được phân công cho người khác", nil)
		}
		if errors.Is(err, appErrors.ErrTaskCompleted) {
			return response.Error(c, fiber.StatusConflict, "TASK_COMPLETED", "Không thể nhận công việc đã hoàn thành hoặc đã hủy", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "CLAIM_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// BlockTask godoc
// @Summary Báo vướng mắc công việc
// @Description Chuyển trạng thái sang blocked và ghi nhận lý do vướng mắc
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body BlockTaskRequest true "Lý do vướng mắc"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Router /tasks/{taskId}/block [post]
func (h *Handler) BlockTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req BlockTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Reason is required", err.Error())
	}

	task, err := h.svc.BlockTask(orgID, actorID, taskID, req.Reason)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "BLOCK_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// UnblockTask godoc
// @Summary Gỡ vướng mắc công việc
// @Description Chuyển trạng thái từ blocked về in_progress và ghi nhận giải pháp
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body UnblockTaskRequest false "Ghi chú giải quyết"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Router /tasks/{taskId}/unblock [post]
func (h *Handler) UnblockTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req UnblockTaskRequest
	_ = c.BodyParser(&req)

	task, err := h.svc.UnblockTask(orgID, actorID, taskID, req.Note)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "UNBLOCK_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// SubmitTask godoc
// @Summary Nộp công việc chờ nghiệm thu
// @Description Chuyển trạng thái sang submitted để quản lý nghiệm thu
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Router /tasks/{taskId}/submit [post]
func (h *Handler) SubmitTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	task, err := h.svc.SubmitTask(orgID, actorID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "SUBMIT_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// MoveTask godoc
// @Summary Di chuyển công việc (Kanban & Cây công việc)
// @Description Hỗ trợ kéo thả đổi cột Kanban, sắp xếp thứ tự và di chuyển cây công việc kèm kiểm tra chu trình
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body MoveTaskRequest true "Dữ liệu di chuyển"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 422 {object} response.ErrorResponse
// @Router /tasks/{taskId}/move [patch]
func (h *Handler) MoveTask(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req MoveTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Validation failed", err.Error())
	}

	task, err := h.svc.MoveTask(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return response.Error(c, fiber.StatusConflict, "TASK_VERSION_CONFLICT",
				"Dữ liệu công việc đã bị thay đổi bởi người khác", nil)
		}
		if errors.Is(err, appErrors.ErrCyclicDependency) {
			return response.Error(c, fiber.StatusBadRequest, "CYCLIC_DEPENDENCY",
				"Không thể di chuyển công việc vào bên trong chính nó hoặc con cháu của nó", nil)
		}
		if errors.Is(err, appErrors.ErrParentDueExceeded) {
			return response.Error(c, fiber.StatusUnprocessableEntity, "TASK_DUE_DATE_OUT_OF_RANGE",
				"Hạn hoàn thành công việc vượt quá hạn công việc cha mới", nil)
		}
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusBadRequest, "MOVE_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// GetTaskTree godoc
// @Summary Cây phân cấp công việc
// @Description Lấy toàn bộ cây nhiệm vụ cha con đa cấp đệ quy
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc gốc hoặc công việc bất kỳ (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
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
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_TREE_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, tree)
}

// GetTaskChildren godoc
// @Summary Danh sách công việc con trực tiếp
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.Task}
// @Failure 401 {object} response.ErrorResponse
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

// SubmitProgress godoc
// @Summary Báo cáo tiến độ công việc
// @Description Nộp báo cáo tiến độ, tự động tính tỷ trọng lên cha và tự động chuyển submitted khi 100%
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateTaskUpdateRequest true "Dữ liệu tiến độ"
// @Success 201 {object} response.SuccessResponse{data=model.TaskUpdate}
// @Failure 400 {object} response.ErrorResponse
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
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid progress update input", err.Error())
	}

	update, err := h.svc.SubmitProgress(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, appErrors.ErrTaskCompleted) {
			return response.Error(c, fiber.StatusConflict, "TASK_ALREADY_CLOSED",
				"Không thể cập nhật tiến độ cho công việc đã hoàn thành hoặc đã hủy", nil)
		}
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "SUBMIT_PROGRESS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, update)
}

// GetTaskUpdates godoc
// @Summary Lịch sử tiến độ
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskUpdate}
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
		return response.Error(c, fiber.StatusInternalServerError, "GET_UPDATES_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, updates)
}

// ReviewTask godoc
// @Summary Nghiệm thu công việc
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateTaskReviewRequest true "Quyết định phê duyệt/yêu cầu sửa đổi"
// @Success 201 {object} response.SuccessResponse{data=model.TaskReview}
// @Failure 409 {object} response.ErrorResponse
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
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid review input", err.Error())
	}

	review, err := h.svc.ReviewTask(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, appErrors.ErrReviewIncomplete) {
			return response.Error(c, fiber.StatusConflict, "CHILDREN_INCOMPLETE",
				"Không thể nghiệm thu công việc khi các công việc con chưa hoàn thành", nil)
		}
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "REVIEW_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, review)
}

// GetTaskReviews godoc
// @Summary Lịch sử nghiệm thu
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskReview}
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
		return response.Error(c, fiber.StatusInternalServerError, "GET_REVIEWS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, reviews)
}

// ReopenTask godoc
// @Summary Mở lại công việc đã hoàn thành
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body ReopenTaskRequest true "Lý do mở lại"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
// @Failure 403 {object} response.ErrorResponse
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
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Reason is required", err.Error())
	}

	task, err := h.svc.ReopenTask(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, appErrors.ErrReopenUnauthorized) {
			return response.Error(c, fiber.StatusForbidden, "REOPEN_FORBIDDEN",
				"Chỉ người nghiệm thu gần nhất hoặc cấp trên mới có quyền mở lại công việc này", nil)
		}
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "REOPEN_TASK_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, task)
}

// CancelTask godoc
// @Summary Hủy công việc
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CancelTaskRequest true "Lý do hủy"
// @Success 200 {object} response.SuccessResponse{data=model.Task}
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

// GetTaskEvents godoc
// @Summary Dòng sự kiện (Audit Log)
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]TaskEventItem}
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
		return response.Error(c, fiber.StatusInternalServerError, "GET_EVENTS_FAILED", err.Error(), nil)
	}

	var items []TaskEventItem
	for _, e := range events {
		var dataMap map[string]interface{}
		_ = e.Data.UnmarshalJSON([]byte(e.Data))
		items = append(items, TaskEventItem{
			ID:        e.ID,
			TaskID:    e.TaskID,
			ActorID:   e.ActorID,
			EventType: e.EventType,
			Data:      dataMap,
			CreatedAt: e.CreatedAt,
		})
	}

	return response.JSON(c, fiber.StatusOK, items)
}

// GetTaskComments godoc
// @Summary Danh sách bình luận
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskComment}
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
		return response.Error(c, fiber.StatusInternalServerError, "GET_COMMENTS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, comments)
}

// CreateComment godoc
// @Summary Thêm bình luận
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateCommentRequest true "Nội dung bình luận"
// @Success 201 {object} response.SuccessResponse{data=model.TaskComment}
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

// GetAssignments godoc
// @Summary Danh sách phân công công việc
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskAssignment}
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
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body CreateAssignmentRequest true "Dữ liệu phân công"
// @Success 201 {object} response.SuccessResponse{data=model.TaskAssignment}
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
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param assignmentId path string true "ID phân công (UUID)"
// @Success 200 {object} response.SuccessResponse
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

// --- Attachment Handlers ---

// PresignAttachmentUpload godoc
// @Summary Lấy URL tải lên tệp đính kèm (Bước 1)
// @Description Hỗ trợ cả MinIO/S3 (Presigned PUT) hoặc Local storage (URL upload backend)
// @Tags Attachments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body PresignUploadRequest true "Thông tin tệp"
// @Success 200 {object} response.SuccessResponse{data=PresignUploadResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /tasks/{taskId}/attachments/presign [post]
func (h *Handler) PresignAttachmentUpload(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req PresignUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Invalid upload params", err.Error())
	}

	res, err := h.svc.PresignAttachmentUpload(orgID, actorID, taskID, req)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "PRESIGN_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, res)
}

// ConfirmAttachmentUpload godoc
// @Summary Xác nhận tải lên tệp thành công (Bước 2)
// @Description Dành cho MinIO sau khi client PUT file thành công
// @Tags Attachments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param request body ConfirmUploadRequest true "Xác nhận tệp"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /tasks/{taskId}/attachments/confirm [post]
func (h *Handler) ConfirmAttachmentUpload(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	var req ConfirmUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON payload", err.Error())
	}
	if err := h.validate.Struct(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Attachment ID is required", err.Error())
	}

	if err := h.svc.ConfirmAttachmentUpload(orgID, actorID, taskID, req.AttachmentID, req.Checksum); err != nil {
		if errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrAttachmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Task or attachment not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "CONFIRM_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"message": "Attachment confirmed successfully",
	})
}

// DirectUploadAttachment godoc
// @Summary Tải lên tệp đính kèm trực tiếp (Multipart)
// @Description Tải tệp lên máy chủ backend Golang trực tiếp (chế độ local storage)
// @Tags Attachments
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param file formData file true "Tệp đính kèm"
// @Success 201 {object} response.SuccessResponse{data=model.TaskAttachment}
// @Failure 400 {object} response.ErrorResponse
// @Router /tasks/{taskId}/attachments/upload [post]
func (h *Handler) DirectUploadAttachment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "FILE_REQUIRED", "Form file 'file' is required", err.Error())
	}

	src, err := file.Open()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "FILE_OPEN_FAILED", err.Error(), nil)
	}
	defer src.Close()

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	att, err := h.svc.DirectUploadAttachment(orgID, actorID, taskID, file.Filename, contentType, file.Size, src)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "UPLOAD_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusCreated, att)
}

// GetTaskAttachments godoc
// @Summary Danh sách tệp đính kèm
// @Tags Attachments
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Success 200 {object} response.SuccessResponse{data=[]model.TaskAttachment}
// @Router /tasks/{taskId}/attachments [get]
func (h *Handler) GetTaskAttachments(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	attachments, err := h.svc.GetTaskAttachments(orgID, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return response.Error(c, fiber.StatusNotFound, "TASK_NOT_FOUND", "Task not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "GET_ATTACHMENTS_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, attachments)
}

// DownloadAttachment godoc
// @Summary Tải xuống tệp đính kèm
// @Description Stream trực tiếp từ máy chủ backend (Local) hoặc Redirect sang URL MinIO/S3
// @Tags Attachments
// @Security BearerAuth
// @Produce octet-stream
// @Param taskId path string true "ID công việc (UUID)"
// @Param attachmentId path string true "ID tệp đính kèm (UUID)"
// @Success 200 {file} binary
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/attachments/{attachmentId}/download [get]
func (h *Handler) DownloadAttachment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	attID, err := uuid.Parse(c.Params("attachmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ATTACHMENT_ID", "Invalid attachment ID in URL", nil)
	}

	att, rc, presignedURL, err := h.svc.DownloadAttachment(orgID, taskID, attID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrAttachmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Task or attachment not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "DOWNLOAD_FAILED", err.Error(), nil)
	}

	if presignedURL != "" {
		return c.Redirect(presignedURL, fiber.StatusTemporaryRedirect)
	}

	c.Set("Content-Type", att.ContentType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", att.FileName))
	return c.SendStream(rc, int(att.SizeBytes))
}

// DeleteAttachment godoc
// @Summary Xóa tệp đính kèm
// @Tags Attachments
// @Security BearerAuth
// @Produce json
// @Param taskId path string true "ID công việc (UUID)"
// @Param attachmentId path string true "ID tệp đính kèm (UUID)"
// @Success 200 {object} response.SuccessResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /tasks/{taskId}/attachments/{attachmentId} [delete]
func (h *Handler) DeleteAttachment(c *fiber.Ctx) error {
	orgID, ok := c.Locals("org_id").(uuid.UUID)
	actorID, ok2 := c.Locals("user_id").(uuid.UUID)
	if !ok || !ok2 {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user/org context", nil)
	}

	taskID, err := uuid.Parse(c.Params("taskId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_TASK_ID", "Invalid task ID in URL", nil)
	}

	attID, err := uuid.Parse(c.Params("attachmentId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ATTACHMENT_ID", "Invalid attachment ID in URL", nil)
	}

	if err := h.svc.DeleteAttachment(orgID, actorID, taskID, attID); err != nil {
		if errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrAttachmentNotFound) {
			return response.Error(c, fiber.StatusNotFound, "NOT_FOUND", "Task or attachment not found", nil)
		}
		return response.Error(c, fiber.StatusInternalServerError, "DELETE_ATTACHMENT_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, fiber.Map{
		"message": "Attachment deleted successfully",
	})
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	tasks := router.Group("/tasks", authMiddleware)

	// CRUD & State Lifecycle
	tasks.Post("/", h.CreateTask)
	tasks.Get("/", h.ListTasks)
	tasks.Get("/:taskId", h.GetTask)
	tasks.Patch("/:taskId", h.UpdateTask)
	tasks.Delete("/:taskId", h.DeleteTask)

	tasks.Post("/:taskId/publish", h.PublishTask)
	tasks.Post("/:taskId/claim", h.ClaimTask)
	tasks.Post("/:taskId/block", h.BlockTask)
	tasks.Post("/:taskId/unblock", h.UnblockTask)
	tasks.Post("/:taskId/submit", h.SubmitTask)
	tasks.Patch("/:taskId/move", h.MoveTask)

	// Hierarchy
	tasks.Get("/:taskId/tree", h.GetTaskTree)
	tasks.Get("/:taskId/children", h.GetTaskChildren)
	tasks.Post("/:taskId/children", h.CreateChildTask)

	// Assignments
	tasks.Get("/:taskId/assignments", h.GetAssignments)
	tasks.Post("/:taskId/assignments", h.AddAssignment)
	tasks.Delete("/:taskId/assignments/:assignmentId", h.DeleteAssignment)

	// Updates & Progress
	tasks.Get("/:taskId/updates", h.GetTaskUpdates)
	tasks.Post("/:taskId/updates", h.SubmitProgress)

	// Reviews
	tasks.Get("/:taskId/reviews", h.GetTaskReviews)
	tasks.Post("/:taskId/reviews", h.ReviewTask)

	// Events & Comments
	tasks.Get("/:taskId/events", h.GetTaskEvents)
	tasks.Get("/:taskId/comments", h.GetTaskComments)
	tasks.Post("/:taskId/comments", h.CreateComment)

	// Reopen & Cancellations
	tasks.Post("/:taskId/reopen", h.ReopenTask)
	tasks.Post("/:taskId/cancellations", h.CancelTask)

	// Attachments
	tasks.Post("/:taskId/attachments/presign", h.PresignAttachmentUpload)
	tasks.Post("/:taskId/attachments/confirm", h.ConfirmAttachmentUpload)
	tasks.Post("/:taskId/attachments/upload", h.DirectUploadAttachment)
	tasks.Get("/:taskId/attachments", h.GetTaskAttachments)
	tasks.Get("/:taskId/attachments/:attachmentId/download", h.DownloadAttachment)
	tasks.Delete("/:taskId/attachments/:attachmentId", h.DeleteAttachment)
}
