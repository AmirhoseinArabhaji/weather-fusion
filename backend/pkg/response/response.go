// Package response provides helpers for writing consistent JSON API responses.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type Meta struct {
	Page       int `json:"page,omitempty"`
	PageSize   int `json:"page_size,omitempty"`
	TotalItems int `json:"total_items,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// Success helpers
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Paginated(c *gin.Context, data interface{}, meta Meta) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data, Meta: &meta})
}

// Error helpers
func Error(c *gin.Context, statusCode int, code, message string) {
	c.JSON(statusCode, Envelope{
		Success: false,
		Error:   &ErrorBody{Code: code, Message: message},
	})
}

func ErrorWithDetails(c *gin.Context, statusCode int, code, message string, details interface{}) {
	c.JSON(statusCode, Envelope{
		Success: false,
		Error:   &ErrorBody{Code: code, Message: message, Details: details},
	})
}

func InternalError(c *gin.Context) {
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
}
