package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/internal/consensus"
	"github.com/amirhosein/weather-fusion/internal/middleware"
	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
	"github.com/amirhosein/weather-fusion/pkg/response"
)

// WeatherHandler streams weather data fanned out across all configured providers.
type WeatherHandler struct {
	providers []providers.WeatherProvider
	log       *slog.Logger
}

// NewWeatherHandler creates a WeatherHandler.
func NewWeatherHandler(provs []providers.WeatherProvider, log *slog.Logger) *WeatherHandler {
	return &WeatherHandler{providers: provs, log: log.With("handler", "weather")}
}

// providerEvent is the payload sent for each "provider" SSE event.
type providerEvent struct {
	Provider string                     `json:"provider"`
	Status   string                     `json:"status"`
	Data     *models.WeatherObservation `json:"data,omitempty"`
	Error    string                     `json:"error,omitempty"`
}

// Current godoc
//
//	@Summary     Stream current weather
//	@Description Fans out to all providers concurrently and streams each result as it
//	@Description arrives via SSE, followed by a final merged consensus event.
//	@Tags        weather
//	@Produce     text/event-stream
//	@Param       city query string false "City name"
//	@Param       lat  query number false "Latitude"
//	@Param       lon  query number false "Longitude"
//	@Router      /weather/current [get]
func (h *WeatherHandler) Current(c *gin.Context) {
	var req models.WeatherRequest
	if !middleware.BindQueryAndValidate(c, &req) {
		return
	}

	if len(h.providers) == 0 {
		response.Error(c, http.StatusServiceUnavailable, "NO_PROVIDERS", "no weather providers configured")
		return
	}

	ctx := c.Request.Context()
	events := make(chan providerEvent, len(h.providers))

	var wg sync.WaitGroup
	for _, p := range h.providers {
		wg.Add(1)
		go func(p providers.WeatherProvider) {
			defer wg.Done()
			obs, err := p.FetchCurrent(ctx, req)
			if err != nil {
				h.log.ErrorContext(ctx, "provider fetch failed", "provider", p.Name(), "error", err)
				events <- providerEvent{Provider: p.Name(), Status: "error", Error: err.Error()}
				return
			}
			events <- providerEvent{Provider: p.Name(), Status: "ok", Data: obs}
		}(p)
	}
	go func() {
		wg.Wait()
		close(events)
	}()

	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	var observations []*models.WeatherObservation
	clientGone := c.Stream(func(w io.Writer) bool {
		ev, ok := <-events
		if !ok {
			return false
		}
		if ev.Data != nil {
			observations = append(observations, ev.Data)
		}
		c.SSEvent("provider", ev)
		return true
	})

	if clientGone {
		return
	}

	if len(observations) == 0 {
		c.SSEvent("error", gin.H{"code": "NO_DATA", "message": "no providers returned data"})
		c.Writer.Flush()
		return
	}

	result := consensus.Merge(observations, len(h.providers))
	c.SSEvent("consensus", result)
	c.Writer.Flush()
}
