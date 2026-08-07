// Package weatherapi implements the WeatherProvider interface for weatherapi.com.
package weatherapi

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

const providerName = "weatherapi"

// Provider is the WeatherAPI.com implementation of providers.WeatherProvider.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// New creates a new WeatherAPI provider.
func New(apiKey, baseURL string, log *slog.Logger) providers.WeatherProvider {
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     log.With("provider", providerName),
	}
}

func (p *Provider) Name() string { return providerName }

// currentResponse mirrors the /current.json response shape.
type currentResponse struct {
	Location struct {
		Name    string  `json:"name"`
		Region  string  `json:"region"`
		Country string  `json:"country"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		TzID    string  `json:"tz_id"`
	} `json:"location"`
	Current struct {
		LastUpdatedEpoch int64   `json:"last_updated_epoch"`
		TempC            float64 `json:"temp_c"`
		Condition        struct {
			Text string `json:"text"`
			Code int    `json:"code"`
		} `json:"condition"`
		WindKph    float64 `json:"wind_kph"`
		WindDegree int     `json:"wind_degree"`
		PressureMb float64 `json:"pressure_mb"`
		Humidity   int     `json:"humidity"`
		FeelslikeC float64 `json:"feelslike_c"`
		VisKm      float64 `json:"vis_km"`
		UV         float64 `json:"uv"`
	} `json:"current"`
}

// apiError mirrors WeatherAPI's error envelope: {"code":..,"message":".."}.
type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	reqURL := p.buildCurrentURL(req)

	raw, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("weatherapi: fetch current: %w", err)
	}

	var parsed currentResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherapi: decode response: %w", err)
	}

	return &models.WeatherObservation{
		Location: models.Location{
			City:      parsed.Location.Name,
			Country:   parsed.Location.Country,
			Latitude:  parsed.Location.Lat,
			Longitude: parsed.Location.Lon,
			Timezone:  parsed.Location.TzID,
		},
		Provider:    providerName,
		Temperature: parsed.Current.TempC,
		FeelsLike:   parsed.Current.FeelslikeC,
		Humidity:    parsed.Current.Humidity,
		Pressure:    parsed.Current.PressureMb,
		WindSpeed:   parsed.Current.WindKph / 3.6, // kph -> m/s
		WindDir:     parsed.Current.WindDegree,
		Visibility:  parsed.Current.VisKm,
		UVIndex:     parsed.Current.UV,
		Condition:   mapCondition(parsed.Current.Condition.Text),
		Description: parsed.Current.Condition.Text,
		RawResponse: raw,
		ObservedAt:  time.Unix(parsed.Current.LastUpdatedEpoch, 0).UTC(),
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// buildCurrentURL prefers city name; falls back to "lat,lon" (both accepted by
// WeatherAPI's "q" parameter).
func (p *Provider) buildCurrentURL(req models.WeatherRequest) string {
	params := url.Values{}
	params.Set("key", p.apiKey)
	if req.City != "" {
		params.Set("q", req.City)
	} else {
		params.Set("q", fmt.Sprintf("%f,%f", req.Lat, req.Lon))
	}
	return fmt.Sprintf("%s/current.json?%s", p.baseURL, params.Encode())
}

// get performs a GET request and returns the raw response body. Non-2xx
// responses are decoded as WeatherAPI's {code,message} error envelope when
// possible, falling back to the raw body text.
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
		var apiErr apiError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("status %d: code %d: %s", resp.StatusCode, apiErr.Code, apiErr.Message)
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// mapCondition normalises WeatherAPI's free-text condition into our shared
// WeatherCondition enum. Keyword-based since the numeric condition codes
// aren't documented in full.
func mapCondition(text string) models.WeatherCondition {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "thunder"):
		return models.ConditionThunder
	case strings.Contains(t, "snow"), strings.Contains(t, "sleet"), strings.Contains(t, "ice"), strings.Contains(t, "blizzard"):
		return models.ConditionSnow
	case strings.Contains(t, "rain"), strings.Contains(t, "drizzle"), strings.Contains(t, "shower"):
		return models.ConditionRain
	case strings.Contains(t, "fog"), strings.Contains(t, "mist"), strings.Contains(t, "haze"):
		return models.ConditionFog
	case strings.Contains(t, "overcast"), strings.Contains(t, "cloud"):
		return models.ConditionCloudy
	case strings.Contains(t, "clear"), strings.Contains(t, "sunny"):
		return models.ConditionClear
	default:
		return models.ConditionUnknown
	}
}

func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	// TODO: call GET {baseURL}/forecast.json?key={apiKey}&q={city}&days={n}
	p.log.DebugContext(ctx, "fetch forecast (stub)", "city", req.City)
	days := req.Days
	if days == 0 {
		days = 7
	}
	forecast := &models.ProviderForecast{
		Location:  models.Location{City: req.City},
		Provider:  providerName,
		FetchedAt: time.Now().UTC(),
	}
	for i := 0; i < days; i++ {
		forecast.Days = append(forecast.Days, models.DailyForecast{
			Date:        time.Now().UTC().AddDate(0, 0, i),
			TempMin:     14.0,
			TempMax:     24.0,
			Condition:   models.ConditionCloudy,
			Description: fmt.Sprintf("stub day %d", i+1),
		})
	}
	return forecast, nil
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return p.apiKey != ""
}
