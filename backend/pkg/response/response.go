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
}

type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
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
