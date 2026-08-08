package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/internal/consensus"
	"github.com/amirhosein/weather-fusion/internal/llm"
	"github.com/amirhosein/weather-fusion/internal/middleware"
	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
	"github.com/amirhosein/weather-fusion/pkg/response"
)

// WeatherHandler streams weather data fanned out across all configured providers.
type WeatherHandler struct {
	providers []providers.WeatherProvider
	llm       llm.LLMService
	log       *slog.Logger
}

// NewWeatherHandler creates a WeatherHandler.
func NewWeatherHandler(provs []providers.WeatherProvider, llmService llm.LLMService, log *slog.Logger) *WeatherHandler {
	return &WeatherHandler{providers: provs, llm: llmService, log: log.With("handler", "weather")}
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
//	@Description arrives via SSE, followed by a merged consensus event, then a
//	@Description separate summary event once the LLM interpretation is ready.
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

	// Sent separately from "consensus" so the numeric data isn't held up
	// waiting on the LLM call. Always fires exactly once (summary, error, or
	// "unavailable") so the client has a reliable signal to stop listening.
	summaryPayload := gin.H{}
	if h.llm != nil && h.llm.IsAvailable(ctx) {
		summary, err := h.llm.Summarize(ctx, llm.SummarizeRequest{
			City:        result.Location.City,
			Temperature: result.Temperature,
			Condition:   string(result.Condition),
			Description: fmt.Sprintf("%.0f%% confidence, ±%.1f° spread across %d providers", result.Confidence*100, result.TempStdDev, len(result.Providers)),
		})
		if err != nil {
			h.log.WarnContext(ctx, "llm summarize failed", "error", err)
			summaryPayload["error"] = err.Error()
		} else {
			summaryPayload["summary"] = summary.Content
		}
	} else {
		summaryPayload["error"] = "llm unavailable"
	}
	c.SSEvent("summary", summaryPayload)
	c.Writer.Flush()
}
