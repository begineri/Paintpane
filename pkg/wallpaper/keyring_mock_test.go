package wallpaper

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, "test-key", c.GetWallhavenAPIKey())
}
