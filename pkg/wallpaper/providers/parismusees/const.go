package parismusees

const (
	// ProviderName is the stable identifier used for configuration and registration.
	ProviderName = "ParisMusees"

	// WebBaseURL is the collections website.
	WebBaseURL = "https://www.parismuseescollections.paris.fr/en"

	// LicenseURL is the open content (CC0) announcement.
	LicenseURL = "https://parismusees.paris.fr/en/actualite/open-content-150000-works-from-the-museum-collections-of-the-city-of-paris-freely"

	// ImageHost is where the original image files live; image_url values in the collection file
	// are full addresses under it.
	ImageHost = "https://www.parismuseescollections.paris.fr/sites/default/files/atoms/images/"

	// CollectionHighlights is the collection used when a saved collection key is unknown.
	CollectionHighlights = "pm_masterpieces"

	// pageSize is how many works one FetchImages call returns. The images are 2-6 MB each, so the
	// pages are kept small.
	pageSize = 10
)
