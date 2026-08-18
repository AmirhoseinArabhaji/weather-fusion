// Package photon implements geocoding.LocationSearch using Photon
// (komoot's OSM-based geocoder) — free, keyless, built for autocomplete.
package photon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
	"unicode"

	"github.com/amirhosein/weather-fusion/internal/geocoding"
)

// baseURL is Photon's public instance. No key needed — hardcoded rather than
// config-driven, same as open-meteo's geocoding endpoint.
const baseURL = "https://photon.komoot.io/api/"

// Geocoder is the Photon implementation of geocoding.LocationSearch.
type Geocoder struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new Photon geocoder.
func New(log *slog.Logger) geocoding.LocationSearch {
	return &Geocoder{
		client: &http.Client{Timeout: 5 * time.Second},
		log:    log.With("geocoder", "photon"),
	}
}

// searchResponse mirrors Photon's GeoJSON FeatureCollection response shape.
type searchResponse struct {
	Features []struct {
		Properties struct {
			OsmKey  string `json:"osm_key"`
			Name    string `json:"name"`
			City    string `json:"city"`
			State   string `json:"state"`
			Country string `json:"country"`
		} `json:"properties"`
		Geometry struct {
			// GeoJSON order: [longitude, latitude].
			Coordinates [2]float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

func (g *Geocoder) Search(ctx context.Context, query string, limit int) ([]geocoding.LocationMatch, error) {
	params := url.Values{}
	// Request more raw results than needed — OSM commonly returns several
	// boundary variants per place (see dedupe below), so asking for exactly
	// `limit` upstream would often leave fewer than `limit` distinct results.
	rawLimit := limit * 3
	if rawLimit > 20 {
		rawLimit = 20
	}
	params.Set("q", query)
	params.Set("limit", fmt.Sprintf("%d", rawLimit))
	// Without this, Photon returns each place's name in its own local script
	// (e.g. "تهران" for Tehran) — matches the rest of this app's English-only UI.
	params.Set("lang", "en")
	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("photon: build request: %w", err)
	}

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("photon: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("photon: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("photon: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var parsed searchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("photon: decode response: %w", err)
	}

	matches := make([]geocoding.LocationMatch, 0, len(parsed.Features))
	seen := make(map[string]bool, len(parsed.Features))
	for _, f := range parsed.Features {
		// Only populated places (osm_key "place": city/town/village/hamlet).
		// Photon also matches streets, POIs, and buildings by name (e.g. a
		// restaurant literally named "Shahrood") which otherwise out-rank the
		// actual city and surface their unrelated parent city/neighbourhood
		// instead — for a weather-by-location search those are just noise.
		if f.Properties.OsmKey != "place" {
			continue
		}
		// A place result is self-named; lang=en localizes it directly. Skip
		// (rather than show) the rare case with no English name at all.
		name := f.Properties.Name
		if name == "" || !isLatinScript(name) {
			continue
		}
		// OSM frequently returns several boundary variants (city point, county
		// centroid, admin polygon centroid) for the same place — indistinguishable
		// to a user picking from a dropdown, so keep only the first (best-ranked).
		dedupeKey := name + "|" + f.Properties.State + "|" + f.Properties.Country
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		matches = append(matches, geocoding.LocationMatch{
			Name:    name,
			Admin1:  f.Properties.State,
			Country: f.Properties.Country,
			Lat:     f.Geometry.Coordinates[1],
			Lon:     f.Geometry.Coordinates[0],
		})
		if len(matches) == limit {
			break
		}
	}
	return matches, nil
}

// isLatinScript reports whether s contains only Latin letters/marks and
// common punctuation/whitespace — used to reject a place name Photon left
// untranslated in a local script despite lang=en (no English name tagged).
func isLatinScript(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.In(r, unicode.Latin) {
			return false
		}
	}
	return true
}
