// Package openweather implements the WeatherProvider interface for OpenWeatherMap API.
package openweather

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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

func (p *Provider) FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error) {
	// TODO: call GET {baseURL}/weather?q={city}&appid={apiKey}&units=metric
	p.log.DebugContext(ctx, "fetch current (stub)", "city", req.City)
	return &models.WeatherObservation{
		Location:    models.Location{City: req.City},
		Provider:    providerName,
		Temperature: 20.0,
		Condition:   models.ConditionClear,
		Description: "stub: clear sky",
		ObservedAt:  time.Now().UTC(),
		FetchedAt:   time.Now().UTC(),
	}, nil
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
