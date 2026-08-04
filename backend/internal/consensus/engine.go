// Package consensus merges results from multiple WeatherProviders into a single
// agreed-upon observation using configurable strategies.
package consensus

import (
	"context"
	"log/slog"

	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
)

// Engine combines data from multiple providers.
type Engine interface {
	// Current returns a merged WeatherData from all healthy providers.
	Current(ctx context.Context, req models.WeatherRequest) (*models.ConsensusResult, error)
}

// engine is the default implementation.
type engine struct {
	providers []providers.WeatherProvider
	log       *slog.Logger
}

// NewEngine creates a new consensus Engine with the given providers.
func NewEngine(provs []providers.WeatherProvider, log *slog.Logger) Engine {
	return &engine{
		providers: provs,
		log:       log.With("component", "consensus"),
	}
}

// Current collects current weather from all providers, then averages numeric
// fields and picks the most common condition.
//
// TODO: implement a proper weighted-average or Bayesian consensus strategy.
func (e *engine) Current(ctx context.Context, req models.WeatherRequest) (*models.ConsensusResult, error) {
	var results []*models.WeatherData
	var sources []string

	for _, p := range e.providers {
		if !p.IsHealthy(ctx) {
			e.log.WarnContext(ctx, "provider unhealthy, skipping", "provider", p.Name())
			continue
		}
		data, err := p.FetchCurrent(ctx, req)
		if err != nil {
			e.log.ErrorContext(ctx, "provider fetch error", "provider", p.Name(), "error", err)
			continue
		}
		results = append(results, data)
		sources = append(sources, p.Name())
	}

	if len(results) == 0 {
		return nil, &providerError{msg: "no providers returned data"}
	}

	merged := average(results)
	return &models.ConsensusResult{
		WeatherData: *merged,
		Confidence:  confidenceScore(len(results), len(e.providers)),
		Sources:     sources,
	}, nil
}

// average computes the mean of numeric weather fields across provider results.
func average(results []*models.WeatherData) *models.WeatherData {
	n := float64(len(results))
	merged := &models.WeatherData{
		Location:  results[0].Location,
		Condition: results[0].Condition, // TODO: pick majority condition
		Provider:  "consensus",
	}
	for _, r := range results {
		merged.Temperature += r.Temperature
		merged.FeelsLike += r.FeelsLike
		merged.Humidity += r.Humidity
		merged.Pressure += r.Pressure
		merged.WindSpeed += r.WindSpeed
		merged.UVIndex += r.UVIndex
	}
	merged.Temperature /= n
	merged.FeelsLike /= n
	merged.Humidity = int(float64(merged.Humidity) / n)
	merged.Pressure /= n
	merged.WindSpeed /= n
	merged.UVIndex /= n
	return merged
}

// confidenceScore is a simple ratio: providers that responded vs total registered.
func confidenceScore(responded, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(responded) / float64(total)
}

// providerError is a local error type for consensus failures.
type providerError struct{ msg string }

func (e *providerError) Error() string { return e.msg }
