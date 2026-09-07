package consensus

import (
	"math"
	"testing"
	"time"

	"github.com/amirhosein/weather-fusion/internal/models"
)

func TestMinMaxHelpers(t *testing.T) {
	if minOf(nil) != 0 {
		t.Errorf("expected 0 for empty slice, got %f", minOf(nil))
	}
	if maxOf(nil) != 0 {
		t.Errorf("expected 0 for empty slice, got %f", maxOf(nil))
	}

	vals := []float64{10.5, 3.2, 18.9, 7.1}
	if minOf(vals) != 3.2 {
		t.Errorf("expected min 3.2, got %f", minOf(vals))
	}
	if maxOf(vals) != 18.9 {
		t.Errorf("expected max 18.9, got %f", maxOf(vals))
	}
}

func TestMergeDaily(t *testing.T) {
	d1 := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	offset := 7200 // +2 hours

	forecasts := []*models.ProviderForecast{
		{
			Provider:         "openmeteo",
			UTCOffsetSeconds: &offset,
			Days: []models.DailyForecast{
				{
					Date:        d1.Add(-2 * time.Hour), // true UTC of local midnight
					TempMin:     15.0,
					TempMax:     25.0,
					Humidity:    50,
					WindSpeed:   4.0,
					PrecipProb:  0.2,
					Condition:   models.ConditionClear,
					Description: "Clear sky",
				},
			},
		},
		{
			Provider: "weatherapi",
			Days: []models.DailyForecast{
				{
					Date:        d1,
					TempMin:     17.0,
					TempMax:     27.0,
					Humidity:    60,
					WindSpeed:   6.0,
					PrecipProb:  0.4,
					Condition:   models.ConditionClear,
					Description: "Sunny",
				},
			},
		},
		{
			Provider: "met.no",
			Days: []models.DailyForecast{
				{
					Date:        d1,
					TempMin:     16.0,
					TempMax:     26.0,
					Humidity:    55,
					WindSpeed:   5.0,
					PrecipProb:  1.0, // binary heuristic, excluded from precip average
					Condition:   models.ConditionRain,
					Description: "Light rain",
				},
			},
		},
	}

	res := MergeDaily(forecasts)
	if len(res) != 1 {
		t.Fatalf("expected 1 merged day, got %d", len(res))
	}

	day := res[0]
	if math.Abs(day.TempMin-16.0) > 1e-6 {
		t.Errorf("expected avg temp_min 16.0, got %f", day.TempMin)
	}
	if math.Abs(day.TempMax-26.0) > 1e-6 {
		t.Errorf("expected avg temp_max 26.0, got %f", day.TempMax)
	}
	// Spread: ((27-25) + (17-15)) / 2 = 2.0
	if math.Abs(day.TempSpread-2.0) > 1e-6 {
		t.Errorf("expected temp spread 2.0, got %f", day.TempSpread)
	}
	// Precip: (0.2 + 0.4) / 2 = 0.3 (met.no excluded)
	if math.Abs(day.PrecipProb-0.3) > 1e-6 {
		t.Errorf("expected precip 0.3, got %f", day.PrecipProb)
	}
	if day.Condition != models.ConditionClear {
		t.Errorf("expected majority condition Clear, got %s", day.Condition)
	}
}

func TestMergeHourly(t *testing.T) {
	nowHour := time.Now().UTC().Truncate(time.Hour)
	pastHour := nowHour.Add(-2 * time.Hour)
	nextHour := nowHour.Add(time.Hour)

	forecasts := []*models.ProviderHourlyForecast{
		{
			Provider: "openmeteo",
			Hours: []models.HourlyForecast{
				{Time: pastHour, Temperature: 18.0, PrecipProb: 0.1, Condition: models.ConditionClear},
				{Time: nextHour, Temperature: 22.0, PrecipProb: 0.2, Condition: models.ConditionClear},
			},
		},
		{
			Provider: "weatherapi",
			Hours: []models.HourlyForecast{
				{Time: pastHour, Temperature: 19.0, PrecipProb: 0.2, Condition: models.ConditionClear},
				{Time: nextHour, Temperature: 24.0, PrecipProb: 0.4, Condition: models.ConditionClear},
			},
		},
	}

	res := MergeHourly(forecasts)
	if len(res) != 1 {
		t.Fatalf("expected 1 merged future hour (past hours filtered), got %d", len(res))
	}

	h := res[0]
	if !h.Time.Equal(nextHour) {
		t.Errorf("expected hour %v, got %v", nextHour, h.Time)
	}
	if math.Abs(h.Temperature-23.0) > 1e-6 {
		t.Errorf("expected avg temp 23.0, got %f", h.Temperature)
	}
	if h.TempMin != 22.0 || h.TempMax != 24.0 {
		t.Errorf("expected spread 22.0..24.0, got %f..%f", h.TempMin, h.TempMax)
	}
}
