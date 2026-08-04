// Package models defines the core domain types shared across all layers.
package models

import (
	"time"

	"github.com/google/uuid"
)

// Location represents a geographic point.
type Location struct {
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

// WeatherCondition is a code describing the sky state (e.g. "clear", "rain").
type WeatherCondition string

const (
	ConditionClear     WeatherCondition = "clear"
	ConditionCloudy    WeatherCondition = "cloudy"
	ConditionRain      WeatherCondition = "rain"
	ConditionSnow      WeatherCondition = "snow"
	ConditionThunder   WeatherCondition = "thunder"
	ConditionFog       WeatherCondition = "fog"
	ConditionUnknown   WeatherCondition = "unknown"
)

// WeatherData represents a single point-in-time weather observation.
type WeatherData struct {
	ID          uuid.UUID        `json:"id"           db:"id"`
	Location    Location         `json:"location"`
	Temperature float64          `json:"temperature"` // Celsius
	FeelsLike   float64          `json:"feels_like"`
	Humidity    int              `json:"humidity"`    // percent
	Pressure    float64          `json:"pressure"`    // hPa
	WindSpeed   float64          `json:"wind_speed"`  // m/s
	WindDir     int              `json:"wind_dir"`    // degrees
	Visibility  float64          `json:"visibility"`  // km
	UVIndex     float64          `json:"uv_index"`
	Condition   WeatherCondition `json:"condition"`
	Description string           `json:"description"`
	IconCode    string           `json:"icon_code"`
	Provider    string           `json:"provider"`    // which API supplied this data
	ObservedAt  time.Time        `json:"observed_at"  db:"observed_at"`
	CreatedAt   time.Time        `json:"created_at"   db:"created_at"`
}

// DailyForecast holds the predicted conditions for a single day.
type DailyForecast struct {
	Date        time.Time        `json:"date"`
	TempMin     float64          `json:"temp_min"`
	TempMax     float64          `json:"temp_max"`
	Humidity    int              `json:"humidity"`
	WindSpeed   float64          `json:"wind_speed"`
	Condition   WeatherCondition `json:"condition"`
	Description string           `json:"description"`
	PrecipProb  float64          `json:"precip_probability"` // 0–1
}

// Forecast is a collection of daily forecasts for a location.
type Forecast struct {
	Location Location        `json:"location"`
	Days     []DailyForecast `json:"days"`
	Provider string          `json:"provider"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// ConsensusResult merges data from multiple providers into a single agreed value.
type ConsensusResult struct {
	WeatherData
	Confidence float64  `json:"confidence"` // 0–1
	Sources    []string `json:"sources"`    // contributing provider names
}

// WeatherRequest carries the parameters for a weather query.
type WeatherRequest struct {
	City      string  `form:"city"      binding:"required_without=Lat"`
	Lat       float64 `form:"lat"`
	Lon       float64 `form:"lon"`
	Units     string  `form:"units"     binding:"omitempty,oneof=metric imperial"`
	Days      int     `form:"days"      binding:"omitempty,min=1,max=14"`
}
