package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/amirhosein/weather-fusion/internal/cache"
	"github.com/amirhosein/weather-fusion/internal/consensus"
	"github.com/amirhosein/weather-fusion/internal/models"
)

type weatherService struct {
	consensus consensus.Engine
	cache     cache.Cache
	log       *slog.Logger
}

func NewWeatherService(eng consensus.Engine, cache cache.Cache, log *slog.Logger) WeatherService {
	return &weatherService{
		consensus: eng,
		cache:     cache,
		log:       log.With("service", "weather"),
	}
}

func (s *weatherService) GetCurrent(ctx context.Context, req models.WeatherRequest) (*models.ConsensusResult, error) {
	cacheKey := fmt.Sprintf("weather:current:%s", req.City)

	// Check Redis first
	if raw, err := s.cache.Get(ctx, cacheKey); err == nil {
		var result models.ConsensusResult
		if jsonErr := json.Unmarshal([]byte(raw), &result); jsonErr == nil {
			s.log.DebugContext(ctx, "cache hit", "key", cacheKey)
			return &result, nil
		}
	}

	// Cache miss — run consensus engine across all providers
	result, err := s.consensus.Current(ctx, req)
	if err != nil {
		return nil, err
	}

	result.GeneratedAt = time.Now().UTC()

	// Write back to cache (non-blocking on error)
	if cacheErr := s.cache.Set(ctx, cacheKey, result, 10*time.Minute); cacheErr != nil {
		s.log.WarnContext(ctx, "cache set failed", "error", cacheErr)
	}

	return result, nil
}
