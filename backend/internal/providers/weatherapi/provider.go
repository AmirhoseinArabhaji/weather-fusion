// Package weatherapi implements the WeatherProvider interface for weatherapi.com.
package weatherapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		client:  providers.NewHTTPClient(),
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
		TempF            float64 `json:"temp_f"`
		IsDay            int     `json:"is_day"` // 1 = day, 0 = night
		Condition        struct {
			Text string `json:"text"`
			Code int    `json:"code"`
		} `json:"condition"`
		WindKph    float64 `json:"wind_kph"`
		WindMph    float64 `json:"wind_mph"`
		WindDegree int     `json:"wind_degree"`
		PressureMb float64 `json:"pressure_mb"`
		Humidity   int     `json:"humidity"`
		FeelslikeC float64 `json:"feelslike_c"`
		FeelslikeF float64 `json:"feelslike_f"`
		VisKm      float64 `json:"vis_km"`
		VisMiles   float64 `json:"vis_miles"`
		UV         float64 `json:"uv"`
	} `json:"current"`
}

// apiError mirrors WeatherAPI's error envelope: {"code":..,"message":".."}.
type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// forecastResponse mirrors the /forecast.json response shape (astro/alerts
// are ignored; day and hour cover our DailyForecast/HourlyForecast models).
type forecastResponse struct {
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Forecast struct {
		Forecastday []struct {
			DateEpoch int64 `json:"date_epoch"`
			Day       struct {
				MaxTempC        float64 `json:"maxtemp_c"`
				MaxTempF        float64 `json:"maxtemp_f"`
				MinTempC        float64 `json:"mintemp_c"`
				MinTempF        float64 `json:"mintemp_f"`
				AvgHumidity     float64 `json:"avghumidity"`
				MaxWindKph      float64 `json:"maxwind_kph"`
				MaxWindMph      float64 `json:"maxwind_mph"`
				DailyChanceRain float64 `json:"daily_chance_of_rain"`
				Condition       struct {
					Text string `json:"text"`
				} `json:"condition"`
			} `json:"day"`
			Hour []struct {
				TimeEpoch    int64   `json:"time_epoch"`
				TempC        float64 `json:"temp_c"`
				TempF        float64 `json:"temp_f"`
				ChanceOfRain float64 `json:"chance_of_rain"`
				Condition    struct {
					Text string `json:"text"`
				} `json:"condition"`
			} `json:"hour"`
		} `json:"forecastday"`
	} `json:"forecast"`
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

	temp, feelsLike, windSpeed, visibility := parsed.Current.TempC, parsed.Current.FeelslikeC, parsed.Current.WindKph/3.6, parsed.Current.VisKm
	if req.Units == "imperial" {
		temp, feelsLike, windSpeed, visibility = parsed.Current.TempF, parsed.Current.FeelslikeF, parsed.Current.WindMph, parsed.Current.VisMiles
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
		Temperature: temp,
		FeelsLike:   feelsLike,
		Humidity:    parsed.Current.Humidity,
		Pressure:    parsed.Current.PressureMb,
		WindSpeed:   windSpeed,
		WindDir:     parsed.Current.WindDegree,
		Visibility:  visibility,
		UVIndex:     parsed.Current.UV,
		IsDay:       providers.BoolPtr(parsed.Current.IsDay == 1),
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
	params.Set("q", cityOrCoords(req))
	return fmt.Sprintf("%s/current.json?%s", p.baseURL, params.Encode())
}

// cityOrCoords formats a request as WeatherAPI's "q" value: city name if
// present, else "lat,lon".
func cityOrCoords(req models.WeatherRequest) string {
	if req.City != "" {
		return req.City
	}
	return fmt.Sprintf("%f,%f", req.Lat, req.Lon)
}

// get performs a GET request and returns the raw response body. Non-2xx
// responses are decoded as WeatherAPI's {code,message} error envelope when
// possible, falling back to the raw body text.
func (p *Provider) get(ctx context.Context, reqURL string) ([]byte, error) {
	body, err := providers.HTTPGet(ctx, p.client, reqURL, nil)
	if err != nil {
		var statusErr *providers.StatusError
		if errors.As(err, &statusErr) {
			var apiErr apiError
			if json.Unmarshal(statusErr.Body, &apiErr) == nil && apiErr.Message != "" {
				return nil, fmt.Errorf("status %d: code %d: %s", statusErr.Code, apiErr.Code, apiErr.Message)
			}
		}
		return nil, err
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
	case strings.Contains(t, "sleet"), strings.Contains(t, "ice"), strings.Contains(t, "blizzard"):
		return models.ConditionSleet
	case strings.Contains(t, "snow"):
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
	days := req.Days
	if days == 0 {
		days = 7
	}

	raw, err := p.get(ctx, p.buildForecastURL(req, days))
	if err != nil {
		return nil, fmt.Errorf("weatherapi: fetch forecast: %w", err)
	}

	var parsed forecastResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherapi: decode forecast response: %w", err)
	}

	return buildProviderForecast(parsed, req, raw), nil
}

// buildProviderForecast maps a parsed forecastResponse (shared by /forecast.json
// and /future.json) into our domain model, honoring req.Units.
func buildProviderForecast(parsed forecastResponse, req models.WeatherRequest, raw []byte) *models.ProviderForecast {
	forecast := &models.ProviderForecast{
		Location:    models.Location{City: parsed.Location.Name},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, fd := range parsed.Forecast.Forecastday {
		tempMin, tempMax, windSpeed := fd.Day.MinTempC, fd.Day.MaxTempC, fd.Day.MaxWindKph/3.6
		if req.Units == "imperial" {
			tempMin, tempMax, windSpeed = fd.Day.MinTempF, fd.Day.MaxTempF, fd.Day.MaxWindMph
		}
		forecast.Days = append(forecast.Days, models.DailyForecast{
			Date:        time.Unix(fd.DateEpoch, 0).UTC(),
			TempMin:     tempMin,
			TempMax:     tempMax,
			Humidity:    int(fd.Day.AvgHumidity),
			WindSpeed:   windSpeed,
			PrecipProb:  fd.Day.DailyChanceRain / 100,
			Condition:   mapCondition(fd.Day.Condition.Text),
			Description: fd.Day.Condition.Text,
		})
	}
	return forecast
}

// buildForecastURL prefers city name; falls back to "lat,lon".
func (p *Provider) buildForecastURL(req models.WeatherRequest, days int) string {
	params := url.Values{}
	params.Set("key", p.apiKey)
	params.Set("days", fmt.Sprintf("%d", days))
	params.Set("q", cityOrCoords(req))
	return fmt.Sprintf("%s/forecast.json?%s", p.baseURL, params.Encode())
}

// FetchHourly reuses the same /forecast.json call as FetchForecast — the
// response already carries an "hour" array per day, just unused until now.
func (p *Provider) FetchHourly(ctx context.Context, req models.WeatherRequest) (*models.ProviderHourlyForecast, error) {
	days := req.Days
	if days == 0 {
		days = 2 // ~48h of hourly data is plenty; keep the request small
	}

	raw, err := p.get(ctx, p.buildForecastURL(req, days))
	if err != nil {
		return nil, fmt.Errorf("weatherapi: fetch hourly: %w", err)
	}

	var parsed forecastResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherapi: decode hourly response: %w", err)
	}

	hourly := &models.ProviderHourlyForecast{
		Location:    models.Location{City: parsed.Location.Name},
		Provider:    providerName,
		RawResponse: raw,
		FetchedAt:   time.Now().UTC(),
	}
	for _, fd := range parsed.Forecast.Forecastday {
		for _, h := range fd.Hour {
			temp := h.TempC
			if req.Units == "imperial" {
				temp = h.TempF
			}
			hourly.Hours = append(hourly.Hours, models.HourlyForecast{
				Time:        time.Unix(h.TimeEpoch, 0).UTC(),
				Temperature: temp,
				PrecipProb:  h.ChanceOfRain / 100,
				Condition:   mapCondition(h.Condition.Text),
				Description: h.Condition.Text,
			})
		}
	}
	return hourly, nil
}

func (p *Provider) IsHealthy(ctx context.Context) bool {
	return p.apiKey != ""
}

// FetchFuture retrieves the forecast for a single date 14-365 days ahead
// (WeatherAPI's /future.json). date must be "yyyy-MM-dd".
func (p *Provider) FetchFuture(ctx context.Context, req models.WeatherRequest, date string) (*models.ProviderForecast, error) {
	params := url.Values{}
	params.Set("key", p.apiKey)
	params.Set("q", cityOrCoords(req))
	params.Set("dt", date)
	reqURL := fmt.Sprintf("%s/future.json?%s", p.baseURL, params.Encode())

	raw, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("weatherapi: fetch future: %w", err)
	}

	// future.json's location/forecast.forecastday shape matches forecast.json.
	var parsed forecastResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherapi: decode future response: %w", err)
	}

	return buildProviderForecast(parsed, req, raw), nil
}

// IPLocation is the result of an IP geolocation lookup (/ip.json).
type IPLocation struct {
	IP          string  `json:"ip"`
	City        string  `json:"city"`
	Region      string  `json:"region"`
	Country     string  `json:"country_name"`
	CountryCode string  `json:"country_code"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	TzID        string  `json:"tz_id"`
}

// LookupIP resolves geolocation info for an IP address.
func (p *Provider) LookupIP(ctx context.Context, ip string) (*IPLocation, error) {
	params := url.Values{}
	params.Set("key", p.apiKey)
	params.Set("q", ip)
	reqURL := fmt.Sprintf("%s/ip.json?%s", p.baseURL, params.Encode())

	raw, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("weatherapi: lookup ip: %w", err)
	}

	var loc IPLocation
	if err := json.Unmarshal(raw, &loc); err != nil {
		return nil, fmt.Errorf("weatherapi: decode ip response: %w", err)
	}
	return &loc, nil
}

// LocationInfo is the result of a timezone/location lookup (/timezone.json).
type LocationInfo struct {
	Name           string  `json:"name"`
	Region         string  `json:"region"`
	Country        string  `json:"country"`
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	TzID           string  `json:"tz_id"`
	LocaltimeEpoch int64   `json:"localtime_epoch"`
	Localtime      string  `json:"localtime"`
}

// GetTimezone resolves location and local-time info for a city or coordinates.
func (p *Provider) GetTimezone(ctx context.Context, req models.WeatherRequest) (*LocationInfo, error) {
	params := url.Values{}
	params.Set("key", p.apiKey)
	params.Set("q", cityOrCoords(req))
	reqURL := fmt.Sprintf("%s/timezone.json?%s", p.baseURL, params.Encode())

	raw, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("weatherapi: get timezone: %w", err)
	}

	var loc LocationInfo
	if err := json.Unmarshal(raw, &loc); err != nil {
		return nil, fmt.Errorf("weatherapi: decode timezone response: %w", err)
	}
	return &loc, nil
}

// Astronomy holds sun/moon data for a location and date (/astronomy.json).
type Astronomy struct {
	Sunrise          string `json:"sunrise"`
	Sunset           string `json:"sunset"`
	Moonrise         string `json:"moonrise"`
	Moonset          string `json:"moonset"`
	MoonPhase        string `json:"moon_phase"`
	MoonIllumination string `json:"moon_illumination"`
}

// GetAstronomy retrieves sunrise/sunset/moon data for a location and date.
// date must be "yyyy-MM-dd", on or after 2015-01-01.
func (p *Provider) GetAstronomy(ctx context.Context, req models.WeatherRequest, date string) (*Astronomy, error) {
	params := url.Values{}
	params.Set("key", p.apiKey)
	params.Set("q", cityOrCoords(req))
	params.Set("dt", date)
	reqURL := fmt.Sprintf("%s/astronomy.json?%s", p.baseURL, params.Encode())

	raw, err := p.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("weatherapi: get astronomy: %w", err)
	}

	var parsed struct {
		Astronomy struct {
			Astro Astronomy `json:"astro"`
		} `json:"astronomy"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("weatherapi: decode astronomy response: %w", err)
	}
	return &parsed.Astronomy.Astro, nil
}
