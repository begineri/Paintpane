package nga

import "encoding/json"

const (
	// ProviderName is the stable identifier used for configuration and registration.
	ProviderName = "NationalGalleryOfArt"

	// WebBaseURL is the museum's website.
	WebBaseURL = "https://www.nga.gov/"

	// LicenseURL is the museum's open access page (CC0).
	LicenseURL = "https://www.nga.gov/artworks/free-images-and-open-access"

	// IIIFPrefix is where NGA's IIIF image service lives; image_url values in the collection
	// file are "<IIIFPrefix><image uuid>".
	IIIFPrefix = "https://api.nga.gov/iiif/"

	// CollectionHighlights is the collection used when a saved collection key is unknown.
	CollectionHighlights = "nga_masterpieces"

	// maxIIIFEdge bounds the longest side of a requested image: a 4K screen's worth. The
	// originals can be tens of thousands of pixels and hundreds of megabytes.
	maxIIIFEdge = 3840

	// thumbnailEdge is the size of gallery previews.
	thumbnailEdge = 800

	// pageSize is how many works one FetchImages call returns.
	pageSize = 20
)

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
