// Package consensus merges results from multiple WeatherProviders into a single
// agreed-upon observation using configurable strategies.
package consensus

import (
	"math"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
)

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
		Location:    bestLocation(observations),
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

// bestLocation prefers the first observation with a non-empty city name.
// Providers race concurrently, so observations[0] isn't a fixed provider —
// and not all of them reverse-geocode lat/lon input (e.g. Visual Crossing
// doesn't), so blindly taking observations[0].Location risked an empty city
// whenever an unnamed one happened to land first.
func bestLocation(obs []*models.WeatherObservation) models.Location {
	for _, o := range obs {
		if o.Location.City != "" {
			return o.Location
		}
	}
	return obs[0].Location
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
