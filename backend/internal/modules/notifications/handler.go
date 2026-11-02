package notifications

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"

	_ "taskmgmt/internal/model"
	"taskmgmt/internal/platform/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// List godoc
// @Summary Danh sách thông báo
// @Description Lấy danh sách thông báo của người dùng hiện tại có phân trang và lọc chưa đọc
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param page query int false "Trang (mặc định 1)"
// @Param page_size query int false "Số lượng (mặc định 20)"
// @Param unread_only query bool false "Chỉ lấy thông báo chưa đọc"
// @Success 200 {object} response.SuccessResponse{data=[]model.Notification,meta=response.PaginationMeta}
// @Failure 401 {object} response.ErrorResponse
// @Router /notifications [get]
func (h *Handler) List(c *fiber.Ctx) error {
	recipientID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	var filter NotificationFilter
	if err := c.QueryParser(&filter); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_QUERY", "Invalid query parameters", err.Error())
	}

	list, total, err := h.svc.List(recipientID, filter)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "LIST_NOTIFICATIONS_FAILED", err.Error(), nil)
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

// GetUnreadCount godoc
// @Summary Số lượng thông báo chưa đọc
// @Description Lấy tổng số thông báo chưa đọc của người dùng hiện tại
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=UnreadCountResponse}
// @Failure 401 {object} response.ErrorResponse
// @Router /notifications/unread-count [get]
func (h *Handler) GetUnreadCount(c *fiber.Ctx) error {
	recipientID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	count, err := h.svc.GetUnreadCount(recipientID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "GET_UNREAD_COUNT_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, UnreadCountResponse{UnreadCount: count})
}

// MarkAsRead godoc
// @Summary Đánh dấu một thông báo đã đọc
// @Description Đánh dấu thông báo cụ thể là đã đọc
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Param notificationId path string true "ID thông báo (UUID)"
// @Success 200 {object} response.SuccessResponse{data=model.Notification}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /notifications/{notificationId} [patch]
func (h *Handler) MarkAsRead(c *fiber.Ctx) error {
	recipientID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	notifID, err := uuid.Parse(c.Params("notificationId"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "INVALID_ID", "Invalid notification ID", nil)
	}

	notif, err := h.svc.MarkAsRead(recipientID, notifID)
	if err != nil {
		return response.Error(c, fiber.StatusNotFound, "NOTIFICATION_NOT_FOUND", "Notification not found", nil)
	}

	return response.JSON(c, fiber.StatusOK, notif)
}

// MarkAllAsRead godoc
// @Summary Đánh dấu tất cả thông báo đã đọc
// @Description Chuyển toàn bộ thông báo chưa đọc của người dùng sang trạng thái đã đọc
// @Tags Notifications
// @Security BearerAuth
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=MarkAllReadResponse}
// @Failure 401 {object} response.ErrorResponse
// @Router /notifications/read-all [post]
func (h *Handler) MarkAllAsRead(c *fiber.Ctx) error {
	recipientID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	count, err := h.svc.MarkAllAsRead(recipientID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "MARK_ALL_READ_FAILED", err.Error(), nil)
	}

	return response.JSON(c, fiber.StatusOK, MarkAllReadResponse{
		Success:      true,
		UpdatedCount: count,
	})
}

// Stream godoc
// @Summary Server-Sent Events (SSE) đẩy thông báo thời gian thực
// @Description Mở kết nối SSE lắng nghe thông báo tức thời
// @Tags Notifications
// @Security BearerAuth
// @Produce text/event-stream
// @Router /notifications/stream [get]
func (h *Handler) Stream(c *fiber.Ctx) error {
	recipientID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return response.Error(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "Missing user context", nil)
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	notifChan := h.svc.Subscribe(recipientID)

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		defer h.svc.Unsubscribe(recipientID, notifChan)

		// Send initial keep-alive ping
		_, _ = fmt.Fprintf(w, ": connected\n\n")
		_ = w.Flush()

		for notif := range notifChan {
			data, _ := json.Marshal(notif)
			_, err := fmt.Fprintf(w, "event: notification\ndata: %s\n\n", string(data))
			if err != nil {
				break
			}
			if err := w.Flush(); err != nil {
				break
			}
		}
	}))

	return nil
}

func RegisterRoutes(router fiber.Router, h *Handler, authMiddleware fiber.Handler) {
	notifs := router.Group("/notifications", authMiddleware)

	notifs.Get("/", h.List)
	notifs.Get("/unread-count", h.GetUnreadCount)
	notifs.Patch("/:notificationId", h.MarkAsRead)
	notifs.Post("/read-all", h.MarkAllAsRead)
	notifs.Get("/stream", h.Stream)
}
