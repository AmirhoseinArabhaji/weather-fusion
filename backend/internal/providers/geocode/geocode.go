// Package geocode resolves a city name to coordinates via Open-Meteo's free,
// keyless geocoding endpoint — shared by providers whose own weather API
// only accepts lat/lon (open-meteo, met.no).
package geocode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/amirhosein/weather-fusion/internal/providers"
)

const searchURL = "https://geocoding-api.open-meteo.com/v1/search"

// Result is a single geocoding match.
type Result struct {
	Name    string
	Lat     float64
	Lon     float64
	Country string
}

type response struct {
	Results []struct {
		Name    string  `json:"name"`
		Lat     float64 `json:"latitude"`
		Lon     float64 `json:"longitude"`
		Country string  `json:"country"`
	} `json:"results"`
}

// Resolve looks up city by name, returning its best match.
func Resolve(ctx context.Context, client *http.Client, city string) (Result, error) {
	params := url.Values{}
	params.Set("name", city)
	params.Set("count", "1")
	params.Set("language", "en")
	params.Set("format", "json")
	reqURL := fmt.Sprintf("%s?%s", searchURL, params.Encode())

	body, err := providers.HTTPGet(ctx, client, reqURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("geocode %q: %w", city, err)
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Result{}, fmt.Errorf("decode geocode response: %w", err)
	}
	if len(parsed.Results) == 0 {
		return Result{}, fmt.Errorf("city not found: %s", city)
	}

	r := parsed.Results[0]
	return Result{Name: r.Name, Lat: r.Lat, Lon: r.Lon, Country: r.Country}, nil
}
