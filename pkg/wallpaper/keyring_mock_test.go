package wallpaper

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zalando/go-keyring"
)

// TestAPIKeysComeFromTheMockedCredentialStore guards TestMain: if keyring.MockInit were dropped,
// this would read the API keys saved in the real OS credential store of whoever runs the tests.
func TestAPIKeysComeFromTheMockedCredentialStore(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("no current user:", err)
	}
	c := &Config{userid: u.Uid}
	assert.Empty(t, c.GetWallhavenAPIKey(), "tests must not read the real wallhaven key")
	assert.Empty(t, c.GetPexelsAPIKey(), "tests must not read the real Pexels key")

	c.SetWallhavenAPIKey("test-key")
	// The mocked store lives as long as the test binary: without this a second run (-count=2)
	// found the key and failed the check above.
	t.Cleanup(func() { _ = keyring.Delete(WallhavenAPIKeyPrefKey, u.Uid) })
	assert.Equal(t, "test-key", c.GetWallhavenAPIKey())
}
