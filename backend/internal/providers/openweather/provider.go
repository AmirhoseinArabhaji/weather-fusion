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
	Dt  int64 `json:"dt"`
	Sys struct {
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
	units := req.Units
	if units == "" {
		units = "metric"
	}

	params := url.Values{}
	params.Set("appid", p.apiKey)
	params.Set("units", units)
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

// maxForecastPoints is the free 5 Day / 3 Hour Forecast API's hard cap:
// 5 days at 3-hour resolution (8 points/day).
const maxForecastPoints = 40

// forecastListResponse mirrors data/2.5/forecast's response shape.
type forecastListResponse struct {
	List []forecastEntry `json:"list"`
	City struct {
		Name     string `json:"name"`
		Timezone int    `json:"timezone"` // seconds offset from UTC
	} `json:"city"`
}

type forecastEntry struct {
	Dt      int64               `json:"dt"`
	Weather []weatherDescriptor `json:"weather"`
	Main    struct {
		Temp     float64 `json:"temp"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
	Pop float64 `json:"pop"`
}

// FetchForecast retrieves a 3-hour-interval forecast (up to 5 days / 40
// points) and buckets it into one DailyForecast per calendar date. req.Days
// beyond 5 is silently capped — the underlying API doesn't offer more.
//
// data/2.5/forecast/daily (native daily, up to 16 days) would be a better
// fit for this model but 401s on the current API key's plan; this endpoint
// is the one that actually works, so it's what's wired in.
func (p *Provider) FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error) {
	days := req.Days
	if days == 0 {
		days = 5
	}
	cnt := min(days*8, maxForecastPoints)

	raw, err := p.get(ctx, p.buildForecastURL(req, cnt))
	if err != nil {
		return nil, fmt.Errorf("openweather: fetch forecast: %w", err)
	}

	var parsed forecastListResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("openweather: decode forecast response: %w", err)
	}

	offset := parsed.City.Timezone
	return &models.ProviderForecast{
		Location:         models.Location{City: parsed.City.Name},
		Provider:         providerName,
		Days:             groupIntoDailyForecast(parsed.List, offset),
		RawResponse:      raw,
		FetchedAt:        time.Now().UTC(),
		UTCOffsetSeconds: &offset,
	}, nil
}

// buildForecastURL prefers city name (native to this endpoint); falls back to lat/lon.
func (p *Provider) buildForecastURL(req models.WeatherRequest, cnt int) string {
	units := req.Units
	if units == "" {
		units = "metric"
	}

	params := url.Values{}
	params.Set("appid", p.apiKey)
	params.Set("units", units)
	params.Set("cnt", fmt.Sprintf("%d", cnt))
	if req.City != "" {
		params.Set("q", req.City)
	} else {
		params.Set("lat", fmt.Sprintf("%f", req.Lat))
		params.Set("lon", fmt.Sprintf("%f", req.Lon))
	}
	return fmt.Sprintf("%s/forecast?%s", p.baseURL, params.Encode())
}

// groupIntoDailyForecast buckets 3-hourly entries by calendar date at the
// query location (offsetSeconds), not raw UTC date — a fixed UTC-midnight
// truncation splits one real local day into two whenever the offset pushes a
// 3-hourly sample's local clock reading onto the other side of midnight from
// its UTC one (e.g. any positive offset's early-morning local hours land in
// the *previous* UTC calendar date). Aggregates: min/max temp across the day,
// average humidity/wind, the day's highest rain chance, most frequent condition.
func groupIntoDailyForecast(entries []forecastEntry, offsetSeconds int) []models.DailyForecast {
	type bucket struct {
		date        time.Time
		tempMin     float64
		tempMax     float64
		humiditySum int
		windSum     float64
		popMax      float64
		count       int
		conditions  map[string]int
		descByCond  map[string]string
	}

	offset := time.Duration(offsetSeconds) * time.Second
	buckets := make(map[string]*bucket)
	var order []string

	for _, e := range entries {
		t := time.Unix(e.Dt, 0).UTC()
		local := t.Add(offset)
		key := local.Format("2006-01-02")
		b, ok := buckets[key]
		if !ok {
			// Representative Date = that local calendar day's midnight,
			// converted back to its true UTC instant — same convention
			// consensus.MergeDaily expects from every provider.
			localMidnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
			b = &bucket{
				date:       localMidnight.Add(-offset),
				tempMin:    e.Main.Temp,
				tempMax:    e.Main.Temp,
				conditions: map[string]int{},
				descByCond: map[string]string{},
			}
			buckets[key] = b
			order = append(order, key)
		}
		if e.Main.Temp < b.tempMin {
			b.tempMin = e.Main.Temp
		}
		if e.Main.Temp > b.tempMax {
			b.tempMax = e.Main.Temp
		}
		b.humiditySum += e.Main.Humidity
		b.windSum += e.Wind.Speed
		if e.Pop > b.popMax {
			b.popMax = e.Pop
		}
		if len(e.Weather) > 0 {
			main := e.Weather[0].Main
			b.conditions[main]++
			if _, seen := b.descByCond[main]; !seen {
				b.descByCond[main] = e.Weather[0].Description
			}
		}
		b.count++
	}

	days := make([]models.DailyForecast, 0, len(order))
	for _, key := range order {
		b := buckets[key]

		majorCondition, majorCount := "", 0
		for cond, count := range b.conditions {
			if count > majorCount {
				majorCondition, majorCount = cond, count
			}
		}

		var avgHumidity int
		var avgWind float64
		if b.count > 0 {
			avgHumidity = b.humiditySum / b.count
			avgWind = b.windSum / float64(b.count)
		}

		days = append(days, models.DailyForecast{
			Date:        b.date,
			TempMin:     b.tempMin,
			TempMax:     b.tempMax,
			Humidity:    avgHumidity,
			WindSpeed:   avgWind,
			PrecipProb:  b.popMax,
			Condition:   mapCondition(majorCondition),
			Description: b.descByCond[majorCondition],
		})
	}
	return days
}

// FetchHourly reuses the same 3-hour-interval endpoint as FetchForecast, but
// returns the raw per-timestamp readings instead of bucketing them into days.
func (p *Provider) FetchHourly(ctx context.Context, req models.WeatherRequest) (*models.ProviderHourlyForecast, error) {
	raw, err := p.get(ctx, p.buildForecastURL(req, maxForecastPoints))
	if err != nil {
		return nil, fmt.Errorf("openweather: fetch hourly: %w", err)
	}

	var parsed forecastListResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("openweather: decode hourly response: %w", err)
	}

	hourly := &models.ProviderHourlyForecast{
		Location:    models.Location{City: parsed.City.Name},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, e := range parsed.List {
		condition := models.ConditionUnknown
		description := ""
		if len(e.Weather) > 0 {
			condition = mapCondition(e.Weather[0].Main)
			description = e.Weather[0].Description
		}
		hourly.Hours = append(hourly.Hours, models.HourlyForecast{
			Time:        time.Unix(e.Dt, 0).UTC(),
			Temperature: e.Main.Temp,
			PrecipProb:  e.Pop,
			Condition:   condition,
			Description: description,
		})
	}
	return hourly, nil
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return p.apiKey != ""
}
