package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/pkg/response"
)

// Recovery returns a Gin middleware that recovers from panics and writes a
// structured JSON 500 response instead of crashing the server.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Error("panic recovered",
					"error", err,
					"stack", string(stack),
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, response.Envelope{
					Success: false,
					Error: &response.ErrorBody{
						Code:    "INTERNAL_ERROR",
						Message: "an unexpected error occurred",
					},
				})
			}
		}()
		c.Next()
	}
}
