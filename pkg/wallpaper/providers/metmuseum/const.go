package metmuseum

import (
	"regexp"
	"time"
)

const (
	// ProviderName is the unique identifier for config
	ProviderName = "MetMuseum"

	// ProviderTitle is the user-facing name
	ProviderTitle = "The Met"

	// ClientUserAgent identifies this application to the Met's API and image servers.
	ClientUserAgent = "Paintpane (https://github.com/begineri/Paintpane)"

	// APIBaseURL is the base for all API calls
	APIBaseURL = "https://collectionapi.metmuseum.org/public/collection/v1"

	// SearchURL is the paged search that replaced v1/search (retired 2026-10-01, now 410 Gone).
	SearchURL = "https://collectionapi.metmuseum.org/public/collection/v1.1/search"
	// searchPageSize is the most IDs v1.1 sends per page.
	searchPageSize = 500
	// maxSearchIDs caps how many IDs one collection takes from a search (v1.1 serves at most the
	// first 10,000 results). The largest collection, European Paintings, has about 2,600.
	maxSearchIDs = 5000

	// Department IDs
	DeptEuropeanPaintings = 11
	DeptModernArt         = 21
	DeptAsianArt          = 6
	DeptEgyptianArt       = 10

	// Collection IDs (Internal Query Strings)
	CollectionSpiceMelange = "metmuseum_best_of" // legacy name; the shipped key is now metmuseum_best_of
	CollectionEuropean     = "european_masterpieces"
	CollectionModern       = "modern_art"
	CollectionAsian        = "asian_art"
	CollectionEgyptian     = "egyptian_art"
	CollectionAmerican     = "american_art"
)

// searchPagePause is the wait between two pages of one search. A variable so tests can shorten it.
var searchPagePause = 250 * time.Millisecond

// Regex for direct URL pasting
// e.g. https://www.metmuseum.org/art/collection/search/437261
var ObjectURLRegex = regexp.MustCompile(`^https://www\.metmuseum\.org/art/collection/search/(\d+)`)
