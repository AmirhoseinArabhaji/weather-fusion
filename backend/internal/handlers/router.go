package handlers

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/internal/config"
	"github.com/amirhosein/weather-fusion/internal/middleware"
	"github.com/amirhosein/weather-fusion/internal/providers"
)

// NewRouter constructs and returns a fully-configured Gin engine.
func NewRouter(
	cfg *config.Config,
	log *slog.Logger,
	weatherProviders []providers.WeatherProvider,
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

	weatherHandler := NewWeatherHandler(weatherProviders, log)

	v1 := r.Group("/api/v1")
	v1.GET("/weather/current", weatherHandler.Current)

	for _, info := range r.Routes() {
		log.Debug("route registered", "method", info.Method, "path", info.Path)
	}

	return r
}
