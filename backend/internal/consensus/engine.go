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
	avgHumidity := avgField(observations, func(o *models.WeatherObservation) float64 { return float64(o.Humidity) })
	avgWind := avgField(observations, func(o *models.WeatherObservation) float64 { return o.WindSpeed })
	avgPrecip := avgPrecipProb(observations)
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
		IsDay:       majorityIsDay(observations),
		Confidence:  confidence,
		Providers:   dereferenceAll(observations),
		GeneratedAt: time.Now().UTC(),
	}
}

// bestLocation prefers the first observation with both a city name and a
// timezone; falls back to city-only, then whatever landed first. Providers
// race concurrently, so observations[0] isn't a fixed provider, and not
// every provider reverse-geocodes (city) or reports an IANA timezone —
// picking whichever happened to answer first risked losing either field
// even when another provider in the same batch had it.
func bestLocation(obs []*models.WeatherObservation) models.Location {
	for _, o := range obs {
		if o.Location.City != "" && o.Location.Timezone != "" {
			return o.Location
		}
	}
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

func avgField(obs []*models.WeatherObservation, get func(*models.WeatherObservation) float64) float64 {
	var sum float64
	for _, o := range obs {
		sum += get(o)
	}
	return sum / float64(len(obs))
}

// avgPrecipProb averages PrecipProb across providers, excluding met.no: unlike
// every other provider's graduated 0-1 forecast probability, met.no has no real
// probability field and reports a binary 0/1 ("any precip amount forecast"),
// which skews the average and inflates spread against genuinely graduated readings.
func avgPrecipProb(obs []*models.WeatherObservation) float64 {
	var sum float64
	var n int
	for _, o := range obs {
		if o.Provider == excludedPrecipProvider {
			continue
		}
		sum += o.PrecipProb
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// excludedPrecipProvider is the provider whose PrecipProb is a binary
// heuristic (0/1), not a real graduated probability, and so is excluded
// from precip-probability averaging/spread calculations everywhere.
const excludedPrecipProvider = "met.no"

// majorityCondition picks the most frequently reported condition across providers.
func majorityCondition(obs []*models.WeatherObservation) models.WeatherCondition {
	counts := make(map[models.WeatherCondition]int)
	for _, o := range obs {
		counts[o.Condition]++
	}
	return majorityOf(counts)
}

// majorityIsDay votes among providers that report a day/night signal (not
// all do — see each provider's IsDay comment). Defaults to true when none do,
// since a "day" icon is the more common/expected default.
func majorityIsDay(obs []*models.WeatherObservation) bool {
	var day, night int
	for _, o := range obs {
		if o.IsDay == nil {
			continue
		}
		if *o.IsDay {
			day++
		} else {
			night++
		}
	}
	return night <= day
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
