package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
)

// CropAnchor represents a user-selected focal point hint for Smart Fit cropping.
// Values 200-209 align with Command channel values for zero-conversion dispatch.
type CropAnchor int

const (
	AnchorAuto         CropAnchor = iota + 200 // 200 — No anchor (pipeline default)
	AnchorTopLeft                              // 201
	AnchorTopCenter                            // 202
	AnchorTopRight                             // 203
	AnchorMiddleLeft                           // 204
	AnchorMiddleCenter                         // 205
	AnchorMiddleRight                          // 206
	AnchorBottomLeft                           // 207
	AnchorBottomCenter                         // 208
	AnchorBottomRight                          // 209
)

// Center returns the normalized (0-1) coordinates for this anchor position.
func (a CropAnchor) Center() (float64, float64) {
	switch a {
	case AnchorTopLeft:
		return 0.2, 0.2
	case AnchorTopCenter:
		return 0.5, 0.2
	case AnchorTopRight:
		return 0.8, 0.2
	case AnchorMiddleLeft:
		return 0.2, 0.5
	case AnchorMiddleCenter:
		return 0.5, 0.5
	case AnchorMiddleRight:
		return 0.8, 0.5
	case AnchorBottomLeft:
		return 0.2, 0.8
	case AnchorBottomCenter:
		return 0.5, 0.8
	case AnchorBottomRight:
		return 0.8, 0.8
	default:
		return 0.5, 0.5 // Auto/unknown → center
	}
}

// String returns a human-readable name for the anchor position.
func (a CropAnchor) String() string {
	switch a {
	case AnchorAuto:
		return "Auto"
	case AnchorTopLeft:
		return "TopLeft"
	case AnchorTopCenter:
		return "TopCenter"
	case AnchorTopRight:
		return "TopRight"
	case AnchorMiddleLeft:
		return "MiddleLeft"
	case AnchorMiddleCenter:
		return "MiddleCenter"
	case AnchorMiddleRight:
		return "MiddleRight"
	case AnchorBottomLeft:
		return "BottomLeft"
	case AnchorBottomCenter:
		return "BottomCenter"
	case AnchorBottomRight:
		return "BottomRight"
	default:
		return "Unknown"
	}
}

// Tuning Option Enums
type FrameOverrideMode int

const (
	FrameOverrideInherit FrameOverrideMode = iota
	FrameOverrideForceOn
	FrameOverrideForceOff
)

// DisplayOverride is how one image is shown, chosen in Tune Image. Inherit follows the global
// display mode; the others are its three choices.
type DisplayOverride int

const (
	DisplayInherit DisplayOverride = iota
	DisplayFull                    // the whole image on the wall color
	DisplayFramed                  // the whole image in the gallery frame
	DisplayCrop                    // cropped to fill the screen
)

type MattingOverrideMode int

const (
	MattingOverrideInherit MattingOverrideMode = iota
	MattingOverrideOn
	MattingOverrideOff
)

type WallColorOverrideMode int

const (
	WallColorOverrideInherit WallColorOverrideMode = iota
	WallColorOverrideAlgorithmic
	WallColorOverrideNeutral
)

// TuningOptions encapsulates per-image, per-resolution overrides.
type TuningOptions struct {
	Anchor        CropAnchor            `json:",omitempty"`
	CropOffset    float64               `json:",omitempty"` // where the kept part sits along the cropped axis, 0-100 %; used when HasCropOffset
	HasCropOffset bool                  `json:",omitempty"` // the picture was dragged: CropOffset replaces the automatic crop (and any Anchor)
	Display       DisplayOverride       `json:",omitempty"` // this image's display mode; Inherit follows the global one
	FrameOverride FrameOverrideMode     `json:",omitempty"` // the older per-image frame switch, read only when Display is Inherit
	WallColor     WallColorOverrideMode `json:",omitempty"`
	Matting       MattingOverrideMode   `json:",omitempty"`
	FrameSize     float64               `json:",omitempty"` // 0 means inherit global default
	TightCrop     bool                  `json:",omitempty"` // Returns only framed art without wall background padding
}

type ContextKey string

const VirtualFramedKey ContextKey = "virtual_framed_result"
const ProviderIDKey ContextKey = "provider_id"

// Image represents a generic wallpaper image.
type Image struct {
	ID               string
	Path             string // URL to download the image
	ViewURL          string // URL to view the image in browser
	FilePath         string // Local path after download (optional/computed)
	Attribution      string // Photographer or Uploader name
	Title            string
	Artist           string
	Year             string
	Provider         string                   // Source provider name
	FileType         string                   // Content type (e.g., "image/jpeg")
	DownloadLocation string                   // URL to trigger download event (Unsplash requirement)
	ProcessingFlags  map[string]bool          // Flags indicating how the image was processed (e.g. "SmartFit", "FaceCrop")
	DerivativePaths  map[string]string        // Local file paths for different resolutions (e.g. "3440x1440" -> "/path/to/image.jpg")
	SourceQueryID    string                   // ID of the query that produced this image (for smart cache clearing)
	Width            int                      // Image Width (if available from source)
	Height           int                      // Image Height (if available from source)
	Tuning           map[string]TuningOptions `json:",omitempty"` // Per-resolution tuning overrides (key = "WxH", e.g. "3440x1440")
	IsFavorited      bool                     // Flag to protect image from cache pruning
	Seen             bool                     // Flag for pagination/history logic
	Purity           string                   `json:",omitempty"` // Source content rating (Wallhaven: "sfw", "sketchy" or "nsfw")
}

// GetTuning returns the tuning options for a specific resolution key.
// Returns an empty struct (where all fields evaluate to their Inherit/Auto zero values) if no tuning is set.
func (img Image) GetTuning(resKey string) TuningOptions {
	opts := TuningOptions{Anchor: AnchorAuto}
	if img.Tuning != nil {
		if t, ok := img.Tuning[resKey]; ok {
			if t.Anchor == 0 {
				t.Anchor = AnchorAuto
			}
			opts = t
		}
	}
	return opts
}

// GetAnchor is a convenience method that delegates to GetTuning.
func (img Image) GetAnchor(resKey string) CropAnchor {
	return img.GetTuning(resKey).Anchor
}

// MergeExistingMetadata copies locally-computed metadata from an existing store
// entry into this image. Used during backlog healing to preserve work already done
// (probed dimensions, incompatibility tags, crop anchors) while allowing the
// pipeline to regenerate DerivativePaths from scratch.
//
// Fields NOT merged (intentionally): DerivativePaths, FilePath, Seen, IsFavorited.
// These are either regenerated by the pipeline or managed by separate subsystems.
func (img *Image) MergeExistingMetadata(existing Image) {
	// Preserve probed dimensions
	if existing.Width > 0 {
		img.Width = existing.Width
	}
	if existing.Height > 0 {
		img.Height = existing.Height
	}

	// Merge processing flags (incompatibility tags, etc.)
	if img.ProcessingFlags == nil {
		img.ProcessingFlags = make(map[string]bool)
	}
	for k, v := range existing.ProcessingFlags {
		img.ProcessingFlags[k] = v
	}

	// Keep a known content rating if this fetch result did not carry one
	if img.Purity == "" {
		img.Purity = existing.Purity
	}

	// Preserve manual tuning options (don't overwrite user-set values)
	if len(existing.Tuning) > 0 {
		if img.Tuning == nil {
			img.Tuning = make(map[string]TuningOptions)
		}
		for k, v := range existing.Tuning {
			if _, exists := img.Tuning[k]; !exists {
				img.Tuning[k] = v
			}
		}
	}
}

// UnmarshalJSON implements custom JSON unmarshalling to migrate legacy CropAnchors to TuningOptions.
func (img *Image) UnmarshalJSON(data []byte) error {
	type Alias Image
	aux := &struct {
		CropAnchors map[string]CropAnchor `json:"CropAnchors,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(img),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.CropAnchors) > 0 {
		if img.Tuning == nil {
			img.Tuning = make(map[string]TuningOptions)
		}
		for resKey, anchor := range aux.CropAnchors {
			if anchor < 200 {
				anchor += 201
			}
			if _, exists := img.Tuning[resKey]; !exists {
				img.Tuning[resKey] = TuningOptions{Anchor: anchor}
			}
		}
	}

	return nil
}

// Favoriter defines the interface for providers that support favoriting images.
type Favoriter interface {
	IsFavorited(img Image) bool
	AddFavorite(img Image) error
	RemoveFavorite(img Image) error
	GetSourceQueryID() string
}

// Thumbnail represents a small version of an artwork for gallery previews.
type Thumbnail struct {
	ID          string
	URL         string
	FallbackURL string // Optional: used if URL returns an error (e.g., IIIF resize not supported)
	ViewURL     string
	Title       string
	Artist      string
	Year        string
}

// ThumbnailProvider defines the interface for providers that can resolve a batch of artwork IDs into thumbnail image URLs.
// This is used by the central gallery generator to create static HTML gallery previews efficiently.
type ThumbnailProvider interface {
	FetchThumbnails(ctx context.Context, ids []string) ([]Thumbnail, error)
}

// ProviderType enum defines the category of an image provider.
type ProviderType int

const (
	TypeCommunity ProviderType = iota
	TypePersonal
	TypeMuseum
)

// AttributionType defines how an image's attribution should be phrased (e.g., "By Photographer" vs "In Folder").
type AttributionType int

const (
	AttributionBy AttributionType = iota
	AttributionIn
)

// ImageProvider defines the interface for an image service.
type ImageProvider interface {
	// --- Identity & Core Display ---

	// ID returns a stable, non-localized internal identifier for the provider (e.g., "ArtInstituteChicago").
	// This is used internally for configuration keys, UI schema control names, and state tracking.
	// It must NEVER change, as changing it will break backwards compatibility for user settings.
	ID() string

	// Name returns the localized, long-form proper name of the provider (e.g., i18n.T("Art Institute of Chicago")).
	// This is typically used in the UI for long-form display elements where full context is preferred.
	Name() string

	// Title returns the short-form display title for the provider (e.g., "AIC").
	// This is used where UI space is limited, such as in the Windows System Tray menu,
	// or as the concise header for the provider's configuration section.
	Title() string

	// GetProviderIcon returns the provider's icon for UI display (e.g., tray menu, settings header).
	// It should return a high-quality, recognizable icon (e.g., []byte for embedded PNG/ICO).
	// Returns nil if no icon is available.
	GetProviderIcon() interface{}

	// --- Provider Metadata ---

	// Type returns the provider category (Online, Local, AI, Museum).
	Type() ProviderType

	// GetAttributionType returns the preferred phrasing for attribution (e.g. By or In).
	GetAttributionType() AttributionType

	// HomeURL returns the home URL of the provider service.
	HomeURL() string

	// SupportsUserQueries returns true if the provider allows users to add custom queries (e.g. search terms, URLs).
	// Returns false if the provider is curated-only (e.g. Museums, Daily Photo).
	SupportsUserQueries() bool

	// --- Core Capabilities ---

	// ParseURL checks if the given web URL is valid for this provider and returns the API URL.
	// It returns an error if the URL is invalid.
	ParseURL(webURL string) (string, error)

	// FetchImages fetches images from the provider using the given API URL and page number.
	FetchImages(ctx context.Context, apiURL string, page int) ([]Image, error)

	// EnrichImage fetches additional details for the image (e.g. attribution) if missing.
	EnrichImage(ctx context.Context, img Image) (Image, error)

	// --- UI Integration ---

	// CreateSettingsPanel creates the general configuration panel (e.g., API Keys).
	// Returns nil if the provider has no general settings.
	CreateSettingsPanel(sm setting.SettingsManager) *schema.PanelSchema

	// CreateQueryPanel creates the image query management panel.
	// Returns nil if the provider does not support custom queries.
	CreateQueryPanel(sm setting.SettingsManager, pendingUrl string) *schema.PanelSchema
}

// (SchemaProvider removed as ImageProvider now returns schemas directly)

// ResolutionAwareProvider is an optional interface for providers that can filter images based on screen resolution.
type ResolutionAwareProvider interface {
	ImageProvider
	// WithResolution returns a new API URL with resolution constraints added if they are missing.
	WithResolution(apiURL string, width, height int) string
}

// ScaledDownloadProvider is an optional interface for providers whose image server can send an
// image already scaled down (IIIF, Wikimedia thumbnails). While High-resolution originals is off,
// originals are capped at about 4K, and these providers are asked for the smaller copy instead of
// downloading the full image and shrinking it here.
type ScaledDownloadProvider interface {
	// ScaledDownloadURL returns where to download img at the size fit gives for its full size
	// (fit keeps the proportions). It returns false when the server cannot scale the image, the
	// full size is unknown, or the image is no larger than fit allows; the full image is then
	// downloaded and shrunk after download if needed.
	ScaledDownloadURL(ctx context.Context, img Image, fit func(w, h int) (int, int)) (string, bool)
}

// HeaderProvider is an optional interface for providers that need custom headers for image downloads.
type HeaderProvider interface {
	GetDownloadHeaders() map[string]string
}

// CustomClientProvider is an optional interface for providers that require a specialized http.Client
// for image downloads (e.g. for strict rate limiting or serialization).
type CustomClientProvider interface {
	GetClient() *http.Client
}

// DownloadAttemptsProvider is an optional interface for providers whose download client already
// retries on its own (SMK's sends another request when one stalls), so the downloader tries each
// image this many times instead of its default.
type DownloadAttemptsProvider interface {
	DownloadAttempts() int
}

// OutagePauseProvider is an optional interface for providers whose image server is known to stop
// answering at certain times (SMK's during the Danish night). OutagePause returns, for the moment
// an image fails to download, how long such an outage still lasts, or 0 outside it. Inside it the
// downloader gives up on the image after one attempt and pauses the source's downloads until the
// outage ends, instead of after several failed images.
type OutagePauseProvider interface {
	OutagePause(now time.Time) time.Duration
}

// Syncer is an optional interface for providers that support automated synchronization of managed queries.
type Syncer interface {
	Sync(ctx context.Context) error
}

// RemoteConfigSyncer is an optional interface for providers that fetch lightweight metadata
// or curated collections from a remote endpoint. This is called unconditionally during
// the nightly refresh to keep the application's internal catalog up to date.
type RemoteConfigSyncer interface {
	SyncRemoteConfig() error
}

// GalleryProvider is an optional interface for providers that can generate
// static HTML virtual gallery walls for their curated collections.
type GalleryProvider interface {
	GenerateGalleries(ctx context.Context, destDir string) error
}

// PageCursorProvider is an optional interface for providers whose page N > 1 can only be fetched
// with a continuation cursor that page N-1 returned (Wikimedia Commons' "continue" parameters).
// The app saves the cursor of each query's next page with its page number and hands it back after
// a restart, so the query resumes where it was instead of starting over.
type PageCursorProvider interface {
	// PageCursor returns the cursor for fetching page of query, if the provider has one.
	PageCursor(query string, page int) (string, bool)
	// SetPageCursor gives back a cursor that PageCursor returned in an earlier run.
	SetPageCursor(query string, page int, cursor string)
}

// ThrottledProvider is an optional interface for providers that can signal
// they are currently in a cooldown state (e.g., due to a 429 error).
type ThrottledProvider interface {
	IsThrottled() bool
}

// PacedProvider is an optional interface for providers that require specific rate limiting gaps
// between API requests and image processing (downloads/enrichments).
type PacedProvider interface {
	GetAPIPacing() time.Duration
	GetProcessPacing() time.Duration
}
