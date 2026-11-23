package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMSIXVersion(t *testing.T) {
	v, err := msixVersion("v1.2.0\n")
	require.NoError(t, err)
	assert.Equal(t, "1.2.0.0", v)
	for _, bad := range []string{"v1.2", "1.2.3.4", "v1.x.0", "v1.2.70000"} {
		_, err := msixVersion(bad)
		assert.Error(t, err, bad)
	}
}

func TestSetVersionChangesIdentityOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AppxManifest.xml")
	in := `<Package>
  <Identity Name="A" Publisher="CN=B" Version="0.0.0.0" ProcessorArchitecture="x64" />
  <TargetDeviceFamily Name="Windows.Desktop" MinVersion="10.0.18362.0" MaxVersionTested="10.0.26100.0" />
</Package>`
	require.NoError(t, os.WriteFile(path, []byte(in), 0o644))
	require.NoError(t, setVersion(path, "1.2.0.0"))
	out, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(out), `Publisher="CN=B" Version="1.2.0.0"`)
	assert.Contains(t, string(out), `MinVersion="10.0.18362.0" MaxVersionTested="10.0.26100.0"`)
}
