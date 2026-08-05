package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/amirhosein/weather-fusion/internal/cache"
	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/repositories"
)

type weatherService struct {
	repo  repositories.WeatherRepository
	cache cache.Cache
	log   *slog.Logger
}

func NewWeatherService(repo repositories.WeatherRepository, cache cache.Cache, log *slog.Logger) WeatherService {
	return &weatherService{
		repo:  repo,
		cache: cache,
		log:   log.With("service", "weather"),
	}
}

func (s *weatherService) GetCurrent(ctx context.Context, req models.WeatherRequest) (*models.WeatherData, error) {
	cacheKey := fmt.Sprintf("weather:current:%s", req.City)

	if raw, err := s.cache.Get(ctx, cacheKey); err == nil {
		var data models.WeatherData
		if jsonErr := json.Unmarshal([]byte(raw), &data); jsonErr == nil {
			s.log.DebugContext(ctx, "cache hit", "key", cacheKey)
			return &data, nil
		}
	}

	var data *models.WeatherData
	var err error
	if s.repo != nil {
		data, err = s.repo.GetLatestByCity(ctx, req.City)
	}
	if s.repo == nil || err != nil {
		s.log.WarnContext(ctx, "no data in db, returning stub", "city", req.City)
		data = &models.WeatherData{
			Location:    models.Location{City: req.City},
			Temperature: 0,
			Condition:   models.ConditionUnknown,
			Description: "no data available yet",
			ObservedAt:  time.Now().UTC(),
		}
	}

	if err := s.cache.Set(ctx, cacheKey, data, 5*time.Minute); err != nil {
		s.log.WarnContext(ctx, "cache set failed", "error", err)
	}

	return data, nil
}

func (s *weatherService) GetForecast(ctx context.Context, req models.WeatherRequest) (*models.Forecast, error) {
	cacheKey := fmt.Sprintf("weather:forecast:%s", req.City)

	if raw, err := s.cache.Get(ctx, cacheKey); err == nil {
		var f models.Forecast
		if jsonErr := json.Unmarshal([]byte(raw), &f); jsonErr == nil {
			s.log.DebugContext(ctx, "cache hit", "key", cacheKey)
			return &f, nil
		}
	}

	var forecast *models.Forecast
	var err error
	if s.repo != nil {
		forecast, err = s.repo.GetForecastByCity(ctx, req.City)
	}
	if s.repo == nil || err != nil {
		s.log.WarnContext(ctx, "no forecast in db, returning stub", "city", req.City)
		forecast = &models.Forecast{
			Location:  models.Location{City: req.City},
			FetchedAt: time.Now().UTC(),
		}
	}

	if err := s.cache.Set(ctx, cacheKey, forecast, 30*time.Minute); err != nil {
		s.log.WarnContext(ctx, "cache set failed", "error", err)
	}

	return forecast, nil
}
