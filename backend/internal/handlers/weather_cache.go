package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/internal/cache"
	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/pkg/response"
)

// cacheKey builds the Redis key for a weather request. City takes precedence
// over lat/lon (matches WeatherRequest's own required_without=Lat rule);
// lat/lon are rounded to ~1km buckets so nearby GPS reads share a cache entry.
func cacheKey(kind string, req models.WeatherRequest) string {
	units := req.Units
	if units == "" {
		units = "metric"
	}
	sig := fmt.Sprintf("geo:%.2f,%.2f", req.Lat, req.Lon)
	if req.City != "" {
		sig = "city:" + strings.ToLower(strings.TrimSpace(req.City))
	}
	if kind == "forecast" {
		return fmt.Sprintf("weather:forecast:%s:%s:d%d", sig, units, req.Days)
	}
	return fmt.Sprintf("weather:current:%s:%s", sig, units)
}

// cachedSSEEvent is one recorded SSE event, replayed verbatim on a cache hit.
type cachedSSEEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// tryServeFromCache replays a cached event sequence if present. Returns true
// if the response was fully served from cache.
func (h *WeatherHandler) tryServeFromCache(c *gin.Context, key string) bool {
	ctx := c.Request.Context()
	raw, err := h.cache.Get(ctx, key)
	if err != nil {
		if err != cache.ErrCacheMiss {
			h.log.WarnContext(ctx, "cache lookup failed", "error", err)
		}
		return false
	}
	var events []cachedSSEEvent
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		h.log.WarnContext(ctx, "failed to decode cached response", "error", err)
		return false
	}
	for _, ev := range events {
		c.SSEvent(ev.Event, ev.Data)
	}
	c.Writer.Flush()
	return true
}

const rateLimitWindow = time.Minute

// checkRateLimit counts a new-location fetch against the client's per-minute
// budget. Returns false (and writes a 429) if the budget is exhausted.
func (h *WeatherHandler) checkRateLimit(c *gin.Context) bool {
	ctx := c.Request.Context()
	key := "ratelimit:newloc:" + c.ClientIP()
	count, err := h.cache.Incr(ctx, key, rateLimitWindow)
	if err != nil {
		h.log.WarnContext(ctx, "rate limit check failed, allowing request", "error", err)
		return true
	}
	if int(count) > h.cfg.RateLimitNewLocationsPerMinute {
		c.Header("Retry-After", "60")
		response.Error(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many new-location requests, try again in a minute")
		return false
	}
	return true
}
