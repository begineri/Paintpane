package wallpaper

import (
	"bytes"
	"context"
	"errors"
	stdlog "log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// A2 §12.9 #5: a job for a blocked image is skipped, which is expected, so it
// is not logged as "Job failed" and leaves the image's files alone.
func TestProcessImageJob_BlockedImageIsASkipNotAFailure(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.addLibraryImage(t, "blocked_one", 1000, 800)
	img, _ := env.store.GetByID("blocked_one")
	env.wp.cfg.AddToAvoidSet(img.ID)

	wasDebug := utilLog.IsDebugEnabled()
	utilLog.SetDebugEnabled(true)
	defer utilLog.SetDebugEnabled(wasDebug)
	var out bytes.Buffer
	prev := stdlog.Writer()
	stdlog.SetOutput(&out)
	_, err := env.wp.ProcessImageJob(context.Background(), DownloadJob{Ctx: context.Background(), Image: img, Provider: &MockProvider{}})
	(&Pipeline{}).logPipelineError(err)
	stdlog.SetOutput(prev)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInAvoidSet))
	assert.NotContains(t, out.String(), "Job failed")
	assert.NotContains(t, out.String(), "Pipeline Error")
	assert.Contains(t, out.String(), "skipped a blocked image")

	master, err := env.wp.masterPathFor(img)
	require.NoError(t, err)
	assert.FileExists(t, master, "skipping does not clean up anything")
}
