// Package gemini implements LLMService using Google's Gemini API.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/amirhosein/weather-fusion/internal/llm"
)

const (
	defaultModel = "gemini-flash-latest"
	baseURL      = "https://generativelanguage.googleapis.com/v1beta"
)

// Client is the Gemini implementation of llm.LLMService.
type Client struct {
	apiKey    string
	model     string
	maxTokens int
	client    *http.Client
	log       *slog.Logger
}

// New creates a new Gemini LLM client.
func New(apiKey, model string, maxTokens int, log *slog.Logger) llm.LLMService {
	if model == "" {
		model = defaultModel
	}
	return &Client{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		client:    &http.Client{Timeout: 20 * time.Second},
		log:       log.With("llm", "gemini"),
	}
}

func (c *Client) ModelName() string { return c.model }

func (c *Client) Summarize(ctx context.Context, req llm.SummarizeRequest) (*llm.LLMResponse, error) {
	prompt := llm.FormatSummarizePrompt(req)
	return c.generate(ctx, llm.SummarizeSystemPrompt, prompt)
}

func (c *Client) Analyze(ctx context.Context, req llm.AnalyzeRequest) (*llm.LLMResponse, error) {
	data, err := json.Marshal(map[string]any{
		"city":       req.City,
		"historical": req.HistoricalData,
		"forecast":   req.ForecastData,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal analyze input: %w", err)
	}
	return c.generate(ctx, llm.AnalyzeSystemPrompt, string(data))
}

func (c *Client) IsAvailable(ctx context.Context) bool {
	return c.apiKey != ""
}

type generateRequest struct {
	Contents          []content        `json:"contents"`
	SystemInstruction *content         `json:"systemInstruction,omitempty"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type generateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []part `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		TotalTokenCount int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// apiError mirrors Gemini's error envelope: {"error":{"code":..,"message":..,"status":".."}}.
type apiError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// generate calls Gemini's generateContent endpoint, retrying on transient
// regularly enough that a single attempt isn't reliable.
func (c *Client) generate(ctx context.Context, systemPrompt, userPrompt string) (*llm.LLMResponse, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp, err := c.doGenerate(ctx, systemPrompt, userPrompt)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func isRetryable(err error) bool {
	return strings.Contains(err.Error(), "status 503") || strings.Contains(err.Error(), "status 429")
}

func (c *Client) doGenerate(ctx context.Context, systemPrompt, userPrompt string) (*llm.LLMResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("gemini: no API key configured")
	}

	reqBody := generateRequest{
		Contents: []content{
			{Role: "user", Parts: []part{{Text: userPrompt}}},
		},
		SystemInstruction: &content{Parts: []part{{Text: systemPrompt}}},
		GenerationConfig: generationConfig{
			MaxOutputTokens: c.maxTokens,
			Temperature:     0.4,
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, c.model, c.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("gemini: status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("gemini: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed generateResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response (finish reason: %s)", firstFinishReason(parsed))
	}

	return &llm.LLMResponse{
		Content:    parsed.Candidates[0].Content.Parts[0].Text,
		TokensUsed: parsed.UsageMetadata.TotalTokenCount,
		Model:      c.model,
		Confidence: 1.0,
	}, nil
}

func firstFinishReason(resp generateResponse) string {
	if len(resp.Candidates) == 0 {
		return "unknown"
	}
	return resp.Candidates[0].FinishReason
}
