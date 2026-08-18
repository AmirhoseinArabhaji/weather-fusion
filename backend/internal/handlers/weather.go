package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/internal/cache"
	"github.com/amirhosein/weather-fusion/internal/config"
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
	cache     cache.Cache
	cfg       *config.Config
	log       *slog.Logger
}

// NewWeatherHandler creates a WeatherHandler.
func NewWeatherHandler(provs []providers.WeatherProvider, llmService llm.LLMService, cacheClient cache.Cache, cfg *config.Config, log *slog.Logger) *WeatherHandler {
	return &WeatherHandler{providers: provs, llm: llmService, cache: cacheClient, cfg: cfg, log: log.With("handler", "weather")}
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
	key := cacheKey("current", req)

	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	if h.tryServeFromCache(c, key) {
		return
	}
	if !h.checkRateLimit(c) {
		return
	}

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

	var recorded []cachedSSEEvent
	emit := func(name string, payload interface{}) {
		if b, err := json.Marshal(payload); err == nil {
			recorded = append(recorded, cachedSSEEvent{Event: name, Data: b})
		}
		c.SSEvent(name, payload)
	}

	var observations []*models.WeatherObservation
	clientGone := c.Stream(func(w io.Writer) bool {
		ev, ok := <-events
		if !ok {
			return false
		}
		if ev.Data != nil {
			observations = append(observations, ev.Data)
		}
		emit("provider", ev)
		return true
	})

	if clientGone {
		return
	}

	if len(observations) == 0 {
		emit("error", gin.H{"code": "NO_DATA", "message": "no providers returned data"})
		c.Writer.Flush()
		return
	}

	result := consensus.Merge(observations, len(h.providers))
	emit("consensus", result)
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
	emit("summary", summaryPayload)
	c.Writer.Flush()

	// Detached from the request context: the client may have already
	// disconnected by now (a fresh request for the same location can cancel
	// this one), but the write is still worth completing for the next caller.
	setCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.cache.Set(setCtx, key, recorded, h.cfg.CacheTTL); err != nil {
		h.log.WarnContext(ctx, "failed to cache weather response", "error", err)
	}
}

// dailyProviderEvent is the payload sent for each "provider_daily" SSE event.
type dailyProviderEvent struct {
	Provider string                   `json:"provider"`
	Status   string                   `json:"status"`
	Data     *models.ProviderForecast `json:"data,omitempty"`
	Error    string                   `json:"error,omitempty"`
}

// hourlyProviderEvent is the payload sent for each "provider_hourly" SSE event.
type hourlyProviderEvent struct {
	Provider string                         `json:"provider"`
	Status   string                         `json:"status"`
	Data     *models.ProviderHourlyForecast `json:"data,omitempty"`
	Error    string                         `json:"error,omitempty"`
}

// Forecast godoc
//
//	@Summary     Stream daily and hourly forecast
//	@Description Fans out FetchForecast and FetchHourly to all providers concurrently,
//	@Description streaming "provider_daily"/"provider_hourly" events as each arrives,
//	@Description followed by merged "daily" and "hourly" consensus events.
//	@Tags        weather
//	@Produce     text/event-stream
//	@Param       city query string false "City name"
//	@Param       lat  query number false "Latitude"
//	@Param       lon  query number false "Longitude"
//	@Router      /weather/forecast [get]
func (h *WeatherHandler) Forecast(c *gin.Context) {
	var req models.WeatherRequest
	if !middleware.BindQueryAndValidate(c, &req) {
		return
	}

	if len(h.providers) == 0 {
		response.Error(c, http.StatusServiceUnavailable, "NO_PROVIDERS", "no weather providers configured")
		return
	}

	ctx := c.Request.Context()
	key := cacheKey("forecast", req)

	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	if h.tryServeFromCache(c, key) {
		return
	}
	if !h.checkRateLimit(c) {
		return
	}

	dailyEvents := make(chan dailyProviderEvent, len(h.providers))
	hourlyEvents := make(chan hourlyProviderEvent, len(h.providers))

	var wg sync.WaitGroup
	for _, p := range h.providers {
		wg.Add(2)
		go func(p providers.WeatherProvider) {
			defer wg.Done()
			fc, err := p.FetchForecast(ctx, req)
			if err != nil {
				h.log.ErrorContext(ctx, "forecast fetch failed", "provider", p.Name(), "error", err)
				dailyEvents <- dailyProviderEvent{Provider: p.Name(), Status: "error", Error: err.Error()}
				return
			}
			dailyEvents <- dailyProviderEvent{Provider: p.Name(), Status: "ok", Data: fc}
		}(p)
		go func(p providers.WeatherProvider) {
			defer wg.Done()
			hf, err := p.FetchHourly(ctx, req)
			if err != nil {
				h.log.ErrorContext(ctx, "hourly fetch failed", "provider", p.Name(), "error", err)
				hourlyEvents <- hourlyProviderEvent{Provider: p.Name(), Status: "error", Error: err.Error()}
				return
			}
			hourlyEvents <- hourlyProviderEvent{Provider: p.Name(), Status: "ok", Data: hf}
		}(p)
	}
	go func() {
		wg.Wait()
		close(dailyEvents)
		close(hourlyEvents)
	}()

	var recorded []cachedSSEEvent
	emit := func(name string, payload interface{}) {
		if b, err := json.Marshal(payload); err == nil {
			recorded = append(recorded, cachedSSEEvent{Event: name, Data: b})
		}
		c.SSEvent(name, payload)
	}

	var dailyForecasts []*models.ProviderForecast
	var hourlyForecasts []*models.ProviderHourlyForecast
	dailyDone, hourlyDone := false, false

	clientGone := c.Stream(func(w io.Writer) bool {
		select {
		case ev, ok := <-dailyEvents:
			if !ok {
				dailyDone = true
				dailyEvents = nil // disable this case now that it's closed
			} else {
				if ev.Data != nil {
					dailyForecasts = append(dailyForecasts, ev.Data)
				}
				emit("provider_daily", ev)
			}
		case ev, ok := <-hourlyEvents:
			if !ok {
				hourlyDone = true
				hourlyEvents = nil
			} else {
				if ev.Data != nil {
					hourlyForecasts = append(hourlyForecasts, ev.Data)
				}
				emit("provider_hourly", ev)
			}
		}
		return !(dailyDone && hourlyDone)
	})

	if clientGone {
		return
	}

	if len(dailyForecasts) > 0 {
		emit("daily", consensus.MergeDaily(dailyForecasts))
	} else {
		emit("daily", gin.H{"error": "no providers returned daily data"})
	}
	c.Writer.Flush()

	if len(hourlyForecasts) > 0 {
		emit("hourly", consensus.MergeHourly(hourlyForecasts))
	} else {
		emit("hourly", gin.H{"error": "no providers returned hourly data"})
	}
	c.Writer.Flush()

	setCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.cache.Set(setCtx, key, recorded, h.cfg.CacheTTL); err != nil {
		h.log.WarnContext(ctx, "failed to cache weather response", "error", err)
	}
}
