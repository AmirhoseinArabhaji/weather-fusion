package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 10 * time.Second

// NewHTTPClient returns the http.Client every provider constructs itself with.
func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: defaultTimeout}
}

// StatusError is returned by HTTPGet for a non-2xx response, carrying the
// response body so callers that need to parse a provider-specific error
// shape out of it (e.g. weatherapi) still can.
type StatusError struct {
	Code int
	Body []byte
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status %d: %s", e.Code, string(e.Body))
}

// HTTPGet issues a GET request and returns the response body. Non-2xx
// responses return a *StatusError alongside the body.
func HTTPGet(ctx context.Context, client *http.Client, reqURL string, headers map[string]string) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, &StatusError{Code: resp.StatusCode, Body: body}
	}

	return body, nil
}

// BoolPtr returns a pointer to b, for building the optional *bool fields
// (e.g. WeatherObservation.IsDay) a provider only sometimes reports.
func BoolPtr(b bool) *bool { return &b }
