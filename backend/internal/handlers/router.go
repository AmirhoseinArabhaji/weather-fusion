package handlers

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/internal/config"
	"github.com/amirhosein/weather-fusion/internal/middleware"
)

// NewRouter constructs and returns a fully-configured Gin engine.
func NewRouter(
	cfg *config.Config,
	log *slog.Logger,
) *gin.Engine {
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	r.Use(middleware.Recovery(log))
	r.Use(middleware.Logger(log))
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	healthHandler := NewHealthHandler(cfg.AppVersion)
	r.GET("/health", healthHandler.Check)

	v1 := r.Group("/api/v1")
	_ = v1

	for _, info := range r.Routes() {
		log.Debug("route registered", "method", info.Method, "path", info.Path)
	}

	return r
}
