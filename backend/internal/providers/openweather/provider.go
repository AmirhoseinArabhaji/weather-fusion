// Package openweather implements the WeatherProvider interface for OpenWeatherMap's
// free Current Weather Data API (data/2.5/weather).
package openweather

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

const providerName = "openweather"

// Provider is the OpenWeatherMap implementation of providers.WeatherProvider.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// New creates a new OpenWeather provider.
func New(apiKey, baseURL string, log *slog.Logger) providers.WeatherProvider {
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     log.With("provider", providerName),
	}
}

func (p *Provider) Name() string { return providerName }

// weatherResponse mirrors the data/2.5/weather response shape.
type weatherResponse struct {
	Coord struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"coord"`
	Weather []weatherDescriptor `json:"weather"`
	Main    struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Pressure  float64 `json:"pressure"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Visibility float64 `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   int     `json:"deg"`
	} `json:"wind"`
	Dt   int64 `json:"dt"`
	Sys  struct {
		Country string `json:"country"`
	} `json:"sys"`
	Name string `json:"name"`
	Cod  int    `json:"cod"`
}

type weatherDescriptor struct {
	ID          int    `json:"id"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	reqURL := p.buildCurrentURL(req)

	raw, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("openweather: fetch current: %w", err)
	}

	var parsed weatherResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("openweather: decode response: %w", err)
	}

	condition := models.ConditionUnknown
	description := ""
	if len(parsed.Weather) > 0 {
		condition = mapCondition(parsed.Weather[0].Main)
		description = parsed.Weather[0].Description
	}

	return &models.WeatherObservation{
		Location: models.Location{
			City:      parsed.Name,
			Country:   parsed.Sys.Country,
			Latitude:  parsed.Coord.Lat,
			Longitude: parsed.Coord.Lon,
		},
		Provider:    providerName,
		Temperature: parsed.Main.Temp,
		FeelsLike:   parsed.Main.FeelsLike,
		Humidity:    parsed.Main.Humidity,
		Pressure:    parsed.Main.Pressure,
		WindSpeed:   parsed.Wind.Speed,
		WindDir:     parsed.Wind.Deg,
		Visibility:  parsed.Visibility / 1000, // metres -> km
		Condition:   condition,
		Description: description,
		RawResponse: raw,
		ObservedAt:  time.Unix(parsed.Dt, 0).UTC(),
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// buildCurrentURL prefers city name (native to this endpoint); falls back to lat/lon.
func (p *Provider) buildCurrentURL(req models.WeatherRequest) string {
	params := url.Values{}
	params.Set("appid", p.apiKey)
	params.Set("units", "metric")
	if req.City != "" {
		params.Set("q", req.City)
	} else {
		params.Set("lat", fmt.Sprintf("%f", req.Lat))
		params.Set("lon", fmt.Sprintf("%f", req.Lon))
	}
	return fmt.Sprintf("%s/weather?%s", p.baseURL, params.Encode())
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

// mapCondition normalises OpenWeatherMap's "main" weather group into our
// shared WeatherCondition enum.
func mapCondition(main string) models.WeatherCondition {
	switch strings.ToLower(main) {
	case "clear":
		return models.ConditionClear
	case "clouds":
		return models.ConditionCloudy
	case "rain", "drizzle":
		return models.ConditionRain
	case "snow":
		return models.ConditionSnow
	case "thunderstorm":
		return models.ConditionThunder
	case "mist", "fog", "haze", "smoke", "dust", "sand", "ash", "squall", "tornado":
		return models.ConditionFog
	default:
		return models.ConditionUnknown
	}
}

func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	// TODO: call GET {baseURL}/forecast?q={city}&appid={apiKey}&units=metric
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
			TempMin:     15.0,
			TempMax:     25.0,
			Condition:   models.ConditionClear,
			Description: fmt.Sprintf("stub day %d", i+1),
		})
	}
	return forecast, nil
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return p.apiKey != ""
}
