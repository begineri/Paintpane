package wallpaper

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// derivativeCleanupWait limits how long a rebuild waits for stale derivatives to be deleted.
const derivativeCleanupWait = 30 * time.Second

// rebuildMissingDerivatives re-renders derivatives from local masters after a settings change.
//
// Backlog healing in fetchFromProvider only sees images on the page being fetched, and pagination
// has moved on by then, so without this the existing library stays invisible until its page comes
// round again. Jobs carry no provider, so this uses no network. A newer rebuild supersedes this one.
func (wp *Plugin) rebuildMissingDerivatives() {
	wp.downloadMutex.RLock()
	submitter := wp.jobSubmitter
	wp.downloadMutex.RUnlock()
	if submitter == nil {
		return
	}

	ctx := wp.startRebuildContext()

	// Sync deletes stale derivatives asynchronously. Starting early could lose a rebuilt file.
	if wp.fm != nil {
		waitCtx, cancel := context.WithTimeout(ctx, derivativeCleanupWait)
		finished := wp.fm.WaitForDerivativeDeletes(waitCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if !finished {
			log.Printf("Derivative rebuild: timed out waiting for cleanup. Continuing.")
		}
	}

	// Images whose files already exist come back first. They need no decoding, so they must not
	// wait in a queue shared with renders and downloads.
	var pending []provider.Image
	reattached := 0
	store, canReattach := wp.store.(*ImageStore)
	resolutions := wp.getResolutionsForDerivatives()
	for _, img := range wp.store.List() {
		if ctx.Err() != nil {
			return
		}
		if !wp.needsDerivativeRebuild(img) {
			continue
		}
		if canReattach {
			if attached, ok := wp.reattachDerivatives(img, resolutions); ok {
				if store.isStaleDisplayMode(attached) {
					return // The mode changed again; a newer rebuild takes over.
				}
				store.replace(attached)
				reattached++
				continue
			}
		}
		pending = append(pending, img)
	}

	for _, img := range pending {
		if !submitter.Submit(ctx, DownloadJob{Ctx: ctx, Image: imageForRebuild(img)}) {
			return
		}
	}
	log.Debugf("Derivative rebuild: reattached %d images, queued %d.", reattached, len(pending))
}

// reattachDerivatives points an image at its existing derivatives for the current settings. It
// reports false unless every monitor resolution has one, leaving the image for a pipeline job.
func (wp *Plugin) reattachDerivatives(img provider.Image, resolutions []Resolution) (provider.Image, bool) {
	derivativeDir := wp.getBaseDerivativeDir()
	if derivativeDir == "" {
		return img, false
	}
	masterPath, err := wp.masterPathFor(img)
	if err != nil {
		return img, false
	}
	paths, allExist := wp.checkExistingDerivatives(img.ID, filepath.Ext(masterPath), derivativeDir, resolutions)
	if !allExist || len(paths) == 0 {
		return img, false
	}

	attached := imageForRebuild(img)
	stampProcessingFlags(&attached, wp.targetProcessingFlags())
	wp.recordFramedFlagsForReuse(attached, resolutions, paths)
	applyDerivativePaths(&attached, wp.ensurePrimaryPath(paths))
	return attached, true
}

// needsDerivativeRebuild reports whether an image can and should be re-rendered from its local master.
func (wp *Plugin) needsDerivativeRebuild(img provider.Image) bool {
	if wp.getBaseDerivativeDir() == "" {
		// Smart Fit disabled: images use the master directly under a "primary" path.
		if len(img.DerivativePaths) > 0 {
			return false
		}
	} else if wp.allMonitorDerivativesExist(img) {
		return false
	}

	masterPath, err := wp.masterPathFor(img)
	if err != nil {
		return false
	}
	_, err = os.Stat(masterPath)
	return err == nil
}

// imageForRebuild copies an image for a job. store.List copies images shallowly and jobs write to
// ProcessingFlags, so sharing the maps would race with the store's background save.
func imageForRebuild(img provider.Image) provider.Image {
	clone := img
	clone.ProcessingFlags = nil
	clone.Tuning = nil
	clone.DerivativePaths = nil
	clone.MergeExistingMetadata(img)
	return clone
}

// startRebuildContext cancels any rebuild in progress and returns the context for a new one.
func (wp *Plugin) startRebuildContext() context.Context {
	wp.rebuildMu.Lock()
	defer wp.rebuildMu.Unlock()
	if wp.rebuildCancel != nil {
		wp.rebuildCancel()
	}
	ctx, cancel := context.WithCancel(wp.ctx)
	wp.rebuildCancel = cancel
	return ctx
}
