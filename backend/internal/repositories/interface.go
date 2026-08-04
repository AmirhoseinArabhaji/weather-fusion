// Package repositories defines data-access interfaces used by the service layer.
package repositories

import (
	"context"

	"github.com/amirhosein/weather-fusion/internal/models"
)

// WeatherRepository abstracts all weather-related database operations.
type WeatherRepository interface {
	SaveCurrent(ctx context.Context, data *models.WeatherData) error
	GetLatestByCity(ctx context.Context, city string) (*models.WeatherData, error)
	GetHistoryByCity(ctx context.Context, city string, page, pageSize int) ([]*models.WeatherData, int, error)
	SaveForecast(ctx context.Context, forecast *models.Forecast) error
	GetForecastByCity(ctx context.Context, city string) (*models.Forecast, error)
}
