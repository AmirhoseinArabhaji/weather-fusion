// Package visualcrossing implements the WeatherProvider interface for
// Visual Crossing's Timeline Weather API.
package visualcrossing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
)

const providerName = "visualcrossing"

// Provider is the Visual Crossing implementation of providers.WeatherProvider.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// New creates a new Visual Crossing provider.
func New(apiKey, baseURL string, log *slog.Logger) providers.WeatherProvider {
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     log.With("provider", providerName),
	}
}

func (p *Provider) Name() string { return providerName }

// currentResponse mirrors the Timeline API's response shape with include=current.
type currentResponse struct {
	Latitude          float64     `json:"latitude"`
	Longitude         float64     `json:"longitude"`
	ResolvedAddress   string      `json:"resolvedAddress"`
	Timezone          string      `json:"timezone"`
	CurrentConditions *conditions `json:"currentConditions"`
}

type conditions struct {
	DatetimeEpoch int64   `json:"datetimeEpoch"`
	Temp          float64 `json:"temp"`
	FeelsLike     float64 `json:"feelslike"`
	Humidity      float64 `json:"humidity"`
	Pressure      float64 `json:"pressure"`
	WindSpeed     float64 `json:"windspeed"` // km/h under unitGroup=metric
	WindDir       float64 `json:"winddir"`
	Visibility    float64 `json:"visibility"`
	PrecipProb    float64 `json:"precipprob"` // forecast-only per docs; expect 0 for current
	Conditions    string  `json:"conditions"`
}

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	raw, err := p.get(ctx, p.buildURL(req, "current"))
	if err != nil {
		return nil, fmt.Errorf("visualcrossing: fetch current: %w", err)
	}

	var parsed currentResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("visualcrossing: decode response: %w", err)
	}
	if parsed.CurrentConditions == nil {
		return nil, fmt.Errorf("visualcrossing: response had no currentConditions")
	}
	cc := parsed.CurrentConditions

	return &models.WeatherObservation{
		Location: models.Location{
			City:      resolveCityName(req.City, parsed.ResolvedAddress),
			Latitude:  parsed.Latitude,
			Longitude: parsed.Longitude,
			Timezone:  parsed.Timezone,
		},
		Provider:    providerName,
		Temperature: cc.Temp,
		FeelsLike:   cc.FeelsLike,
		Humidity:    int(cc.Humidity),
		Pressure:    cc.Pressure,
		WindSpeed:   cc.WindSpeed / 3.6, // km/h -> m/s
		WindDir:     int(cc.WindDir),
		Visibility:  cc.Visibility,
		PrecipProb:  cc.PrecipProb / 100,
		Condition:   mapCondition(cc.Conditions),
		Description: cc.Conditions,
		RawResponse: raw,
		ObservedAt:  time.Unix(cc.DatetimeEpoch, 0).UTC(),
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// dailyResponse mirrors the Timeline API's response shape with include=days.
type dailyResponse struct {
	ResolvedAddress string       `json:"resolvedAddress"`
	Days            []dailyEntry `json:"days"`
}

type dailyEntry struct {
	DatetimeEpoch int64   `json:"datetimeEpoch"`
	TempMax       float64 `json:"tempmax"`
	TempMin       float64 `json:"tempmin"`
	Humidity      float64 `json:"humidity"`
	WindSpeed     float64 `json:"windspeed"` // km/h under unitGroup=metric
	PrecipProb    float64 `json:"precipprob"`
	Conditions    string  `json:"conditions"`
}

// FetchForecast retrieves the default forecast window (next 15 days) and
// trims it to req.Days if requested.
func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	raw, err := p.get(ctx, p.buildURL(req, "days"))
	if err != nil {
		return nil, fmt.Errorf("visualcrossing: fetch forecast: %w", err)
	}

	var parsed dailyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("visualcrossing: decode forecast response: %w", err)
	}

	entries := parsed.Days
	if req.Days > 0 && req.Days < len(entries) {
		entries = entries[:req.Days]
	}

	forecast := &models.ProviderForecast{
		Location:    models.Location{City: resolveCityName(req.City, parsed.ResolvedAddress)},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, d := range entries {
		forecast.Days = append(forecast.Days, models.DailyForecast{
			Date:        time.Unix(d.DatetimeEpoch, 0).UTC(),
			TempMin:     d.TempMin,
			TempMax:     d.TempMax,
			Humidity:    int(d.Humidity),
			WindSpeed:   d.WindSpeed / 3.6,
			PrecipProb:  d.PrecipProb / 100,
			Condition:   mapCondition(d.Conditions),
			Description: d.Conditions,
		})
	}
	return forecast, nil
}

// buildURL prefers city/address text; falls back to "lat,lon" — Visual
// Crossing's location path segment accepts either natively, no geocoding step needed.
func (p *Provider) buildURL(req models.WeatherRequest, include string) string {
	location := req.City
	if location == "" {
		location = fmt.Sprintf("%f,%f", req.Lat, req.Lon)
	}

	unitGroup := "metric"
	if req.Units == "imperial" {
		unitGroup = "us"
	}

	params := url.Values{}
	params.Set("key", p.apiKey)
	params.Set("unitGroup", unitGroup)
	params.Set("include", include)
	params.Set("contentType", "json")
	// p.baseURL already ends in "/timeline" (see VISUALCROSSING_BASE_URL) — don't
	// append it again here, or Visual Crossing's router mis-parses the path.
	return fmt.Sprintf("%s/%s?%s", p.baseURL, url.PathEscape(location), params.Encode())
}

// resolveCityName prefers the request's own city text (already known-good);
// otherwise falls back to Visual Crossing's resolvedAddress — except when
// queried by lat/lon, resolvedAddress just echoes the coordinates back
// rather than reverse-geocoding them, so that case is left empty rather
// than reporting a latitude as a city name.
func resolveCityName(reqCity, resolvedAddress string) string {
	if reqCity != "" {
		return reqCity
	}
	if looksLikeCoordinates(resolvedAddress) {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(resolvedAddress, ",", 2)[0])
}

// looksLikeCoordinates reports whether s contains nothing but digits and
// coordinate punctuation — real place names always contain letters.
func looksLikeCoordinates(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r == '.' || r == ',' || r == '-' || r == ' ') {
			return false
		}
	}
	return true
}

// get performs a GET request and returns the raw response body, treating any
// non-2xx status as an error.
func (p *Provider) get(ctx context.Context, reqURL string) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// mapCondition normalises Visual Crossing's free-text "conditions" field
// (e.g. "Rain, Partially cloudy") into our shared WeatherCondition enum.
func mapCondition(text string) models.WeatherCondition {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "thunder"):
		return models.ConditionThunder
	case strings.Contains(t, "snow"), strings.Contains(t, "ice"):
		return models.ConditionSnow
	case strings.Contains(t, "rain"), strings.Contains(t, "drizzle"):
		return models.ConditionRain
	case strings.Contains(t, "fog"), strings.Contains(t, "mist"), strings.Contains(t, "haze"):
		return models.ConditionFog
	case strings.Contains(t, "overcast"), strings.Contains(t, "cloud"):
		return models.ConditionCloudy
	case strings.Contains(t, "clear"):
		return models.ConditionClear
	default:
		return models.ConditionUnknown
	}
}

// hourlyResponse mirrors the Timeline API's response shape with
// include=hours: hourly readings nested inside each day.
type hourlyResponse struct {
	ResolvedAddress string `json:"resolvedAddress"`
	Days            []struct {
		Hours []hourEntry `json:"hours"`
	} `json:"days"`
}

type hourEntry struct {
	DatetimeEpoch int64   `json:"datetimeEpoch"`
	Temp          float64 `json:"temp"`
	PrecipProb    float64 `json:"precipprob"`
	Conditions    string  `json:"conditions"`
}

// FetchHourly retrieves hourly data for the default forecast window (Visual
// Crossing nests hours inside each day; this flattens them into one list).
func (p *Provider) FetchHourly(ctx context.Context, req models.WeatherRequest) (*models.ProviderHourlyForecast, error) {
	raw, err := p.get(ctx, p.buildURL(req, "hours"))
	if err != nil {
		return nil, fmt.Errorf("visualcrossing: fetch hourly: %w", err)
	}

	var parsed hourlyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("visualcrossing: decode hourly response: %w", err)
	}

	hourly := &models.ProviderHourlyForecast{
		Location:    models.Location{City: resolveCityName(req.City, parsed.ResolvedAddress)},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, d := range parsed.Days {
		for _, h := range d.Hours {
			hourly.Hours = append(hourly.Hours, models.HourlyForecast{
				Time:        time.Unix(h.DatetimeEpoch, 0).UTC(),
				Temperature: h.Temp,
				PrecipProb:  h.PrecipProb / 100,
				Condition:   mapCondition(h.Conditions),
				Description: h.Conditions,
			})
		}
	}
	return hourly, nil
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return p.apiKey != ""
}
