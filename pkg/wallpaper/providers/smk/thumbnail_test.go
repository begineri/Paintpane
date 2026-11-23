package smk

import (
	"context"
	"testing"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
)

func TestFetchThumbnails_TDD(t *testing.T) {
	// The search result is replayed from testdata (recorded from api.smk.dk on 2026-11-18, cut
	// down to the fields FetchThumbnails reads), so the test runs offline and in CI.
	client := nettest.Replay(t, map[string]string{
		"api.smk.dk/api/v1/art/?object_number=KMS3418": "testdata/object-KMS3418.json",
	})
	p := NewProvider(nil, client)

	// Valid ID from SMK
	ids := []string{"KMS3418"}
	thumbnails, err := p.FetchThumbnails(context.Background(), ids)
	if err != nil {
		t.Fatalf("FetchThumbnails error: %v", err)
	}

	if len(thumbnails) != 1 {
		t.Fatalf("Expected 1 thumbnail, got %d", len(thumbnails))
	}

	if thumbnails[0].ID != "KMS3418" {
		t.Errorf("Expected ID 'KMS3418', got %s", thumbnails[0].ID)
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
		t.Errorf("Expected no error for invalid ID, got %v", err)
	}
}
