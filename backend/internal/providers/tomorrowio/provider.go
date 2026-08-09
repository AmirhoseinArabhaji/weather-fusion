// Package tomorrowio implements the WeatherProvider interface for tomorrow.io.
package tomorrowio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
)

const providerName = "tomorrow.io"

// Provider is the tomorrow.io implementation of providers.WeatherProvider.
// Uses the purpose-built /weather/realtime and /weather/forecast endpoints
// rather than the general /timelines endpoint — a much closer fit for our
// single-point "current" and per-day "forecast" model, no minutely/hourly
// array to reduce ourselves.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// New creates a new tomorrow.io provider.
func New(apiKey, baseURL string, log *slog.Logger) providers.WeatherProvider {
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     log.With("provider", providerName),
	}
}

func (p *Provider) Name() string { return providerName }

// realtimeResponse mirrors /v4/weather/realtime's response shape.
type realtimeResponse struct {
	Data struct {
		Time   string         `json:"time"`
		Values realtimeValues `json:"values"`
	} `json:"data"`
	Location struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"location"`
}

type realtimeValues struct {
	Temperature              float64 `json:"temperature"`
	TemperatureApparent      float64 `json:"temperatureApparent"`
	Humidity                 float64 `json:"humidity"`
	PressureSeaLevel         float64 `json:"pressureSeaLevel"`
	WindSpeed                float64 `json:"windSpeed"`
	WindDirection            float64 `json:"windDirection"`
	Visibility               float64 `json:"visibility"`
	PrecipitationProbability float64 `json:"precipitationProbability"`
	WeatherCode              int     `json:"weatherCode"`
}

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	raw, err := p.get(ctx, p.buildURL(req, "realtime", ""))
	if err != nil {
		return nil, fmt.Errorf("tomorrowio: fetch current: %w", err)
	}

	var parsed realtimeResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("tomorrowio: decode response: %w", err)
	}

	observedAt, err := time.Parse(time.RFC3339, parsed.Data.Time)
	if err != nil {
		observedAt = time.Now().UTC()
	}

	v := parsed.Data.Values
	return &models.WeatherObservation{
		Location: models.Location{
			// tomorrow.io doesn't reverse-geocode lat/lon into a place name —
			// only the request's own city text (if any) is trustworthy here.
			City:      req.City,
			Latitude:  parsed.Location.Lat,
			Longitude: parsed.Location.Lon,
		},
		Provider:    providerName,
		Temperature: v.Temperature,
		FeelsLike:   v.TemperatureApparent,
		Humidity:    int(v.Humidity),
		Pressure:    v.PressureSeaLevel,
		WindSpeed:   v.WindSpeed,
		WindDir:     int(v.WindDirection),
		Visibility:  v.Visibility,
		PrecipProb:  v.PrecipitationProbability / 100,
		Condition:   mapWeatherCode(v.WeatherCode),
		Description: describeWeatherCode(v.WeatherCode),
		RawResponse: raw,
		ObservedAt:  observedAt,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// forecastResponse mirrors /v4/weather/forecast's response shape with timesteps=1d.
type forecastResponse struct {
	Timelines struct {
		Daily []dailyEntry `json:"daily"`
	} `json:"timelines"`
}

type dailyEntry struct {
	Time   string `json:"time"`
	Values struct {
		TemperatureMin              float64 `json:"temperatureMin"`
		TemperatureMax              float64 `json:"temperatureMax"`
		HumidityAvg                 float64 `json:"humidityAvg"`
		WindSpeedAvg                float64 `json:"windSpeedAvg"`
		PrecipitationProbabilityAvg float64 `json:"precipitationProbabilityAvg"`
		WeatherCodeMax              int     `json:"weatherCodeMax"`
	} `json:"values"`
}

// FetchForecast retrieves the daily forecast and trims it to req.Days if requested.
func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	raw, err := p.get(ctx, p.buildURL(req, "forecast", "1d"))
	if err != nil {
		return nil, fmt.Errorf("tomorrowio: fetch forecast: %w", err)
	}

	var parsed forecastResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("tomorrowio: decode forecast response: %w", err)
	}

	entries := parsed.Timelines.Daily
	if req.Days > 0 && req.Days < len(entries) {
		entries = entries[:req.Days]
	}

	forecast := &models.ProviderForecast{
		Location:    models.Location{City: req.City},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, d := range entries {
		date, err := time.Parse(time.RFC3339, d.Time)
		if err != nil {
			continue
		}
		forecast.Days = append(forecast.Days, models.DailyForecast{
			Date:        date,
			TempMin:     d.Values.TemperatureMin,
			TempMax:     d.Values.TemperatureMax,
			Humidity:    int(d.Values.HumidityAvg),
			WindSpeed:   d.Values.WindSpeedAvg,
			PrecipProb:  d.Values.PrecipitationProbabilityAvg / 100,
			Condition:   mapWeatherCode(d.Values.WeatherCodeMax),
			Description: describeWeatherCode(d.Values.WeatherCodeMax),
		})
	}
	return forecast, nil
}

// buildURL prefers city text; falls back to "lat,lon" — tomorrow.io's
// "location" parameter accepts either natively.
func (p *Provider) buildURL(req models.WeatherRequest, endpoint, timesteps string) string {
	location := req.City
	if location == "" {
		location = fmt.Sprintf("%f,%f", req.Lat, req.Lon)
	}

	units := "metric"
	if req.Units == "imperial" {
		units = "imperial"
	}

	params := url.Values{}
	params.Set("apikey", p.apiKey)
	params.Set("location", location)
	params.Set("units", units)
	if timesteps != "" {
		params.Set("timesteps", timesteps)
	}
	// p.baseURL is expected to be ".../v4" — no trailing endpoint segment.
	return fmt.Sprintf("%s/weather/%s?%s", p.baseURL, endpoint, params.Encode())
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

// mapWeatherCode normalises tomorrow.io's numeric weather code into our
// shared WeatherCondition enum.
func mapWeatherCode(code int) models.WeatherCondition {
	switch {
	case code == 1000 || code == 1100:
		return models.ConditionClear
	case code == 1001 || code == 1101 || code == 1102:
		return models.ConditionCloudy
	case code == 2000 || code == 2100:
		return models.ConditionFog
	case code == 4000 || code == 4001 || code == 4200 || code == 4201:
		return models.ConditionRain
	case code == 5000 || code == 5001 || code == 5100 || code == 5101:
		return models.ConditionSnow
	case code == 6000 || code == 6001 || code == 6200 || code == 6201:
		return models.ConditionSnow // freezing rain/drizzle — icy, closer to snow than plain rain
	case code == 7000 || code == 7101 || code == 7102:
		return models.ConditionSnow // ice pellets
	case code == 8000:
		return models.ConditionThunder
	default:
		return models.ConditionUnknown
	}
}

// describeWeatherCode returns a short human-readable label for a weather code.
func describeWeatherCode(code int) string {
	switch code {
	case 1000:
		return "clear"
	case 1100:
		return "mostly clear"
	case 1101:
		return "partly cloudy"
	case 1102:
		return "mostly cloudy"
	case 1001:
		return "cloudy"
	case 2000:
		return "fog"
	case 2100:
		return "light fog"
	case 4000:
		return "drizzle"
	case 4001:
		return "rain"
	case 4200:
		return "light rain"
	case 4201:
		return "heavy rain"
	case 5000:
		return "snow"
	case 5001:
		return "flurries"
	case 5100:
		return "light snow"
	case 5101:
		return "heavy snow"
	case 6000:
		return "freezing drizzle"
	case 6001:
		return "freezing rain"
	case 6200:
		return "light freezing rain"
	case 6201:
		return "heavy freezing rain"
	case 7000:
		return "ice pellets"
	case 7101:
		return "heavy ice pellets"
	case 7102:
		return "light ice pellets"
	case 8000:
		return "thunderstorm"
	default:
		return "unknown"
	}
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return p.apiKey != ""
}
