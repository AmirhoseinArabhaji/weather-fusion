package consensus

import (
	"sort"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
)

// MergeDaily combines multiple providers' daily forecasts into one per-date
// consensus forecast, aligning by calendar date. Providers cover different
// date ranges (5–16 days depending on the provider/plan), so a date only
// appears in the result if at least one provider reported it — it's averaged
// across however many did.
func MergeDaily(forecasts []*models.ProviderForecast) []models.DailyForecast {
	type bucket struct {
		date       time.Time
		tempMins   []float64
		tempMaxs   []float64
		humidities []float64
		winds      []float64
		precips    []float64
		conditions map[models.WeatherCondition]int
		descByCond map[models.WeatherCondition]string
	}

	buckets := map[string]*bucket{}
	var order []string

	for _, f := range forecasts {
		if f == nil {
			continue
		}
		for _, d := range f.Days {
			// Providers disagree on what a "day" anchors to: some use UTC
			// midnight, others local midnight at the query location (which,
			// once normalised to UTC, lands a few hours into the previous
			// calendar day for positive-offset timezones). A straight UTC
			// date string then splits one real day into two buckets. Shifting
			// forward before truncating re-centers the boundary away from
			// midnight so both conventions land in the same bucket — a
			// heuristic, not exact for extreme UTC offsets, but covers the
			// common case without needing each provider's own tz semantics.
			shifted := d.Date.Add(6 * time.Hour)
			key := shifted.Format("2006-01-02")
			b, ok := buckets[key]
			if !ok {
				repDate := time.Date(shifted.Year(), shifted.Month(), shifted.Day(), 0, 0, 0, 0, time.UTC)
				b = &bucket{
					date:       repDate,
					conditions: map[models.WeatherCondition]int{},
					descByCond: map[models.WeatherCondition]string{},
				}
				buckets[key] = b
				order = append(order, key)
			}
			b.tempMins = append(b.tempMins, d.TempMin)
			b.tempMaxs = append(b.tempMaxs, d.TempMax)
			b.humidities = append(b.humidities, float64(d.Humidity))
			b.winds = append(b.winds, d.WindSpeed)
			b.precips = append(b.precips, d.PrecipProb)
			b.conditions[d.Condition]++
			if _, seen := b.descByCond[d.Condition]; !seen {
				b.descByCond[d.Condition] = d.Description
			}
		}
	}
	sort.Strings(order)

	result := make([]models.DailyForecast, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		condition := majorityOf(b.conditions)
		// Average of how much providers disagree on the day's low and its
		// high — one number for "how confident is this day", same role
		// TempStdDev plays for current weather.
		spread := (maxOf(b.tempMaxs) - minOf(b.tempMaxs) + maxOf(b.tempMins) - minOf(b.tempMins)) / 2

		result = append(result, models.DailyForecast{
			Date:        b.date,
			TempMin:     mean(b.tempMins),
			TempMax:     mean(b.tempMaxs),
			TempSpread:  spread,
			Humidity:    int(mean(b.humidities)),
			WindSpeed:   mean(b.winds),
			PrecipProb:  mean(b.precips),
			Condition:   condition,
			Description: b.descByCond[condition],
		})
	}
	return result
}

// MergeHourly combines multiple providers' hourly forecasts into one
// per-hour consensus, aligning by truncated hour. Each provider gives a
// single reading per hour (not a range), so TempMin/TempMax here represent
// how much providers disagree on that hour — the hourly equivalent of
// ConsensusResult.TempStdDev.
func MergeHourly(forecasts []*models.ProviderHourlyForecast) []models.ConsensusHourly {
	type bucket struct {
		time       time.Time
		temps      []float64
		precips    []float64
		conditions map[models.WeatherCondition]int
	}

	buckets := map[int64]*bucket{}
	var order []int64

	for _, f := range forecasts {
		if f == nil {
			continue
		}
		for _, h := range f.Hours {
			truncated := h.Time.Truncate(time.Hour)
			key := truncated.Unix()
			b, ok := buckets[key]
			if !ok {
				b = &bucket{time: truncated, conditions: map[models.WeatherCondition]int{}}
				buckets[key] = b
				order = append(order, key)
			}
			b.temps = append(b.temps, h.Temperature)
			b.precips = append(b.precips, h.PrecipProb)
			b.conditions[h.Condition]++
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	result := make([]models.ConsensusHourly, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		result = append(result, models.ConsensusHourly{
			Time:        b.time,
			Temperature: mean(b.temps),
			TempMin:     minOf(b.temps),
			TempMax:     maxOf(b.temps),
			PrecipProb:  mean(b.precips),
			Condition:   majorityOf(b.conditions),
		})
	}
	return result
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func minOf(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxOf(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// majorityOf picks the most frequently reported condition.
func majorityOf(counts map[models.WeatherCondition]int) models.WeatherCondition {
	var best models.WeatherCondition
	var max int
	for cond, count := range counts {
		if count > max {
			best, max = cond, count
		}
	}
	return best
}
