//go:build windows

package wallpaper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Windows tells the time this session began; it is in the past and no more than a few weeks ago
// (a session older than that would be a very patient person).
func TestTimeSinceLogon(t *testing.T) {
	since, ok := timeSinceLogon()
	if !ok {
		t.Skip("Windows does not report a logon time to this process (a service session or a restricted account)")
	}
	t.Logf("this session began %v ago", since.Round(time.Second))
	assert.Greater(t, since, time.Duration(0))
	assert.Less(t, since, 90*24*time.Hour)

	assert.True(t, startedSoonAfterLogon(since+time.Minute), "a window that is longer than the session counts it")
	assert.False(t, startedSoonAfterLogon(since-time.Second), "a window that is shorter does not")
}
