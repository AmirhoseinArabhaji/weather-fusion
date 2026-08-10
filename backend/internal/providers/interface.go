// Package providers defines the WeatherProvider interface and related types.
package providers

import (
	"context"

	"github.com/amirhosein/weather-fusion/internal/models"
)

// WeatherProvider is the contract that every weather data source must satisfy.
// New providers (e.g. AccuWeather, Tomorrow.io) only need to implement this
// interface to plug into the consensus engine.
type WeatherProvider interface {
	// Name returns the unique identifier for this provider (e.g. "openweather").
	Name() string

	// FetchCurrent retrieves the current weather conditions for the given location.
	FetchCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherObservation, error)

	// FetchForecast retrieves a multi-day forecast for the given location.
	FetchForecast(ctx context.Context, req models.WeatherRequest) (*models.ProviderForecast, error)

	// FetchHourly retrieves an hourly forecast for the given location.
	FetchHourly(ctx context.Context, req models.WeatherRequest) (*models.ProviderHourlyForecast, error)

	// IsHealthy returns true if the provider's API is reachable.
	IsHealthy(ctx context.Context) bool
}
