// Package metno implements the WeatherProvider interface for MET Norway's
// Locationforecast API, a free weather API that requires no API key.
package metno

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
	"github.com/amirhosein/weather-fusion/internal/providers/geocode"
)

const providerName = "met.no"

// Provider is the MET Norway implementation of providers.WeatherProvider.
// Uses the "compact" Locationforecast variant (no separate daily/hourly
// endpoints — one timeseries covers both, at 1h resolution for the first
// ~2.5 days then 6h resolution out to ~9 days).
type Provider struct {
	baseURL   string
	userAgent string
	client    *http.Client
	log       *slog.Logger
}

// New creates a new MET Norway provider. apiKey is accepted for constructor
// symmetry with the other providers but unused — MET Norway needs no key,
// only an identifying User-Agent (required by their terms of service, or
// requests get a 403).
func New(apiKey, baseURL, userAgent string, log *slog.Logger) providers.WeatherProvider {
	return &Provider{
		baseURL:   baseURL,
		userAgent: userAgent,
		client:    providers.NewHTTPClient(),
		log:       log.With("provider", providerName),
	}
}

func (p *Provider) Name() string { return providerName }

type forecastResponse struct {
	Properties struct {
		Timeseries []timeseriesEntry `json:"timeseries"`
	} `json:"properties"`
}

type timeseriesEntry struct {
	Time string `json:"time"`
	Data struct {
		Instant struct {
			Details instantDetails `json:"details"`
		} `json:"instant"`
		Next1Hours *periodForecast `json:"next_1_hours"`
		Next6Hours *periodForecast `json:"next_6_hours"`
	} `json:"data"`
}

type instantDetails struct {
	AirTemperature        float64 `json:"air_temperature"`
	RelativeHumidity      float64 `json:"relative_humidity"`
	WindSpeed             float64 `json:"wind_speed"`
	WindFromDirection     float64 `json:"wind_from_direction"`
	AirPressureAtSeaLevel float64 `json:"air_pressure_at_sea_level"`
	CloudAreaFraction     float64 `json:"cloud_area_fraction"`
}

type periodForecast struct {
	Summary struct {
		SymbolCode string `json:"symbol_code"`
	} `json:"summary"`
	Details struct {
		PrecipitationAmount float64 `json:"precipitation_amount"`
	} `json:"details"`
}

// symbolCode returns whichever period summary is present, preferring the
// shortest window (most specific to "now").
func (e timeseriesEntry) symbolCode() string {
	if e.Data.Next1Hours != nil {
		return e.Data.Next1Hours.Summary.SymbolCode
	}
	if e.Data.Next6Hours != nil {
		return e.Data.Next6Hours.Summary.SymbolCode
	}
	return ""
}

// precipAmount and precipProb are a heuristic: Locationforecast's compact
// format doesn't expose a real chance-of-precipitation percentage, only an
// mm amount for the next window. Treat "any amount forecast" as 100% and
// "none" as 0% rather than inventing a fake in-between number.
func (e timeseriesEntry) precipAmount() float64 {
	if e.Data.Next1Hours != nil {
		return e.Data.Next1Hours.Details.PrecipitationAmount
	}
	if e.Data.Next6Hours != nil {
		return e.Data.Next6Hours.Details.PrecipitationAmount
	}
	return 0
}

func (e timeseriesEntry) precipProb() float64 {
	if e.precipAmount() > 0 {
		return 1
	}
	return 0
}

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	lat, lon, city, err := p.resolveLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("metno: %w", err)
	}

	raw, err := p.get(ctx, p.buildURL(lat, lon))
	if err != nil {
		return nil, fmt.Errorf("metno: fetch current: %w", err)
	}

	var parsed forecastResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("metno: decode response: %w", err)
	}
	if len(parsed.Properties.Timeseries) == 0 {
		return nil, fmt.Errorf("metno: empty timeseries")
	}
	entry := parsed.Properties.Timeseries[0]
	d := entry.Data.Instant.Details

	observedAt, err := time.Parse(time.RFC3339, entry.Time)
	if err != nil {
		observedAt = time.Now().UTC()
	}

	return &models.WeatherObservation{
		Location: models.Location{
			City:      city,
			Latitude:  lat,
			Longitude: lon,
		},
		Provider:    providerName,
		Temperature: d.AirTemperature,
		FeelsLike:   d.AirTemperature, // Locationforecast doesn't report an apparent temperature
		Humidity:    int(d.RelativeHumidity),
		Pressure:    d.AirPressureAtSeaLevel,
		WindSpeed:   d.WindSpeed,
		WindDir:     int(d.WindFromDirection),
		PrecipProb:  entry.precipProb(),
		Condition:   mapSymbolCode(entry.symbolCode()),
		IsDay:       symbolIsDay(entry.symbolCode()),
		Description: describeSymbolCode(entry.symbolCode()),
		RawResponse: raw,
		ObservedAt:  observedAt,
		FetchedAt:   time.Now().UTC(),
	}, nil
}

// FetchForecast derives a daily forecast by grouping the timeseries by UTC
// calendar date — Locationforecast has no separate daily endpoint. Known
// gap shared with tomorrow.io/weatherapi in MergeDaily: the response
// carries no UTC offset, so days are bucketed on UTC dates rather than the
// location's local calendar day.
func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	lat, lon, city, err := p.resolveLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("metno: %w", err)
	}

	raw, err := p.get(ctx, p.buildURL(lat, lon))
	if err != nil {
		return nil, fmt.Errorf("metno: fetch forecast: %w", err)
	}

	var parsed forecastResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("metno: decode forecast response: %w", err)
	}

	type dayAccum struct {
		date              time.Time
		tempMin, tempMax  float64
		humiditySum       float64
		windSum           float64
		precipSum         float64
		count             int
		noonSymbol        string
		noonDistanceHours float64
	}
	days := map[string]*dayAccum{}
	var order []string

	for _, e := range parsed.Properties.Timeseries {
		t, err := time.Parse(time.RFC3339, e.Time)
		if err != nil {
			continue
		}
		key := t.Format("2006-01-02")
		acc, ok := days[key]
		if !ok {
			acc = &dayAccum{date: t.Truncate(24 * time.Hour), tempMin: e.Data.Instant.Details.AirTemperature, tempMax: e.Data.Instant.Details.AirTemperature, noonDistanceHours: 999}
			days[key] = acc
			order = append(order, key)
		}
		temp := e.Data.Instant.Details.AirTemperature
		if temp < acc.tempMin {
			acc.tempMin = temp
		}
		if temp > acc.tempMax {
			acc.tempMax = temp
		}
		acc.humiditySum += e.Data.Instant.Details.RelativeHumidity
		acc.windSum += e.Data.Instant.Details.WindSpeed
		acc.precipSum += e.precipAmount()
		acc.count++

		distFromNoon := absFloat(float64(t.Hour()) - 12)
		if code := e.symbolCode(); code != "" && distFromNoon < acc.noonDistanceHours {
			acc.noonSymbol = code
			acc.noonDistanceHours = distFromNoon
		}
	}

	sort.Strings(order)
	if req.Days > 0 && req.Days < len(order) {
		order = order[:req.Days]
	}

	forecast := &models.ProviderForecast{
		Location:    models.Location{City: city},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, key := range order {
		acc := days[key]
		forecast.Days = append(forecast.Days, models.DailyForecast{
			Date:        acc.date,
			TempMin:     acc.tempMin,
			TempMax:     acc.tempMax,
			Humidity:    int(acc.humiditySum / float64(acc.count)),
			WindSpeed:   acc.windSum / float64(acc.count),
			PrecipProb:  boolToProb(acc.precipSum > 0),
			Condition:   mapSymbolCode(acc.noonSymbol),
			Description: describeSymbolCode(acc.noonSymbol),
		})
	}
	return forecast, nil
}

func (p *Provider) FetchHourly(ctx context.Context, req models.WeatherRequest) (*models.ProviderHourlyForecast, error) {
	lat, lon, city, err := p.resolveLocation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("metno: %w", err)
	}

	raw, err := p.get(ctx, p.buildURL(lat, lon))
	if err != nil {
		return nil, fmt.Errorf("metno: fetch hourly: %w", err)
	}

	var parsed forecastResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("metno: decode hourly response: %w", err)
	}

	hourly := &models.ProviderHourlyForecast{
		Location:    models.Location{City: city},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, e := range parsed.Properties.Timeseries {
		t, err := time.Parse(time.RFC3339, e.Time)
		if err != nil {
			continue
		}
		hourly.Hours = append(hourly.Hours, models.HourlyForecast{
			Time:        t,
			Temperature: e.Data.Instant.Details.AirTemperature,
			PrecipProb:  e.precipProb(),
			Condition:   mapSymbolCode(e.symbolCode()),
			Description: describeSymbolCode(e.symbolCode()),
		})
	}
	return hourly, nil
}

// resolveLocation prefers req.Lat/Lon; falls back to geocoding req.City via
// the same free Open-Meteo endpoint the openmeteo provider uses.
func (p *Provider) resolveLocation(ctx context.Context, req models.WeatherRequest) (lat, lon float64, city string, err error) {
	if req.Lat != 0 || req.Lon != 0 {
		return req.Lat, req.Lon, req.City, nil
	}
	if req.City == "" {
		return 0, 0, "", fmt.Errorf("city or lat/lon required")
	}

	r, err := geocode.Resolve(ctx, p.client, req.City)
	if err != nil {
		return 0, 0, "", err
	}
	return r.Lat, r.Lon, r.Name, nil
}

func (p *Provider) buildURL(lat, lon float64) string {
	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%.4f", lat))
	params.Set("lon", fmt.Sprintf("%.4f", lon))
	return fmt.Sprintf("%s/compact?%s", p.baseURL, params.Encode())
}

// get performs a GET request and returns the raw response body, treating any
// non-2xx status as an error. A custom User-Agent is mandatory — MET Norway
// returns 403 without one.
func (p *Provider) get(ctx context.Context, reqURL string) ([]byte, error) {
	return providers.HTTPGet(ctx, p.client, reqURL, map[string]string{"User-Agent": p.userAgent})
}

// mapSymbolCode normalises MET Norway's symbol_code (e.g. "clearsky_day",
// "lightrainshowersandthunder_night") into our shared WeatherCondition enum.
// Codes are suffixed with _day/_night/_polartwilight, so this matches on
// substrings rather than exact values.
func mapSymbolCode(code string) models.WeatherCondition {
	switch {
	case code == "":
		return models.ConditionUnknown
	case strings.Contains(code, "thunder"):
		return models.ConditionThunder
	case strings.Contains(code, "snow"):
		return models.ConditionSnow
	case strings.Contains(code, "sleet"):
		return models.ConditionSleet
	case strings.Contains(code, "rain"):
		return models.ConditionRain
	case strings.Contains(code, "fog"):
		return models.ConditionFog
	case strings.Contains(code, "cloudy"):
		return models.ConditionCloudy
	case strings.HasPrefix(code, "fair"):
		return models.ConditionClear // mostly clear, a few clouds
	case strings.HasPrefix(code, "clearsky"):
		return models.ConditionClear
	default:
		return models.ConditionUnknown
	}
}

// symbolIsDay reads the _day/_night suffix MET Norway attaches to symbol
// codes that visually differ by daylight (e.g. "clearsky_day" vs
// "clearsky_night"). Not every code has one — an overcast sky looks the same
// at any hour — so codes without either suffix return nil rather than
// guessing. _polartwilight (the sun near the horizon, common at high
// latitudes) is treated as day.
func symbolIsDay(code string) *bool {
	switch {
	case strings.HasSuffix(code, "_day"), strings.HasSuffix(code, "_polartwilight"):
		return providers.BoolPtr(true)
	case strings.HasSuffix(code, "_night"):
		return providers.BoolPtr(false)
	default:
		return nil
	}
}

// describeSymbolCode turns a symbol_code into a short human-readable label
// by dropping the _day/_night/_polartwilight suffix and underscores.
func describeSymbolCode(code string) string {
	if code == "" {
		return "unknown"
	}
	for _, suffix := range []string{"_day", "_night", "_polartwilight"} {
		code = strings.TrimSuffix(code, suffix)
	}
	return strings.ReplaceAll(code, "_", " ")
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func boolToProb(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return true
}
