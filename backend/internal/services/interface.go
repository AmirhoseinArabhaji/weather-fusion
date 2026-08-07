// Package services defines the business logic interfaces consumed by handlers.
package services

import (
	"context"

	"github.com/amirhosein/weather-fusion/internal/models"
)

// WeatherService defines the business operations for weather data.
type WeatherService interface {
	// GetCurrent returns the consensus weather for a location (cache-aside).
	GetCurrent(ctx context.Context, req models.WeatherRequest) (*models.ConsensusResult, error)
}
