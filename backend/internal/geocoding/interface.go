// Package geocoding defines the LocationSearch interface for resolving a
// free-text place name into candidate coordinates — a distinct concern from
// providers.WeatherProvider, kept in its own package the same way.
package geocoding

import "context"

// LocationMatch is one candidate result for a place-name search.
type LocationMatch struct {
	Name    string  `json:"name"`    // place name, e.g. "Padova"
	Admin1  string  `json:"admin1"`  // state/region, e.g. "Veneto"
	Country string  `json:"country"` // e.g. "Italy"
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

// LocationSearch resolves free-text place names into candidate coordinates.
type LocationSearch interface {
	// Search returns up to limit candidate matches for query, ranked by the
	// underlying geocoder's own relevance ordering.
	Search(ctx context.Context, query string, limit int) ([]LocationMatch, error)
}
