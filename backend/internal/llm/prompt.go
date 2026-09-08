package llm

import (
	"fmt"
	"strings"
)

// SummarizeSystemPrompt keeps the model firmly in "interpreter" mode
const SummarizeSystemPrompt = `You are an expert weather consensus data interpreter, not a forecaster. ` +
	`Explain the current weather consensus and cross-provider agreement in concise, natural language. ` +
	`Clearly convey the average temperature, the observed range/spread across providers, and whether ` +
	`the reporting providers strongly agree or show notable variance. Do not invent data or predict beyond what is provided. ` +
	`Keep your summary to 2-3 sentences.`

// AnalyzeSystemPrompt is used for historical vs forecast trend analysis.
const AnalyzeSystemPrompt = `You are a weather data interpreter, not a forecaster. ` +
	`Given historical and forecast weather data, describe the notable trend in plain language. ` +
	`Do not invent data. Keep it to 2-3 sentences.`

// FormatSummarizePrompt builds a structured prompt for the LLM summarizer.
func FormatSummarizePrompt(req SummarizeRequest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Location: %s\n", req.City))
	if req.TempMin != 0 || req.TempMax != 0 {
		sb.WriteString(fmt.Sprintf("Consensus Temperature: %.1f°C (Range: %.1f°C to %.1f°C, Spread: ±%.1f°C, StdDev: %.1f°C)\n",
			req.Temperature, req.TempMin, req.TempMax, req.TempSpread, req.TempStdDev))
	} else {
		sb.WriteString(fmt.Sprintf("Consensus Temperature: %.1f°C (StdDev: %.1f°C)\n", req.Temperature, req.TempStdDev))
	}
	if req.FeelsLike != 0 {
		sb.WriteString(fmt.Sprintf("Feels Like: %.1f°C\n", req.FeelsLike))
	}
	sb.WriteString(fmt.Sprintf("Condition: %s\n", req.Condition))
	sb.WriteString(fmt.Sprintf("Humidity: %.0f%%\n", req.Humidity))
	sb.WriteString(fmt.Sprintf("Wind Speed: %.1f m/s\n", req.WindSpeed))
	sb.WriteString(fmt.Sprintf("Precipitation Probability (Rain Risk): %.0f%%\n", req.PrecipProb*100))
	sb.WriteString(fmt.Sprintf("Confidence Score: %.0f%%\n", req.Confidence*100))

	if len(req.Providers) > 0 {
		sb.WriteString("Provider Readings:\n")
		for _, p := range req.Providers {
			sb.WriteString(fmt.Sprintf("- %s: %.1f°C, %s, humidity %d%%\n", p.Provider, p.Temperature, p.Condition, p.Humidity))
		}
	} else if req.Description != "" {
		sb.WriteString(fmt.Sprintf("Details: %s\n", req.Description))
	}
	return sb.String()
}
