// Package omniroute implements LLMService using an OpenAI-compatible OmniRoute gateway.
package omniroute

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

const defaultModel = "gpt-4o-mini"

// Client is the OpenAI-compatible OmniRoute implementation of llm.LLMService.
type Client struct {
	baseURL   string
	apiKey    string
	model     string
	maxTokens int
	client    *http.Client
	log       *slog.Logger
}

// New creates a new OmniRoute LLM client.
func New(baseURL, apiKey, model string, maxTokens int, log *slog.Logger) llm.LLMService {
	if model == "" {
		model = defaultModel
	}
	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		client:    &http.Client{Timeout: 20 * time.Second},
		log:       log.With("llm", "omniroute"),
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
		return nil, fmt.Errorf("omniroute: marshal analyze input: %w", err)
	}
	return c.generate(ctx, llm.AnalyzeSystemPrompt, string(data))
}

func (c *Client) IsAvailable(ctx context.Context) bool {
	return c.baseURL != ""
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

func resolveChatURL(rawBaseURL string) string {
	base := strings.TrimRight(rawBaseURL, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

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
	msg := err.Error()
	return strings.Contains(msg, "status 500") ||
		strings.Contains(msg, "status 502") ||
		strings.Contains(msg, "status 503") ||
		strings.Contains(msg, "status 504") ||
		strings.Contains(msg, "status 429")
}

func (c *Client) doGenerate(ctx context.Context, systemPrompt, userPrompt string) (*llm.LLMResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("omniroute: no base URL configured")
	}

	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   c.maxTokens,
		Temperature: 0.4,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("omniroute: marshal request: %w", err)
	}

	reqURL := resolveChatURL(c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("omniroute: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("omniroute: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("omniroute: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errEnvelope apiError
		if json.Unmarshal(respBody, &errEnvelope) == nil && errEnvelope.Error.Message != "" {
			return nil, fmt.Errorf("omniroute: status %d: %s", resp.StatusCode, errEnvelope.Error.Message)
		}
		return nil, fmt.Errorf("omniroute: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("omniroute: decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("omniroute: empty response choices")
	}

	modelName := parsed.Model
	if modelName == "" {
		modelName = c.model
	}

	return &llm.LLMResponse{
		Content:    strings.TrimSpace(parsed.Choices[0].Message.Content),
		TokensUsed: parsed.Usage.TotalTokens,
		Model:      modelName,
		Confidence: 1.0,
	}, nil
}
