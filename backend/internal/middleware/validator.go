package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/amirhosein/weather-fusion/pkg/response"
)

// validate is a shared validator instance (thread-safe).
var validate = validator.New()

// BindAndValidate binds a JSON request body into dst and returns validation errors
// as a structured 400 response. Returns false if the handler should abort.
func BindAndValidate(c *gin.Context, dst interface{}) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		details := formatValidationErrors(err)
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "request validation failed", details)
		c.Abort()
		return false
	}
	return true
}

// BindQueryAndValidate binds query parameters into dst and returns validation errors.
func BindQueryAndValidate(c *gin.Context, dst interface{}) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		details := formatValidationErrors(err)
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "query validation failed", details)
		c.Abort()
		return false
	}
	return true
}

// formatValidationErrors converts validator.ValidationErrors to a human-friendly map.
func formatValidationErrors(err error) map[string]string {
	details := map[string]string{}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			details[fe.Field()] = validationMessage(fe)
		}
	} else {
		details["_"] = err.Error()
	}
	return details
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "this field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "value is too short (min " + fe.Param() + ")"
	case "max":
		return "value is too long (max " + fe.Param() + ")"
	case "oneof":
		return "must be one of: " + fe.Param()
	default:
		return "invalid value"
	}
}
