package cleveland

import (
	"context"
	"testing"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
)

func TestFetchThumbnails_TDD(t *testing.T) {
	// The artwork record is replayed from testdata (recorded from openaccess-api.clevelandart.org
	// on 2026-11-18, cut down to the fields apiArtwork reads), so the test never touches the real API.
	client := nettest.Replay(t, map[string]string{
		"openaccess-api.clevelandart.org/api/artworks/141639": "testdata/artwork-141639.json",
	})
	p := NewProvider(nil, client)

	// Valid ID from CMA
	ids := []string{"141639"}
	thumbnails, err := p.FetchThumbnails(context.Background(), ids)
	if err != nil {
		t.Fatalf("FetchThumbnails error: %v", err)
	}

	if len(thumbnails) != 1 {
		t.Fatalf("Expected 1 thumbnail, got %d", len(thumbnails))
	}

	if thumbnails[0].ID != "141639" {
		t.Errorf("Expected ID '141639', got %s", thumbnails[0].ID)
	}
	if thumbnails[0].URL == "" {
		t.Error("Expected a valid URL, got empty string")
	}
	if thumbnails[0].ViewURL == "" {
		t.Errorf("Expected ViewURL to be populated, got empty string")
	}

	// Invalid ID
	_, err = p.FetchThumbnails(context.Background(), []string{"invalid"})
	if err != nil {
		t.Errorf("Expected no error for invalid ID (should skip), got %v", err)
	}
}
