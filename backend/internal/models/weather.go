// Package models defines the core domain types shared across all layers.
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WeatherCondition is a normalised sky-state code shared across all providers.
type WeatherCondition string

const (
	ConditionClear   WeatherCondition = "clear"
	ConditionCloudy  WeatherCondition = "cloudy"
	ConditionRain    WeatherCondition = "rain"
	ConditionSnow    WeatherCondition = "snow"
	ConditionThunder WeatherCondition = "thunder"
	ConditionFog     WeatherCondition = "fog"
	ConditionUnknown WeatherCondition = "unknown"
)

// Location is a geographic point resolved from a city name or coordinates.
type Location struct {
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

// WeatherObservation is a single normalised point-in-time reading from one provider.
// It maps directly to the weather_observations table.
// RawResponse stores the original API JSON for auditing and future re-processing.
type WeatherObservation struct {
	ID          uuid.UUID        `json:"id"          db:"id"`
	Location    Location         `json:"location"`
	Provider    string           `json:"provider"`    // "weatherapi", "openweather", …
	Temperature float64          `json:"temperature"` // °C
	FeelsLike   float64          `json:"feels_like"`  // °C
	Humidity    int              `json:"humidity"`    // percent
	Pressure    float64          `json:"pressure"`    // hPa
	WindSpeed   float64          `json:"wind_speed"`  // m/s
	WindDir     int              `json:"wind_dir"`    // degrees
	Visibility  float64          `json:"visibility"`  // km
	UVIndex     float64          `json:"uv_index"`
	PrecipProb  float64          `json:"precip_prob"` // 0–1
	Condition   WeatherCondition `json:"condition"`
	Description string           `json:"description"`
	RawResponse json.RawMessage  `json:"raw_response" db:"raw_response"` // original API JSON (JSONB)
	ObservedAt  time.Time        `json:"observed_at"  db:"observed_at"`
	FetchedAt   time.Time        `json:"fetched_at"   db:"fetched_at"`
}

// DailyForecast holds the predicted conditions for a single day from one provider.
type DailyForecast struct {
	Date        time.Time        `json:"date"`
	TempMin     float64          `json:"temp_min"`    // °C
	TempMax     float64          `json:"temp_max"`    // °C
	Humidity    int              `json:"humidity"`    // percent
	WindSpeed   float64          `json:"wind_speed"`  // m/s
	PrecipProb  float64          `json:"precip_prob"` // 0–1
	Condition   WeatherCondition `json:"condition"`
	Description string           `json:"description"`
}

// ProviderForecast is a full forecast response from a single provider.
type ProviderForecast struct {
	Location    Location        `json:"location"`
	Provider    string          `json:"provider"`
	Days        []DailyForecast `json:"days"`
	RawResponse json.RawMessage `json:"raw_response"` // original API JSON
	FetchedAt   time.Time       `json:"fetched_at"`
}

// ConsensusResult is the merged output of all provider observations for one location.
// This is what the consensus engine produces and what the API returns to the client.
type ConsensusResult struct {
	Location      Location           `json:"location"`
	Temperature   float64            `json:"temperature"`    // average °C
	TempStdDev    float64            `json:"temp_std_dev"`   // spread between providers
	Humidity      float64            `json:"humidity"`       // average percent
	WindSpeed     float64            `json:"wind_speed"`     // average m/s
	PrecipProb    float64            `json:"precip_prob"`    // average 0–1
	Condition     WeatherCondition   `json:"condition"`      // majority vote
	Confidence    float64            `json:"confidence"`     // 0–1 based on agreement
	Providers     []WeatherObservation `json:"providers"`    // individual readings
	Summary       string             `json:"summary"`        // LLM-generated explanation
	GeneratedAt   time.Time          `json:"generated_at"`
}

// WeatherRequest carries the inbound query parameters.
type WeatherRequest struct {
	City string  `form:"city" binding:"required_without=Lat"`
	Lat  float64 `form:"lat"`
	Lon  float64 `form:"lon"`
	Days int     `form:"days" binding:"omitempty,min=1,max=14"`
}
