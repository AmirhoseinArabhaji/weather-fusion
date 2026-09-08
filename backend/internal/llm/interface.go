// Package llm defines the LLMService interface for AI-powered weather analysis.
package llm

import "context"

// ProviderReading carries one provider's observed metrics for LLM synthesis.
type ProviderReading struct {
	Provider    string  `json:"provider"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
	Humidity    int     `json:"humidity"`
	PrecipProb  float64 `json:"precip_prob"`
}

// SummarizeRequest carries the input for a weather summary generation.
type SummarizeRequest struct {
	City        string            `json:"city"`
	Temperature float64           `json:"temperature"`
	FeelsLike   float64           `json:"feels_like"`
	TempMin     float64           `json:"temp_min"`
	TempMax     float64           `json:"temp_max"`
	TempSpread  float64           `json:"temp_spread"`
	TempStdDev  float64           `json:"temp_std_dev"`
	Humidity    float64           `json:"humidity"`
	WindSpeed   float64           `json:"wind_speed"`
	PrecipProb  float64           `json:"precip_prob"`
	Condition   string            `json:"condition"`
	Confidence  float64           `json:"confidence"`
	Description string            `json:"description,omitempty"`
	Providers   []ProviderReading `json:"providers,omitempty"`
}

// AnalyzeRequest carries structured weather data for trend analysis.
type AnalyzeRequest struct {
	City           string
	HistoricalData []map[string]interface{}
	ForecastData   []map[string]interface{}
}

// LLMResponse is the generic output from any LLM operation.
type LLMResponse struct {
	Content    string  // Generated text
	TokensUsed int     // Prompt + completion tokens consumed
	Model      string  // Model that produced the response
	Confidence float64 // 0–1 confidence score where applicable
}

// LLMService is the contract for AI/LLM integrations.
// Swap the implementation (OpenAI, Anthropic, Gemini, local Ollama, …)
// without touching any business logic.
type LLMService interface {
	// Summarize generates a natural-language weather summary for the given conditions.
	Summarize(ctx context.Context, req SummarizeRequest) (*LLMResponse, error)

	// Analyze performs trend analysis over historical and forecast data.
	Analyze(ctx context.Context, req AnalyzeRequest) (*LLMResponse, error)

	// ModelName returns the identifier of the underlying model.
	ModelName() string

	// IsAvailable returns true if the LLM backend is reachable.
	IsAvailable(ctx context.Context) bool
}
