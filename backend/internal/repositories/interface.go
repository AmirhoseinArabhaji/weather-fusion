// Package repositories defines data-access interfaces used by the service layer.
package repositories

import (
	"context"

	"github.com/amirhosein/weather-fusion/internal/models"
)

// WeatherRepository abstracts all weather-related database operations.
type WeatherRepository interface {
	// SaveObservation persists a normalised provider observation (async, non-blocking path).
	SaveObservation(ctx context.Context, obs *models.WeatherObservation) error

	// GetLatestByCity returns the most recent cached observation for a city across all providers.
	GetLatestByCity(ctx context.Context, city string) ([]*models.WeatherObservation, error)

	// GetHistoryByCity returns paginated historical observations for a city.
	GetHistoryByCity(ctx context.Context, city string, page, pageSize int) ([]*models.WeatherObservation, int, error)
}
