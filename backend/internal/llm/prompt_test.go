package llm

import (
	"strings"
	"testing"
)

func TestFormatSummarizePrompt(t *testing.T) {
	req := SummarizeRequest{
		City:        "Padova",
		Temperature: 25.1,
		FeelsLike:   25.0,
		TempMin:     24.2,
		TempMax:     26.5,
		TempSpread:  2.3,
		TempStdDev:  0.9,
		Humidity:    50,
		WindSpeed:   3.2,
		PrecipProb:  0.0,
		Condition:   "clear",
		Confidence:  0.88,
		Providers: []ProviderReading{
			{Provider: "open-meteo", Temperature: 24.8, Condition: "clear", Humidity: 50},
			{Provider: "met.no", Temperature: 26.5, Condition: "clear", Humidity: 48},
		},
	}

	prompt := FormatSummarizePrompt(req)

	if !strings.Contains(prompt, "Location: Padova") {
		t.Errorf("prompt missing location: %s", prompt)
	}
	if !strings.Contains(prompt, "Consensus Temperature: 25.1°C") {
		t.Errorf("prompt missing temp: %s", prompt)
	}
	if !strings.Contains(prompt, "Range: 24.2°C to 26.5°C") {
		t.Errorf("prompt missing range: %s", prompt)
	}
	if !strings.Contains(prompt, "Spread: ±2.3°C") {
		t.Errorf("prompt missing spread: %s", prompt)
	}
	if !strings.Contains(prompt, "StdDev: 0.9°C") {
		t.Errorf("prompt missing stddev: %s", prompt)
	}
	if !strings.Contains(prompt, "open-meteo: 24.8°C") {
		t.Errorf("prompt missing provider info: %s", prompt)
	}
}
