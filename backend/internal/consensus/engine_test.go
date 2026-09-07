package consensus

import (
	"math"
	"testing"

	"github.com/amirhosein/weather-fusion/internal/models"
)

func TestMergeEmpty(t *testing.T) {
	res := Merge(nil, 5)
	if res != nil {
		t.Errorf("expected nil result for empty observations, got %+v", res)
	}
}

func TestMergeCalculations(t *testing.T) {
	isDay := true
	isNight := false

	obs := []*models.WeatherObservation{
		{
			Provider:    "openmeteo",
			Temperature: 20.0,
			Humidity:    50,
			WindSpeed:   5.0,
			PrecipProb:  0.2,
			Condition:   models.ConditionClear,
			IsDay:       &isDay,
			Location: models.Location{
				City:     "Tehran",
				Timezone: "Asia/Tehran",
			},
		},
		{
			Provider:    "weatherapi",
			Temperature: 22.0,
			Humidity:    60,
			WindSpeed:   7.0,
			PrecipProb:  0.4,
			Condition:   models.ConditionClear,
			IsDay:       &isDay,
			Location: models.Location{
				City: "Tehran",
			},
		},
		{
			Provider:    "met.no",
			Temperature: 21.0,
			Humidity:    55,
			WindSpeed:   6.0,
			PrecipProb:  1.0, // binary heuristic, must be excluded from average
			Condition:   models.ConditionCloudy,
			IsDay:       &isNight,
			Location: models.Location{
				City: "Tehran",
			},
		},
	}

	res := Merge(obs, 3)
	if res == nil {
		t.Fatal("expected non-nil consensus result")
	}

	if math.Abs(res.Temperature-21.0) > 1e-6 {
		t.Errorf("expected avg temp 21.0, got %f", res.Temperature)
	}

	// Precip average should be (0.2 + 0.4) / 2 = 0.3, excluding met.no
	if math.Abs(res.PrecipProb-0.3) > 1e-6 {
		t.Errorf("expected avg precip 0.3 (excluding met.no), got %f", res.PrecipProb)
	}

	if res.Condition != models.ConditionClear {
		t.Errorf("expected majority condition Clear, got %s", res.Condition)
	}

	if !res.IsDay {
		t.Errorf("expected majority IsDay true, got %v", res.IsDay)
	}

	if res.Location.Timezone != "Asia/Tehran" {
		t.Errorf("expected bestLocation to pick timezone Asia/Tehran, got %s", res.Location.Timezone)
	}
}

func TestConfidenceScore(t *testing.T) {
	if confidenceScore(0, 0, 0) != 0 {
		t.Errorf("expected 0 for total=0")
	}

	scorePerfect := confidenceScore(5, 5, 0.0)
	if scorePerfect != 1.0 {
		t.Errorf("expected 1.0 for perfect agreement, got %f", scorePerfect)
	}

	scoreLow := confidenceScore(5, 5, 5.0)
	if scoreLow >= scorePerfect {
		t.Errorf("expected lower confidence for high stddev, got %f", scoreLow)
	}
}
