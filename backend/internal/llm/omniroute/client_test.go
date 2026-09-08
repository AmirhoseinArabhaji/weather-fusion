package omniroute

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amirhosein/weather-fusion/internal/llm"
)

func TestResolveChatURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://omni.amirhosein.me/v1", "https://omni.amirhosein.me/v1/chat/completions"},
		{"https://omni.amirhosein.me/v1/", "https://omni.amirhosein.me/v1/chat/completions"},
		{"https://omni.amirhosein.me", "https://omni.amirhosein.me/v1/chat/completions"},
		{"https://omni.amirhosein.me/", "https://omni.amirhosein.me/v1/chat/completions"},
		{"https://omni.amirhosein.me/v1/chat/completions", "https://omni.amirhosein.me/v1/chat/completions"},
	}

	for _, tc := range tests {
		got := resolveChatURL(tc.input)
		if got != tc.expected {
			t.Errorf("resolveChatURL(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSummarizeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header with Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(req.Messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			t.Errorf("unexpected message roles: %+v", req.Messages)
		}

		resp := chatResponse{
			ID:    "chatcmpl-test",
			Model: "gpt-4o-mini",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      chatMessage `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: chatMessage{
						Role:    "assistant",
						Content: "Padova is currently clear with an average temperature of 25.1°C across 7 providers.",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     15,
				CompletionTokens: 10,
				TotalTokens:      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	client := New(server.URL, "test-key", "gpt-4o-mini", 256, logger)

	res, err := client.Summarize(context.Background(), llm.SummarizeRequest{
		City:        "Padova",
		Temperature: 25.1,
		FeelsLike:   25.0,
		TempMin:     24.2,
		TempMax:     26.5,
		TempSpread:  2.3,
		TempStdDev:  0.9,
		Condition:   "clear",
		Confidence:  0.88,
		Providers: []llm.ProviderReading{
			{Provider: "openmeteo", Temperature: 24.8, Condition: "clear", Humidity: 50},
			{Provider: "met.no", Temperature: 26.5, Condition: "clear", Humidity: 48},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.Content != "Padova is currently clear with an average temperature of 25.1°C across 7 providers." {
		t.Errorf("unexpected content: %s", res.Content)
	}
	if res.TokensUsed != 25 {
		t.Errorf("expected 25 tokens used, got %d", res.TokensUsed)
	}
	if res.Model != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %s", res.Model)
	}
}

func TestSummarizeApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid model identifier","type":"invalid_request_error"}}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	client := New(server.URL, "test-key", "invalid-model", 256, logger)

	_, err := client.Summarize(context.Background(), llm.SummarizeRequest{
		City: "Milan",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expectedSubstr := "Invalid model identifier"
	if !strings.Contains(err.Error(), expectedSubstr) {
		t.Errorf("expected error containing %q, got %q", expectedSubstr, err.Error())
	}
}
