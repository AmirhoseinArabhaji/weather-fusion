// Package handlers contains the HTTP handler layer (Gin controllers).
package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/pkg/response"
)

// HealthResponse describes the /health endpoint payload.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
}

// HealthHandler holds dependencies for the health endpoint.
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{version: version}
}

// Check godoc
//
//	@Summary     Health check
//	@Description Returns server liveness and basic runtime metadata
//	@Tags        system
//	@Produce     json
//	@Success     200 {object} response.Envelope{data=HealthResponse}
//	@Router      /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	c.JSON(http.StatusOK, response.Envelope{
		Success: true,
		Data: HealthResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC(),
			Version:   h.version,
			GoVersion: runtime.Version(),
		},
	})
}
