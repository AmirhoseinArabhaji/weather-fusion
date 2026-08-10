// Package openmeteo implements the WeatherProvider interface for Open-Meteo,
// a free weather API that requires no API key.
package openmeteo

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

const providerName = "open-meteo"

// geocodeURL resolves a city name to coordinates. Open-Meteo's forecast
// endpoint only accepts lat/lon, so city-name requests are geocoded first
// via this separate (also free, no-key) endpoint.
const geocodeURL = "https://geocoding-api.open-meteo.com/v1/search"

// Provider is the Open-Meteo implementation of providers.WeatherProvider.
type Provider struct {
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// New creates a new Open-Meteo provider. apiKey is accepted for constructor
// symmetry with the other providers but unused — Open-Meteo needs no key.
func New(apiKey, baseURL string, log *slog.Logger) providers.WeatherProvider {
	return &Provider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     log.With("provider", providerName),
	}
}

func (p *Provider) Name() string { return providerName }

// currentResponse mirrors /v1/forecast's "current" response shape.
type currentResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Current   struct {
		Time                string  `json:"time"`
		Temperature2m       float64 `json:"temperature_2m"`
		RelativeHumidity2m  int     `json:"relative_humidity_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		WeatherCode         int     `json:"weather_code"`
		WindSpeed10m        float64 `json:"wind_speed_10m"`
		WindDirection10m    int     `json:"wind_direction_10m"`
		PressureMsl         float64 `json:"pressure_msl"`
		CloudCover          int     `json:"cloud_cover"`
		Visibility          float64 `json:"visibility"`
	} `json:"current"`
}

// geocodeResponse mirrors the Geocoding API's response shape.
type geocodeResponse struct {
	Results []geocodeResult `json:"results"`
}

type geocodeResult struct {
	Name    string  `json:"name"`
	Lat     float64 `json:"latitude"`
	Lon     float64 `json:"longitude"`
	Country string  `json:"country"`
}

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	lat, lon, city, country, err := p.resolveLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo: %w", err)
	}

	raw, err := p.get(ctx, p.buildCurrentURL(lat, lon, req.Units))
	if err != nil {
		return nil, fmt.Errorf("open-meteo: fetch current: %w", err)
	}

	var parsed currentResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("open-meteo: decode response: %w", err)
	}

	observedAt, err := time.ParseInLocation("2006-01-02T15:04", parsed.Current.Time, time.UTC)
	if err != nil {
		observedAt = time.Now().UTC()
	}

	return &models.WeatherObservation{
		Location: models.Location{
			City:      city,
			Country:   country,
			Latitude:  parsed.Latitude,
			Longitude: parsed.Longitude,
			Timezone:  parsed.Timezone,
		},
		Provider:    providerName,
		Temperature: parsed.Current.Temperature2m,
		FeelsLike:   parsed.Current.ApparentTemperature,
		Humidity:    parsed.Current.RelativeHumidity2m,
		Pressure:    parsed.Current.PressureMsl,
		WindSpeed:   parsed.Current.WindSpeed10m,
		WindDir:     parsed.Current.WindDirection10m,
		Visibility:  parsed.Current.Visibility / 1000, // metres -> km
		Condition:   mapWMOCode(parsed.Current.WeatherCode),
		Description: describeWMOCode(parsed.Current.WeatherCode),
		RawResponse: raw,
		ObservedAt:  observedAt,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// resolveLocation returns coordinates for the request, geocoding the city
// name first if lat/lon weren't provided directly.
func (p *Provider) resolveLocation(ctx context.Context, req models.WeatherRequest) (lat, lon float64, city, country string, err error) {
	if req.Lat != 0 || req.Lon != 0 {
		return req.Lat, req.Lon, req.City, "", nil
	}
	if req.City == "" {
		return 0, 0, "", "", fmt.Errorf("city or lat/lon required")
	}

	params := url.Values{}
	params.Set("name", req.City)
	params.Set("count", "1")
	params.Set("language", "en")
	params.Set("format", "json")
	geoReqURL := fmt.Sprintf("%s?%s", geocodeURL, params.Encode())

	body, err := p.get(ctx, geoReqURL)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("geocode %q: %w", req.City, err)
	}

	var parsed geocodeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0, "", "", fmt.Errorf("decode geocode response: %w", err)
	}
	if len(parsed.Results) == 0 {
		return 0, 0, "", "", fmt.Errorf("city not found: %s", req.City)
	}

	r := parsed.Results[0]
	return r.Lat, r.Lon, r.Name, r.Country, nil
}

// buildCurrentURL requests exactly the fields our model maps.
func (p *Provider) buildCurrentURL(lat, lon float64, units string) string {
	tempUnit, windUnit := "celsius", "ms"
	if units == "imperial" {
		tempUnit, windUnit = "fahrenheit", "mph"
	}

	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", lat))
	params.Set("longitude", fmt.Sprintf("%f", lon))
	params.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,"+
		"wind_speed_10m,wind_direction_10m,pressure_msl,cloud_cover,visibility")
	params.Set("temperature_unit", tempUnit)
	params.Set("wind_speed_unit", windUnit)
	params.Set("timezone", "auto")
	return fmt.Sprintf("%s/forecast?%s", p.baseURL, params.Encode())
}

// dailyResponse mirrors /v1/forecast's "daily" response shape — columnar
// (one array per field, aligned by index), not an array of per-day objects.
type dailyResponse struct {
	Daily struct {
		Time                        []string  `json:"time"`
		WeatherCode                 []int     `json:"weather_code"`
		Temperature2mMax            []float64 `json:"temperature_2m_max"`
		Temperature2mMin            []float64 `json:"temperature_2m_min"`
		PrecipitationProbabilityMax []int     `json:"precipitation_probability_max"`
		WindSpeed10mMax             []float64 `json:"wind_speed_10m_max"`
	} `json:"daily"`
}

func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	lat, lon, city, _, err := p.resolveLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo: %w", err)
	}

	days := req.Days
	if days == 0 {
		days = 7
	}
	if days > 16 {
		days = 16
	}

	raw, err := p.get(ctx, p.buildDailyURL(lat, lon, req.Units, days))
	if err != nil {
		return nil, fmt.Errorf("open-meteo: fetch forecast: %w", err)
	}

	var parsed dailyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("open-meteo: decode forecast response: %w", err)
	}

	forecast := &models.ProviderForecast{
		Location:    models.Location{City: city, Latitude: lat, Longitude: lon},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for i, dateStr := range parsed.Daily.Time {
		date, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
		if err != nil {
			continue
		}
		forecast.Days = append(forecast.Days, models.DailyForecast{
			Date:        date,
			TempMin:     parsed.Daily.Temperature2mMin[i],
			TempMax:     parsed.Daily.Temperature2mMax[i],
			WindSpeed:   parsed.Daily.WindSpeed10mMax[i],
			PrecipProb:  float64(parsed.Daily.PrecipitationProbabilityMax[i]) / 100,
			Condition:   mapWMOCode(parsed.Daily.WeatherCode[i]),
			Description: describeWMOCode(parsed.Daily.WeatherCode[i]),
		})
	}
	return forecast, nil
}

// buildDailyURL requests exactly the fields our DailyForecast model uses.
func (p *Provider) buildDailyURL(lat, lon float64, units string, days int) string {
	tempUnit, windUnit := "celsius", "ms"
	if units == "imperial" {
		tempUnit, windUnit = "fahrenheit", "mph"
	}

	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", lat))
	params.Set("longitude", fmt.Sprintf("%f", lon))
	params.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,"+
		"precipitation_probability_max,wind_speed_10m_max")
	params.Set("temperature_unit", tempUnit)
	params.Set("wind_speed_unit", windUnit)
	params.Set("timezone", "auto")
	params.Set("forecast_days", fmt.Sprintf("%d", days))
	return fmt.Sprintf("%s/forecast?%s", p.baseURL, params.Encode())
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

// mapWMOCode normalises Open-Meteo's WMO weather code into our shared
// WeatherCondition enum. See https://open-meteo.com/en/docs for the table.
func mapWMOCode(code int) models.WeatherCondition {
	switch {
	case code == 0:
		return models.ConditionClear
	case code >= 1 && code <= 3:
		return models.ConditionCloudy
	case code == 45 || code == 48:
		return models.ConditionFog
	case code >= 51 && code <= 67:
		return models.ConditionRain
	case code >= 80 && code <= 82:
		return models.ConditionRain
	case (code >= 71 && code <= 77) || code == 85 || code == 86:
		return models.ConditionSnow
	case code >= 95 && code <= 99:
		return models.ConditionThunder
	default:
		return models.ConditionUnknown
	}
}

// describeWMOCode returns a short human-readable label for a WMO weather code.
func describeWMOCode(code int) string {
	switch code {
	case 0:
		return "clear sky"
	case 1:
		return "mainly clear"
	case 2:
		return "partly cloudy"
	case 3:
		return "overcast"
	case 45:
		return "fog"
	case 48:
		return "depositing rime fog"
	case 51:
		return "light drizzle"
	case 53:
		return "moderate drizzle"
	case 55:
		return "dense drizzle"
	case 56:
		return "light freezing drizzle"
	case 57:
		return "dense freezing drizzle"
	case 61:
		return "slight rain"
	case 63:
		return "moderate rain"
	case 65:
		return "heavy rain"
	case 66:
		return "light freezing rain"
	case 67:
		return "heavy freezing rain"
	case 71:
		return "slight snow fall"
	case 73:
		return "moderate snow fall"
	case 75:
		return "heavy snow fall"
	case 77:
		return "snow grains"
	case 80:
		return "slight rain showers"
	case 81:
		return "moderate rain showers"
	case 82:
		return "violent rain showers"
	case 85:
		return "slight snow showers"
	case 86:
		return "heavy snow showers"
	case 95:
		return "thunderstorm"
	case 96:
		return "thunderstorm with slight hail"
	case 99:
		return "thunderstorm with heavy hail"
	default:
		return "unknown"
	}
}

// hourlyResponse mirrors /v1/forecast's "hourly" response shape — columnar,
// like "daily".
type hourlyResponse struct {
	Hourly struct {
		Time                     []string  `json:"time"`
		Temperature2m            []float64 `json:"temperature_2m"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
		WeatherCode              []int     `json:"weather_code"`
	} `json:"hourly"`
}

// FetchHourly retrieves the next 48 hours at hourly resolution.
func (p *Provider) FetchHourly(ctx context.Context, req models.WeatherRequest) (*models.ProviderHourlyForecast, error) {
	lat, lon, city, _, err := p.resolveLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo: %w", err)
	}

	raw, err := p.get(ctx, p.buildHourlyURL(lat, lon, req.Units))
	if err != nil {
		return nil, fmt.Errorf("open-meteo: fetch hourly: %w", err)
	}

	var parsed hourlyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("open-meteo: decode hourly response: %w", err)
	}

	hourly := &models.ProviderHourlyForecast{
		Location:    models.Location{City: city, Latitude: lat, Longitude: lon},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for i, t := range parsed.Hourly.Time {
		hourTime, err := time.ParseInLocation("2006-01-02T15:04", t, time.UTC)
		if err != nil {
			continue
		}
		hourly.Hours = append(hourly.Hours, models.HourlyForecast{
			Time:        hourTime,
			Temperature: parsed.Hourly.Temperature2m[i],
			PrecipProb:  float64(parsed.Hourly.PrecipitationProbability[i]) / 100,
			Condition:   mapWMOCode(parsed.Hourly.WeatherCode[i]),
			Description: describeWMOCode(parsed.Hourly.WeatherCode[i]),
		})
	}
	return hourly, nil
}

// buildHourlyURL requests exactly the fields our HourlyForecast model uses,
// capped to the next 48 hours.
func (p *Provider) buildHourlyURL(lat, lon float64, units string) string {
	tempUnit := "celsius"
	if units == "imperial" {
		tempUnit = "fahrenheit"
	}

	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", lat))
	params.Set("longitude", fmt.Sprintf("%f", lon))
	params.Set("hourly", "temperature_2m,precipitation_probability,weather_code")
	params.Set("temperature_unit", tempUnit)
	params.Set("forecast_hours", "48")
	params.Set("timezone", "auto")
	return fmt.Sprintf("%s/forecast?%s", p.baseURL, params.Encode())
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return true
}
