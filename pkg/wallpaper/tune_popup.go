package wallpaper

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

const tuningUIThrottle = 4000 * time.Millisecond

// anchorLabels maps each grid position to its display label.
var anchorLabels = [9]string{"↖", "↑", "↗", "←", "●", "→", "↙", "↓", "↘"}

// anchorValues maps each grid index to its provider.CropAnchor value.
var anchorValues = [9]provider.CropAnchor{
	provider.AnchorTopLeft, provider.AnchorTopCenter, provider.AnchorTopRight,
	provider.AnchorMiddleLeft, provider.AnchorMiddleCenter, provider.AnchorMiddleRight,
	provider.AnchorBottomLeft, provider.AnchorBottomCenter, provider.AnchorBottomRight,
}

// showTuneImagePopup gathers state and delegates popup creation to the PluginManager.
// The outer ring (ui/ui.go) owns window lifecycle and OpenGL error recovery.
func (wp *Plugin) showTuneImagePopup(monitorID int) {
	log.Debugf("showTuneImagePopup: Preparing tune image popup for monitor %d", monitorID)

	// Determine current anchor from monitor state
	wp.monMu.RLock()
	mc, ok := wp.Monitors[monitorID]
	wp.monMu.RUnlock()
	if !ok {
		log.Printf("[WARN] showTuneImagePopup: Monitor %d not found", monitorID)
		return
	}

	mc.mu.RLock()
	resKey := fmt.Sprintf("%dx%d", mc.Monitor.Rect.Dx(), mc.Monitor.Rect.Dy())
	currentOpts := mc.State.CurrentImage.GetTuning(resKey)
	mc.mu.RUnlock()

	if wp.manager == nil {
		log.Printf("[WARN] showTuneImagePopup: No plugin manager available")
		return
	}

	mc.mu.RLock()
	effectiveOpts := effectiveTuning(currentOpts, mc.State.CurrentImage, resKey, wp.cfg)
	mc.mu.RUnlock()
	// The preview shows the desktop result, drawn by the same processors as the wallpaper.
	mc.mu.RLock()
	providerID := mc.State.CurrentImage.Provider
	mc.mu.RUnlock()
	preview := ui.TunePreview{
		ScreenW: mc.Monitor.Rect.Dx(),
		ScreenH: mc.Monitor.Rect.Dy(),
		Global:  displayOverride(wp.cfg.GetArtworkDisplayMode()),
		Render: func(src image.Image, w, h int, opts provider.TuningOptions) image.Image {
			ctx := context.WithValue(context.Background(), provider.ProviderIDKey, providerID)
			out, err := wp.imgProcessor.FitImage(ctx, src, w, h, opts)
			if err != nil || out == nil {
				log.Debugf("showTuneImagePopup: preview render failed: %v", err)
				return nil
			}
			return out
		},
		AutoPosition: func(src image.Image, w, h int) (float64, bool) {
			return wp.autoCropPosition(mc, src, w, h)
		},
	}

	// Send CmdTuningStart to pause the controller
	mc.Commands <- CmdTuningStart

	// Kept for the interface; the window now reads the image's own display mode (preview.Global and
	// effectiveOpts.Display) and crops whenever that is Smart crop.
	lockAnchors := wp.cfg.GetArtworkDisplayMode() != ArtworkDisplaySmartCrop

	wp.manager.ShowTuneImagePopup(monitorID, wp.tunePreviewPath(mc), currentOpts, effectiveOpts, anchorLabels, anchorValues, preview, lockAnchors,
		func(opts provider.TuningOptions, onDone func()) {
			log.Debugf("showTuneImagePopup: User selected options %v for monitor %d", opts, monitorID)
			go func() {
				// Hook into OnWallpaperChanged to detect when reprocessing completes
				done := make(chan struct{}, 1)
				wp.monMu.RLock()
				mc, ok := wp.Monitors[monitorID]
				wp.monMu.RUnlock()

				if ok {
					origCallback := mc.OnWallpaperChanged
					mc.OnWallpaperChanged = func(img provider.Image, mID int) {
						// Restore original callback first
						mc.OnWallpaperChanged = origCallback
						// Signal done
						select {
						case done <- struct{}{}:
						default:
						}
						// Call the original callback
						if origCallback != nil {
							origCallback(img, mID)
						}
					}
				}

				start := time.Now()
				wp.SetTuningOptions(monitorID, opts)

				// Wait for completion (with timeout to prevent hanging)
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					log.Printf("[WARN] Tuning reprocessing timed out for monitor %d", monitorID)
				}

				// Enforce a minimum delay to prevent macOS wallpaper daemon choke and UI spam
				// macOS NSWorkspace can drop or queue transitions if called too rapidly.
				if runtime.GOOS == "darwin" {
					elapsed := time.Since(start)
					if elapsed < tuningUIThrottle {
						time.Sleep(tuningUIThrottle - elapsed)
					}
				}

				if onDone != nil {
					onDone()
				}
			}()
		},
		func() {
			// Resume automatic rotation when popup closes
			mc.Commands <- CmdTuningEnd
		},
	)
}

// effectiveTuning resolves the "inherit" values of a picture's tuning to what the wallpaper really
// uses: its display mode (its own choice, else the global mode; the older frame switch is folded
// into it), frame size, paper mat and wall colour from the settings. Tune Image starts from it, and
// the monitor actor uses it to recognise a render it already made.
func effectiveTuning(opts provider.TuningOptions, img provider.Image, resKey string, cfg *Config) provider.TuningOptions {
	eff := opts
	if eff.IsAutoCrop() {
		eff.Anchor = provider.AnchorAuto
	}
	if cfg == nil {
		// No settings to read the global mode from: the last render says whether it was framed.
		if eff.Display == provider.DisplayInherit && (eff.FrameOverride == provider.FrameOverrideForceOn ||
			(eff.FrameOverride == provider.FrameOverrideInherit && img.ProcessingFlags["VirtualFramed:"+resKey])) {
			eff.Display = provider.DisplayFramed
		}
		eff.FrameOverride = provider.FrameOverrideInherit
		return eff
	}
	eff.Display = displayOverride(displayModeFor(cfg.GetArtworkDisplayMode(), opts))
	eff.FrameOverride = provider.FrameOverrideInherit
	if eff.FrameSize == 0 {
		eff.FrameSize = cfg.VirtualFrameSize
	}
	if eff.Matting == provider.MattingOverrideInherit {
		if cfg.VirtualPaperMatting {
			eff.Matting = provider.MattingOverrideOn
		} else {
			eff.Matting = provider.MattingOverrideOff
		}
	}
	if eff.WallColor == provider.WallColorOverrideInherit {
		if cfg.VirtualWallColor == WallAlgorithmic {
			eff.WallColor = provider.WallColorOverrideAlgorithmic
		} else {
			eff.WallColor = provider.WallColorOverrideNeutral
		}
	}
	return eff
}

// autoCropPosition says where the automatic crop keeps src along the cropped axis, in percent, so
// Tune Image starts a drag from where the picture really is and draws Reset's result. It is read
// from the wallpaper on screen when that is the automatic crop: the analysis behind it (faces,
// entropy) can choose a different part of the small copy src than of the full original, so running
// it on src would show a crop the desktop does not have. Only when the screen shows something else
// (a dragged position, a frame, a file that cannot be read) does it fall back to that analysis.
func (wp *Plugin) autoCropPosition(mc *MonitorController, src image.Image, w, h int) (float64, bool) {
	mc.mu.RLock()
	img := mc.State.CurrentImage
	resKey := fmt.Sprintf("%dx%d", mc.Monitor.Rect.Dx(), mc.Monitor.Rect.Dy())
	mc.mu.RUnlock()
	tuning := img.GetTuning(resKey)
	// The picture can be cropped by its own choice in Tune Image under any global mode.
	cropped := displayModeFor(wp.cfg.GetArtworkDisplayMode(), tuning) == ArtworkDisplaySmartCrop
	if cropped && tuning.IsAutoCrop() && !img.ProcessingFlags["VirtualFramed:"+resKey] && img.FilePath != "" {
		if shown, err := imaging.Open(img.FilePath); err == nil {
			if p, ok := matchCropPercent(src, shown, w, h); ok {
				return p, true
			}
			log.Debugf("autoCropPosition: the wallpaper %s does not match the picture; using the analysis", img.FilePath)
		}
	}
	if ac, ok := wp.imgProcessor.(autoCropper); ok {
		return ac.AutoCropPercent(context.Background(), src, w, h)
	}
	return 0, false
}

// tunePreviewPath returns the image to preview in Tune Image: the original
// download when it is on disk, otherwise the wallpaper file on screen.
func (wp *Plugin) tunePreviewPath(mc *MonitorController) string {
	mc.mu.RLock()
	img := mc.State.CurrentImage
	mc.mu.RUnlock()
	if wp.fm != nil && img.ID != "" && img.FilePath != "" {
		if master, err := wp.fm.GetMasterPath(img.ID, filepath.Ext(img.FilePath)); err == nil {
			if _, err := os.Stat(master); err == nil {
				return master
			}
		}
	}
	return img.FilePath
}
