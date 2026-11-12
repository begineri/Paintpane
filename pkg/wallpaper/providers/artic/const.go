package artic

import (
	"regexp"
)

const (
	// ProviderName is the unique identifier for config
	ProviderName = "ArtInstituteChicago"

	// ProviderTitle is the user-facing name
	ProviderTitle = "AIC"

	// APIBaseURL is the base for all API calls
	APIBaseURL = "https://api.artic.edu/api/v1"

	// ClientUserAgent and ClientIdentity identify this application to AIC.
	ClientUserAgent = "Paintpane"
	ClientIdentity  = "Paintpane (https://github.com/begineri/Paintpane)"

	// FullImageWidth requests the most detail AIC's image server offers: it caps the long
	// edge at 3000 and never upscales. AIC's docs only list widths up to 1686, which looks
	// soft on high-resolution screens, but the server does accept this size.
	FullImageWidth = 3000
	// ThumbnailWidth is the width AIC recommends for general use, and caches.
	ThumbnailWidth = 843

	// Collection IDs (Internal Query Strings)
	CollectionHighlights = "artic_highlights"
	CollectionImpression = "artic_impressionism"
	CollectionModern     = "artic_modern"
	CollectionAsia       = "artic_asia"
)

// ObjectURLRegex matches direct AIC artwork URLs
// e.g. https://www.artic.edu/artworks/27992/a-sunday-on-la-grande-jatte-1884
var ObjectURLRegex = regexp.MustCompile(`^https://www\.artic\.edu/artworks/(\d+)`)
