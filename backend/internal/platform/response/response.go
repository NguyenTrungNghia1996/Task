package response

import (
	"github.com/gofiber/fiber/v2"
)

type ErrorDetails struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta interface{} `json:"meta,omitempty"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func JSON(c *fiber.Ctx, statusCode int, data interface{}) error {
	return c.Status(statusCode).JSON(SuccessResponse{
		Data: data,
	})
}

func JSONWithMeta(c *fiber.Ctx, statusCode int, data interface{}, meta interface{}) error {
	return c.Status(statusCode).JSON(SuccessResponse{
		Data: data,
		Meta: meta,
	})
}

func Error(c *fiber.Ctx, statusCode int, code string, message string, details interface{}) error {
	reqID, _ := c.Locals("requestid").(string)
	return c.Status(statusCode).JSON(ErrorResponse{
		Error: ErrorDetails{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: reqID,
		},
	})
}

