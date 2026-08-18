// Package weatherbit implements the WeatherProvider interface for Weatherbit.
package weatherbit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
)

const providerName = "weatherbit"

// Provider is the Weatherbit implementation of providers.WeatherProvider.
// Uses /v2.0/current, /v2.0/forecast/daily, /v2.0/forecast/hourly — the last
// of which 403s on a free-tier key ("Your API key does not allow access to
// this endpoint"), verified live. FetchHourly still calls it and surfaces
// the error rather than faking a hardcoded skip, in case a paid key is used.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// New creates a new Weatherbit provider.
func New(apiKey, baseURL string, log *slog.Logger) providers.WeatherProvider {
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     log.With("provider", providerName),
	}
}

func (p *Provider) Name() string { return providerName }

type weatherDescriptor struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

// currentResponse mirrors /v2.0/current's response shape.
type currentResponse struct {
	Data []struct {
		Temp        float64           `json:"temp"`
		AppTemp     float64           `json:"app_temp"`
		RH          int               `json:"rh"`
		Pres        float64           `json:"pres"`
		WindSpd     float64           `json:"wind_spd"`
		WindDir     int               `json:"wind_dir"`
		Vis         float64           `json:"vis"`
		UV          float64           `json:"uv"`
		Precip      float64           `json:"precip"`
		Pod         string            `json:"pod"` // "d" or "n"
		Weather     weatherDescriptor `json:"weather"`
		CityName    string            `json:"city_name"`
		CountryCode string            `json:"country_code"`
		Lat         float64           `json:"lat"`
		Lon         float64           `json:"lon"`
		Timezone    string            `json:"timezone"`
		ObTime      string            `json:"ob_time"`
	} `json:"data"`
}

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	raw, err := p.get(ctx, p.buildURL(req, "current", 0))
	if err != nil {
		return nil, fmt.Errorf("weatherbit: fetch current: %w", err)
	}

	var parsed currentResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherbit: decode response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("weatherbit: empty data array")
	}
	d := parsed.Data[0]

	observedAt, err := time.Parse("2006-01-02 15:04", d.ObTime)
	if err != nil {
		observedAt = time.Now().UTC()
	}

	return &models.WeatherObservation{
		Location: models.Location{
			City:      d.CityName,
			Country:   d.CountryCode,
			Latitude:  d.Lat,
			Longitude: d.Lon,
			Timezone:  d.Timezone,
		},
		Provider:    providerName,
		Temperature: d.Temp,
		FeelsLike:   d.AppTemp,
		Humidity:    d.RH,
		Pressure:    d.Pres,
		WindSpeed:   d.WindSpd,
		WindDir:     d.WindDir,
		Visibility:  d.Vis,
		UVIndex:     d.UV,
		PrecipProb:  0, // current endpoint reports precip amount, not probability
		Condition:   mapWeatherCode(d.Weather.Code),
		IsDay:       podIsDay(d.Pod),
		Description: d.Weather.Description,
		RawResponse: raw,
		ObservedAt:  observedAt,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// dailyResponse mirrors /v2.0/forecast/daily's response shape.
type dailyResponse struct {
	CityName string  `json:"city_name"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Timezone string  `json:"timezone"`
	Data     []struct {
		ValidDate string            `json:"valid_date"`
		MaxTemp   float64           `json:"max_temp"`
		MinTemp   float64           `json:"min_temp"`
		RH        int               `json:"rh"`
		WindSpd   float64           `json:"wind_spd"`
		Pop       float64           `json:"pop"`
		Weather   weatherDescriptor `json:"weather"`
	} `json:"data"`
}

func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	days := req.Days
	if days <= 0 {
		days = 7
	}
	raw, err := p.get(ctx, p.buildURL(req, "forecast/daily", days))
	if err != nil {
		return nil, fmt.Errorf("weatherbit: fetch forecast: %w", err)
	}

	var parsed dailyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherbit: decode forecast response: %w", err)
	}

	forecast := &models.ProviderForecast{
		Location: models.Location{
			City:     parsed.CityName,
			Timezone: parsed.Timezone,
		},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, entry := range parsed.Data {
		date, err := time.Parse("2006-01-02", entry.ValidDate)
		if err != nil {
			continue
		}
		forecast.Days = append(forecast.Days, models.DailyForecast{
			Date:        date,
			TempMin:     entry.MinTemp,
			TempMax:     entry.MaxTemp,
			Humidity:    entry.RH,
			WindSpeed:   entry.WindSpd,
			PrecipProb:  entry.Pop / 100,
			Condition:   mapWeatherCode(entry.Weather.Code),
			Description: entry.Weather.Description,
		})
	}
	return forecast, nil
}

// hourlyResponse mirrors /v2.0/forecast/hourly's response shape.
type hourlyResponse struct {
	CityName string `json:"city_name"`
	Data     []struct {
		TimestampUTC string            `json:"timestamp_utc"`
		Temp         float64           `json:"temp"`
		Pop          float64           `json:"pop"`
		Weather      weatherDescriptor `json:"weather"`
	} `json:"data"`
}

func (p *Provider) FetchHourly(ctx context.Context, req models.WeatherRequest) (*models.ProviderHourlyForecast, error) {
	raw, err := p.get(ctx, p.buildURL(req, "forecast/hourly", 48))
	if err != nil {
		return nil, fmt.Errorf("weatherbit: fetch hourly: %w", err)
	}

	var parsed hourlyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherbit: decode hourly response: %w", err)
	}

	hourly := &models.ProviderHourlyForecast{
		Location:    models.Location{City: parsed.CityName},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, entry := range parsed.Data {
		t, err := time.Parse("2006-01-02T15:04:05", entry.TimestampUTC)
		if err != nil {
			continue
		}
		hourly.Hours = append(hourly.Hours, models.HourlyForecast{
			Time:        t.UTC(),
			Temperature: entry.Temp,
			PrecipProb:  entry.Pop / 100,
			Condition:   mapWeatherCode(entry.Weather.Code),
			Description: entry.Weather.Description,
		})
	}
	return hourly, nil
}

// buildURL prefers city text; falls back to lat/lon. days is only sent for
// the daily endpoint (0 = omit); hours is fixed to 48 for the hourly endpoint
// via the caller.
func (p *Provider) buildURL(req models.WeatherRequest, endpoint string, count int) string {
	params := url.Values{}
	params.Set("key", p.apiKey)
	if req.City != "" {
		params.Set("city", req.City)
	} else {
		params.Set("lat", strconv.FormatFloat(req.Lat, 'f', -1, 64))
		params.Set("lon", strconv.FormatFloat(req.Lon, 'f', -1, 64))
	}

	units := "M"
	if req.Units == "imperial" {
		units = "I"
	}
	params.Set("units", units)

	switch {
	case strings.HasSuffix(endpoint, "daily") && count > 0:
		params.Set("days", strconv.Itoa(count))
	case strings.HasSuffix(endpoint, "hourly") && count > 0:
		params.Set("hours", strconv.Itoa(count))
	}

	return fmt.Sprintf("%s/%s?%s", p.baseURL, endpoint, params.Encode())
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

// mapWeatherCode normalises Weatherbit's numeric weather code into our
// shared WeatherCondition enum.
func mapWeatherCode(code int) models.WeatherCondition {
	switch {
	case code >= 200 && code <= 233:
		return models.ConditionThunder
	case code >= 300 && code <= 302:
		return models.ConditionRain
	case code >= 500 && code <= 522:
		return models.ConditionRain
	case code >= 610 && code <= 613:
		return models.ConditionSleet // rain/snow mix, sleet, heavy sleet, sleet showers
	case code >= 600 && code <= 623:
		return models.ConditionSnow
	case code == 741 || code == 751:
		return models.ConditionFog
	case code >= 700 && code <= 731:
		return models.ConditionFog // mist/smoke/haze/sand — no dedicated enum value, closest fit
	case code == 800:
		return models.ConditionClear
	case code >= 801 && code <= 804:
		return models.ConditionCloudy
	default:
		return models.ConditionUnknown
	}
}

func podIsDay(pod string) *bool {
	switch pod {
	case "d":
		return boolPtr(true)
	case "n":
		return boolPtr(false)
	default:
		return nil
	}
}

func boolPtr(b bool) *bool { return &b }

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return p.apiKey != ""
}
