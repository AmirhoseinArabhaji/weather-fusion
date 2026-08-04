// Package openai implements LLMService using the OpenAI Chat Completions API.
package openai

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/amirhosein/weather-fusion/internal/llm"
)

const defaultModel = "gpt-4o"

// Client is the OpenAI implementation of llm.LLMService.
type Client struct {
	apiKey    string
	model     string
	maxTokens int
	log       *slog.Logger
}

// New creates a new OpenAI LLM client.
func New(apiKey, model string, maxTokens int, log *slog.Logger) llm.LLMService {
	if model == "" {
		model = defaultModel
	}
	return &Client{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		log:       log.With("llm", "openai"),
	}
}

func (c *Client) ModelName() string { return c.model }

func (c *Client) Summarize(ctx context.Context, req llm.SummarizeRequest) (*llm.LLMResponse, error) {
	// TODO: POST https://api.openai.com/v1/chat/completions
	// with a system prompt asking for a weather summary.
	c.log.DebugContext(ctx, "summarize (stub)", "city", req.City)

	content := fmt.Sprintf(
		"Current conditions in %s: %.1f°C, %s. %s",
		req.City, req.Temperature, req.Condition, req.Description,
	)
	return &llm.LLMResponse{
		Content:    content,
		TokensUsed: 0,
		Model:      c.model,
		Confidence: 1.0,
	}, nil
}

func (c *Client) Analyze(ctx context.Context, req llm.AnalyzeRequest) (*llm.LLMResponse, error) {
	// TODO: build a structured prompt from historical + forecast data and call the API.
	c.log.DebugContext(ctx, "analyze (stub)", "city", req.City)
	return &llm.LLMResponse{
		Content:    fmt.Sprintf("Trend analysis for %s: data looks nominal (stub).", req.City),
		TokensUsed: 0,
		Model:      c.model,
		Confidence: 0.8,
	}, nil
}

func (c *Client) IsAvailable(ctx context.Context) bool {
	// TODO: make a lightweight models list request to verify the API key.
	return c.apiKey != ""
}
