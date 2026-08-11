package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/amirhosein/weather-fusion/internal/geocoding"
	"github.com/amirhosein/weather-fusion/pkg/response"
)

// LocationsHandler serves place-name search for the frontend's autocomplete.
type LocationsHandler struct {
	geocoder geocoding.LocationSearch
	log      *slog.Logger
}

// NewLocationsHandler creates a LocationsHandler.
func NewLocationsHandler(geocoder geocoding.LocationSearch, log *slog.Logger) *LocationsHandler {
	return &LocationsHandler{geocoder: geocoder, log: log.With("handler", "locations")}
}

// Search godoc
//
//	@Summary     Search place names
//	@Description Resolves a free-text query into candidate place matches with coordinates
//	@Tags        locations
//	@Produce     json
//	@Param       q query string true "Place name query"
//	@Router      /locations/search [get]
func (h *LocationsHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 2 {
		response.OK(c, []geocoding.LocationMatch{})
		return
	}

	limit := 5
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 10 {
			limit = n
		}
	}

	matches, err := h.geocoder.Search(c.Request.Context(), q, limit)
	if err != nil {
		h.log.WarnContext(c.Request.Context(), "location search failed", "query", q, "error", err)
		response.Error(c, http.StatusBadGateway, "GEOCODER_ERROR", "location search unavailable")
		return
	}

	response.OK(c, matches)
}
