package yale

import (
	"encoding/json"
	"time"
)

const (
	// ProviderName is the stable identifier used for configuration and registration.
	ProviderName = "YaleUniversityArtCollections"

	// WebBaseURL is the Yale University Art Gallery's website.
	WebBaseURL = "https://artgallery.yale.edu/"

	// LicenseURL is the Yale University Art Gallery page on using its images (open access to public-domain works).
	LicenseURL = "https://artgallery.yale.edu/using-collection/using-images/rights-and-reproductions"

	// IIIFPrefix is where Yale's IIIF image service lives; image_url values in the collection
	// file are "<IIIFPrefix><institution>:<image uuid>".
	IIIFPrefix = "https://images.collections.yale.edu/iiif/2/"

	// CollectionHighlights is the collection used when a saved collection key is unknown.
	CollectionHighlights = "yale_masterpieces"

	// maxIIIFEdge bounds the longest side of a requested image: a 4K screen's worth.
	maxIIIFEdge = 3840

	// thumbnailEdge is the size of gallery previews.
	thumbnailEdge = 800

	// pageSize is how many works one FetchImages call returns.
	pageSize = 20

	// imageHeaderTimeout is how long a request to Yale's image server waits for it to start
	// answering. The server renders a scaled copy on request; for some works that took 18-60
	// seconds or ended in a 504 (measured 2026-11-12), while the stored full-size image
	// ("full/max") came within a second. A scaled request that does not answer in this time is
	// asked again as full/max (maxFallback).
	imageHeaderTimeout = 20 * time.Second

	// imageRequestTimeout bounds a whole image download from Yale.
	imageRequestTimeout = 3 * time.Minute
)

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
