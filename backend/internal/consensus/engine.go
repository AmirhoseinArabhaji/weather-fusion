// Package consensus merges results from multiple WeatherProviders into a single
// agreed-upon observation using configurable strategies.
package consensus

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
	"github.com/amirhosein/weather-fusion/internal/providers"
)

// Engine combines data from multiple providers.
type Engine interface {
	// Current fetches from all healthy providers and returns a merged ConsensusResult.
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

// Current collects current weather from all providers, averages numeric fields,
// picks the most common condition, and calculates a confidence score based on
// how much providers agree on temperature.
func (e *engine) Current(ctx context.Context, req models.WeatherRequest) (*models.ConsensusResult, error) {
	var observations []*models.WeatherObservation

	for _, p := range e.providers {
		if !p.IsHealthy(ctx) {
			e.log.WarnContext(ctx, "provider unhealthy, skipping", "provider", p.Name())
			continue
		}
		obs, err := p.FetchCurrent(ctx, req)
		if err != nil {
			e.log.ErrorContext(ctx, "provider fetch error", "provider", p.Name(), "error", err)
			continue
		}
		observations = append(observations, obs)
	}

	if len(observations) == 0 {
		return nil, &providerError{msg: "no providers returned data"}
	}

	return Merge(observations, len(e.providers)), nil
}

// Merge combines individually-fetched provider observations into a ConsensusResult.
// totalProviders is the number of providers that were attempted (healthy + unhealthy),
// used for the response-ratio part of the confidence score. Exported so callers that
// fetch observations themselves (e.g. a streaming handler fanning out concurrently)
// can reuse the same averaging/confidence logic instead of duplicating it.
func Merge(observations []*models.WeatherObservation, totalProviders int) *models.ConsensusResult {
	if len(observations) == 0 {
		return nil
	}

	avgTemp, stdDev := temperatureStats(observations)
	avgHumidity := averageHumidity(observations)
	avgWind := averageWindSpeed(observations)
	avgPrecip := averagePrecipProb(observations)
	condition := majorityCondition(observations)
	confidence := confidenceScore(len(observations), totalProviders, stdDev)

	return &models.ConsensusResult{
		Location:    observations[0].Location,
		Temperature: avgTemp,
		TempStdDev:  stdDev,
		Humidity:    avgHumidity,
		WindSpeed:   avgWind,
		PrecipProb:  avgPrecip,
		Condition:   condition,
		Confidence:  confidence,
		Providers:   dereferenceAll(observations),
		GeneratedAt: time.Now().UTC(),
	}
}

func temperatureStats(obs []*models.WeatherObservation) (avg, stdDev float64) {
	n := float64(len(obs))
	for _, o := range obs {
		avg += o.Temperature
	}
	avg /= n
	for _, o := range obs {
		diff := o.Temperature - avg
		stdDev += diff * diff
	}
	stdDev = math.Sqrt(stdDev / n)
	return
}

func averageHumidity(obs []*models.WeatherObservation) float64 {
	var sum float64
	for _, o := range obs {
		sum += float64(o.Humidity)
	}
	return sum / float64(len(obs))
}

func averageWindSpeed(obs []*models.WeatherObservation) float64 {
	var sum float64
	for _, o := range obs {
		sum += o.WindSpeed
	}
	return sum / float64(len(obs))
}

func averagePrecipProb(obs []*models.WeatherObservation) float64 {
	var sum float64
	for _, o := range obs {
		sum += o.PrecipProb
	}
	return sum / float64(len(obs))
}

// majorityCondition picks the most frequently reported condition across providers.
func majorityCondition(obs []*models.WeatherObservation) models.WeatherCondition {
	counts := make(map[models.WeatherCondition]int)
	for _, o := range obs {
		counts[o.Condition]++
	}
	var best models.WeatherCondition
	var max int
	for cond, count := range counts {
		if count > max {
			best = cond
			max = count
		}
	}
	return best
}

// confidenceScore combines provider response ratio with temperature agreement.
// High stdDev = low confidence even if all providers responded.
func confidenceScore(responded, total int, tempStdDev float64) float64 {
	if total == 0 {
		return 0
	}
	responsePart := float64(responded) / float64(total)
	// penalise: stdDev > 5°C starts to significantly reduce confidence
	agreementPart := math.Max(0, 1.0-(tempStdDev/5.0))
	return math.Min(1.0, (responsePart+agreementPart)/2.0)
}

func dereferenceAll(obs []*models.WeatherObservation) []models.WeatherObservation {
	out := make([]models.WeatherObservation, len(obs))
	for i, o := range obs {
		out[i] = *o
	}
	return out
}

// providerError is a local error type for consensus failures.
type providerError struct{ msg string }

func (e *providerError) Error() string { return e.msg }
