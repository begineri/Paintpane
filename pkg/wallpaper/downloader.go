package wallpaper

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// ErrInAvoidSet is returned for a job whose image has been blocked. Skipping it
// is expected, so it is not logged as a failed job.
var ErrInAvoidSet = errors.New("image is in avoid set")

// ProcessImageJob processes a single image download job.
// It implements the Source + Derivative architecture:
// 1. Ensure Master (Raw) exists.
// 2. Ensure Derivative (Processed) exists (generating from Master if needed).
func (wp *Plugin) ProcessImageJob(ctx context.Context, job DownloadJob) (resultImg provider.Image, finalErr error) {
	img := job.Image
	downloadProvider := job.Provider

	// One job per image at a time (A2 phase 5 #4). A second download of an image in progress is
	// dropped, as it would be once the image is in the store; a local rebuild waits its turn. The
	// claim is given back after the clean-up below, so no other job sees files being deleted.
	if downloadProvider != nil {
		release, ok := wp.jobClaims.tryClaim(img.ID)
		if !ok {
			return provider.Image{}, fmt.Errorf("%w: %s", ErrImageInProgress, img.ID)
		}
		defer release()
	} else {
		release, err := wp.jobClaims.claim(ctx, img.ID)
		if err != nil {
			return provider.Image{}, err
		}
		defer release()
	}

	// Prevent orphaned files from leaking by ensuring partial processing artifacts are scrubbed.
	// If context is cancelled midway through downloading/processing, we aggressively clean up
	// any master or derivative files we just wrote instead of waiting for the nightly sweep.
	defer func() {
		if errors.Is(finalErr, ErrInAvoidSet) {
			// Expected after blocking an image; nothing was written (A2 §12.9 #5).
			return
		}
		if finalErr != nil && wp.fm != nil {
			log.Debugf("Job failed for %s (Err: %v), cleaning up artifacts...", img.ID, finalErr)
			if downloadProvider == nil {
				// A local rebuild downloaded nothing, its master may be the only copy, and the
				// derivatives kept for other display modes are still good.
				return
			}
			_ = wp.fm.DeepDelete(img.ID)
		}
	}()

	if wp.cfg.InAvoidSet(img.ID) {
		return provider.Image{}, fmt.Errorf("%w: %s", ErrInAvoidSet, img.ID)
	}

	// 0. Early Filtering (Optimization)
	if err := wp.checkImageCompatibility(img); err != nil {
		return provider.Image{}, err
	}

	// 1. Lazy Enrichment (Soft Fail)
	// (API pacing is automatically handled upstream by the Fair Scheduler Dispatcher)
	if downloadProvider != nil {
		log.Debugf("Enriching image %s...", img.ID)
	}
	img = wp.enrichImage(ctx, img, downloadProvider)

	// 1.5 Oversized originals: decoding a 200 MP JPEG takes gigabytes of memory, so an image the
	// source reports as larger than MaxMasterPixels is skipped before anything is downloaded.
	// (Providers that can serve a smaller version, like Wikimedia, hand us that version instead.)
	if err := checkMasterSize(img); err != nil {
		return provider.Image{}, err
	}

	// 2. Ensure Master (Raw Image)
	// (Download pacing is automatically handled upstream by the Fair Scheduler Dispatcher)
	if downloadProvider != nil {
		log.Debugf("Downloading/Ensuring raw master image %s...", img.ID)
	}
	masterPath, err := wp.ensureMaster(ctx, img, downloadProvider)
	if err != nil {
		providerName := "Unknown"
		if downloadProvider != nil {
			providerName = downloadProvider.ID()
		}
		return provider.Image{}, fmt.Errorf("failed to ensure master (%s): %w", providerName, err)
	}

	// 2.3 Originals are capped at about 4K unless High-resolution originals is on (master_cap.go).
	// An original larger than that (downloaded in full, or kept from before) is shrunk here, and
	// the image takes the size of the file on disk: a scaled download is smaller than the size the
	// source reported.
	if wp.capOriginals() {
		w, h, err := capMaster(ctx, masterPath)
		switch {
		case err == nil:
			if w > 0 && h > 0 {
				img.Width, img.Height = w, h
			}
		case ctx.Err() != nil:
			return provider.Image{}, ctx.Err()
		default:
			log.Printf("Warning: could not cap the original of %s: %v", img.ID, err)
		}
	}

	// 2.5 Resolution Probing & Persistence (Fixes "Ghost Dimensions")
	if img.Width == 0 || img.Height == 0 {
		w, h, err := wp.probeDimensions(masterPath)
		if err == nil {
			img.Width = w
			img.Height = h
			log.Debugf("Probed dimensions for %s: %dx%d.", img.ID, w, h)
		} else {
			log.Printf("Warning: Failed to probe dimensions for %s: %v", img.ID, err)
		}
	}

	// 2.7 Multi-Monitor Compatibility & Rejection Tagging
	// We check again now that we have REAL dimensions (either from API or Probing)
	resolutions := wp.getResolutionsForDerivatives()
	incompatibleCount := 0
	for _, res := range resolutions {
		resKey := fmt.Sprintf("%dx%d", res.Width, res.Height)
		tagKey := "incompatible:" + resKey

		// Check if it was already tagged as incompatible.
		if img.ProcessingFlags[tagKey] {
			incompatibleCount++
			continue
		}

		// Perform actual check
		if err := wp.imgProcessor.CheckCompatibility(img.Width, img.Height, res.Width, res.Height); err != nil && !errors.Is(err, ErrRequiresVirtualFraming) {
			log.Debugf("Image %s is incompatible with %s: %v. Tagging.", img.ID, resKey, err)
			if img.ProcessingFlags == nil {
				img.ProcessingFlags = make(map[string]bool)
			}
			img.ProcessingFlags[tagKey] = true
			incompatibleCount++
		}
	}

	// Note: Rejection tags are persisted when the fully-processed image
	// lands in the store via stateManagerLoop (Add or Update fallback).

	if incompatibleCount == len(resolutions) && len(resolutions) > 0 {
		return img, fmt.Errorf("incompatible image skipped (fits zero monitors)")
	}

	// 3. Ensure Derivative (Processed Image)
	stampProcessingFlags(&img, wp.targetProcessingFlags())

	derivativePaths, err := wp.ensureDerivative(ctx, img, masterPath)
	if err != nil {
		return img, fmt.Errorf("failed to ensure derivative: %w", err)
	}

	applyDerivativePaths(&img, derivativePaths)

	if wp.favoriter != nil && wp.favoriter.IsFavorited(img) {
		img.IsFavorited = true
	}

	return img, nil
}

func (wp *Plugin) checkImageCompatibility(img provider.Image) error {
	if img.Width <= 0 || img.Height <= 0 {
		return nil // Cannot check without dimensions
	}
	// Check the size the original will have on disk: a capped original has less to spare when
	// cropped (Smart Fit's Flexibility mode allows more for a larger original).
	img.Width, img.Height = wp.expectedMasterSize(img.Width, img.Height)

	var resolutions []Resolution
	monitors, err := wp.os.GetMonitors()
	if err == nil && len(monitors) > 0 {
		resolutions = GetUniqueResolutions(monitors)
	}

	// Fallback to primary if Sync is OFF or GetMonitors failed
	if len(resolutions) == 0 {
		w, h, err := wp.os.GetDesktopDimension()
		if err == nil {
			resolutions = append(resolutions, Resolution{Width: w, Height: h})
		}
	}

	// Check all candidate resolutions.
	incompatibleCount := 0
	for _, res := range resolutions {
		resKey := fmt.Sprintf("%dx%d", res.Width, res.Height)
		tagKey := "incompatible:" + resKey

		// Check rejection tag first
		if img.ProcessingFlags[tagKey] {
			incompatibleCount++
			continue
		}

		if err := wp.imgProcessor.CheckCompatibility(img.Width, img.Height, res.Width, res.Height); err != nil && !errors.Is(err, ErrRequiresVirtualFraming) {
			incompatibleCount++
		}
	}

	if incompatibleCount == len(resolutions) && len(resolutions) > 0 {
		return fmt.Errorf("incompatible image skipped (fits zero monitors)")
	}
	return nil
}

// probeDimensions uses DecodeConfig to get image dimensions without loading the whole file.
func (wp *Plugin) probeDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	return cfg.Width, cfg.Height, nil
}

func (wp *Plugin) enrichImage(ctx context.Context, img provider.Image, p provider.ImageProvider) provider.Image {
	if p == nil {
		return img
	}

	// *** NAMESPACING Middleware ***
	// Strip prefix so provider sees raw ID
	originalID := img.ID
	namespaced := false
	if p.Type() != provider.TypePersonal {
		prefix := p.ID() + "_"
		if strings.HasPrefix(img.ID, prefix) {
			img.ID = strings.TrimPrefix(img.ID, prefix)
			namespaced = true
		}
	}

	enrichedImg, err := p.EnrichImage(ctx, img)
	if err != nil {
		// SOFT FAIL: Log warning but proceed.
		log.Debugf("Lazy enrichment failed for %s (will try later): %v", originalID, err)
		// Restore ID if we stripped it, just in case
		if namespaced {
			img.ID = originalID
		}
		return img
	}

	// Restore Namespaced ID
	if namespaced {
		enrichedImg.ID = originalID
	}

	return enrichedImg
}

// ensureMaster ensures the raw image is on disk.
// Returns absolute path.
// masterPathFor returns where an image's master file is stored.
func (wp *Plugin) masterPathFor(img provider.Image) (string, error) {
	// Determine extension. We prefer what's in URL or Content-Type.
	ext := filepath.Ext(extractFilenameFromURL(img.Path))
	if ext == "" {
		if img.FileType == "image/png" {
			ext = ".png"
		} else {
			ext = ".jpg" // Default
		}
	}

	masterPath, err := wp.fm.GetMasterPath(img.ID, ext)
	if err != nil {
		return "", fmt.Errorf("security check failed for master path: %w", err)
	}
	return masterPath, nil
}

func (wp *Plugin) ensureMaster(ctx context.Context, img provider.Image, imgProvider provider.ImageProvider) (string, error) {
	masterPath, err := wp.masterPathFor(img)
	if err != nil {
		return "", err
	}

	// Check existence
	if _, err := os.Stat(masterPath); !os.IsNotExist(err) {
		// Exists.
		return masterPath, nil
	}

	// Download Remote URL
	client := wp.httpClient
	if cp, ok := imgProvider.(provider.CustomClientProvider); ok {
		client = cp.GetClient()
	}

	reqUrl := img.Path
	if rap, ok := imgProvider.(provider.ResolutionAwareProvider); ok {
		// Attempt to get screen resolution to request a perfectly sized image
		prefs := wp.manager.GetPreferences()
		w, h := prefs.Int("screen_width"), prefs.Int("screen_height")
		if w > 0 && h > 0 {
			reqUrl = rap.WithResolution(reqUrl, w, h)
		}
	}

	// While originals are capped, ask a server that can scale for the smaller copy.
	if wp.capOriginals() {
		if sp, ok := imgProvider.(provider.ScaledDownloadProvider); ok {
			if scaledURL, ok := sp.ScaledDownloadURL(ctx, img, cappedSize); ok {
				path, err := wp.downloadMasterFile(ctx, client, scaledURL, masterPath, imgProvider)
				var se *statusError
				if err == nil || ctx.Err() != nil || !errors.As(err, &se) || se.code < 400 || se.code >= 500 {
					return path, err
				}
				// The server refused that size: take the full image and shrink it here.
				log.Printf("Scaled download of %s refused (%v); downloading the full image", img.ID, err)
			}
		}
	}

	return wp.downloadMasterFile(ctx, client, reqUrl, masterPath, imgProvider)
}

// MaxMasterPixels is the largest original (width x height, as reported by the source) that is
// downloaded. A 200 MP JPEG decodes to roughly a gigabyte per worker, and four workers run at once.
// It applies whether or not originals are capped: a source that reports the size and cannot scale
// would send the whole file, which has to be decoded to be shrunk.
const MaxMasterPixels = 100_000_000

// checkMasterSize rejects an image whose reported size exceeds MaxMasterPixels. Images without a
// reported size pass; they are measured after download.
func checkMasterSize(img provider.Image) error {
	if img.Width <= 0 || img.Height <= 0 {
		return nil
	}
	if px := int64(img.Width) * int64(img.Height); px > MaxMasterPixels {
		return fmt.Errorf("oversized image skipped (%dx%d, %d MP; limit %d MP)", img.Width, img.Height, px/1_000_000, MaxMasterPixels/1_000_000)
	}
	return nil
}

const (
	// downloadAttempts is how many times one image download is tried in total. Some image servers
	// (SMK's iip.smk.dk was seen hanging about half of its requests while the retry returned in
	// seconds) stall at random, so a second try usually works. The per-request timeouts stay as
	// they are; only the number of tries grows.
	downloadAttempts = 4
	// downloadRetryDelay is the pause between tries.
	downloadRetryDelay = 1500 * time.Millisecond
)

// downloadRetryWait is the pause between download attempts. A variable so tests can shorten it.
var downloadRetryWait = downloadRetryDelay

// statusError is returned for an unexpected HTTP status, so the retry logic can tell a server
// hiccup (502/503/504) from an answer that will not change (404, 403, ...).
type statusError struct {
	provider string
	code     int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("failed to ensure master (%s): status %d", e.provider, e.code)
}

// isRetryableDownloadError reports whether a failed download attempt is worth repeating: a timeout,
// a dropped or refused connection, a body cut short, or a 502/503/504. A cancelled context, a 4xx
// answer and errors raised by a provider's own client (such as a rate-limit circuit breaker) are not.
func isRetryableDownloadError(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	var se *statusError
	if errors.As(err, &se) {
		return se.code == http.StatusBadGateway || se.code == http.StatusServiceUnavailable || se.code == http.StatusGatewayTimeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	var oe *net.OpError
	if errors.As(err, &oe) {
		return true
	}
	return errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
}

// downloadMasterFile downloads reqUrl to masterPath, trying again after a timeout or a dropped
// connection (see downloadAttempts).
func (wp *Plugin) downloadMasterFile(ctx context.Context, client *http.Client, reqUrl, masterPath string, imgProvider provider.ImageProvider) (string, error) {
	source := ""
	if imgProvider != nil {
		source = imgProvider.ID()
	}
	backoff := wp.downloadBackoff()
	attempts := downloadAttempts
	if ap, ok := imgProvider.(provider.DownloadAttemptsProvider); ok && ap.DownloadAttempts() > 0 {
		attempts = ap.DownloadAttempts()
	}
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		var path string
		path, err = wp.downloadMasterFileOnce(ctx, client, reqUrl, masterPath, imgProvider)
		if err == nil {
			backoff.succeeded(source)
			return path, nil
		}
		if !isRetryableDownloadError(ctx, err) {
			return "", err
		}
		if op, ok := imgProvider.(provider.OutagePauseProvider); ok {
			if pause := op.OutagePause(backoff.now()); pause > 0 {
				// The source says its server is down at this time: do not try again, wait it out.
				backoff.pauseForOutage(source, pause)
				break
			}
		}
		if attempt == attempts {
			backoff.failed(source)
			break
		}
		if backoff.pausedFor(source) > 0 {
			// Other images already showed the server is unreachable; stop trying this one.
			break
		}
		log.Printf("Download attempt %d/%d failed for %s: %v; trying again", attempt, attempts, reqUrl, err)
		select {
		case <-ctx.Done():
			return "", err
		case <-time.After(downloadRetryWait):
		}
	}
	return "", err
}

func (wp *Plugin) downloadMasterFileOnce(ctx context.Context, client *http.Client, reqUrl, masterPath string, imgProvider provider.ImageProvider) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Standard User-Agent to prevent 403 Forbidden from providers like Pexels
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	if hp, ok := imgProvider.(provider.HeaderProvider); ok {
		headers := hp.GetDownloadHeaders()
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		providerName := "Unknown"
		if imgProvider != nil {
			providerName = imgProvider.ID()
		}
		return "", &statusError{provider: providerName, code: resp.StatusCode}
	}

	// Write to a temporary file and rename it when complete, so nothing ever reads half a master
	// (A2 phase 5 #4). The name ends in the image extension, so the orphan sweep removes one left
	// behind by a crash.
	dir, base := filepath.Split(masterPath)
	ext := filepath.Ext(base)
	file, err := os.CreateTemp(dir, strings.TrimSuffix(base, ext)+".part-*"+ext)
	if err != nil {
		return "", err
	}
	tmpPath := file.Name()
	discard := func() {
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to clean up aborted download %s: %v", tmpPath, err)
		}
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close() // Close before attempting removal
		discard()
		return "", err
	}
	if err := file.Close(); err != nil {
		discard()
		return "", err
	}
	if err := os.Rename(tmpPath, masterPath); err != nil {
		discard()
		return "", err
	}

	return masterPath, nil
}

// getBaseDerivativeDir returns the relative path segment for derivatives based on current settings.
// Format: fitted / [quality|flexibility] / [standard|faceboost|facecrop]
// or fitted / [full|framed] / standard. The display mode comes first because a whole-image
// mode must still process images when Smart Fit is disabled, or the OS would crop the master.
func (wp *Plugin) getBaseDerivativeDir() string {
	switch wp.cfg.GetArtworkDisplayMode() {
	case ArtworkDisplayFull:
		return filepath.Join(FittedRootDir, FullArtDir, StandardDir)
	case ArtworkDisplayFramed:
		return filepath.Join(FittedRootDir, FramedArtDir, StandardDir)
	}

	mode := wp.cfg.GetSmartFitMode()
	if mode == SmartFitOff {
		return ""
	}

	// 1. Determine Mode Segment
	var modeDir string
	if mode == SmartFitAggressive {
		modeDir = FlexibilityDir
	} else {
		modeDir = QualityDir
	}

	// 2. Determine Type Segment
	var typeDir string
	if wp.cfg.GetFaceCropEnabled() {
		typeDir = FaceCropDir
	} else if wp.cfg.GetFaceBoostEnabled() {
		typeDir = FaceBoostDir
	} else {
		typeDir = StandardDir
	}

	return filepath.Join(FittedRootDir, modeDir, typeDir)
}

// ensureDerivative ensures the processed image exists for all detected monitor resolutions.
// Returns a map of resolution "WxH" -> absolute path.
func (wp *Plugin) ensureDerivative(ctx context.Context, img provider.Image, masterPath string) (map[string]string, error) {
	derivativeDir := wp.getBaseDerivativeDir()
	if derivativeDir == "" {
		return map[string]string{"primary": masterPath}, nil
	}

	ext := filepath.Ext(masterPath)
	resolutions := wp.getResolutionsForDerivatives()

	// 1. Check if all exist
	paths, allExist := wp.checkExistingDerivatives(img.ID, ext, derivativeDir, resolutions)
	wp.recordFramedFlagsForReuse(img, resolutions, paths)
	if allExist && len(paths) > 0 {
		return wp.ensurePrimaryPath(paths), nil
	}

	// 2. Generate Missing
	if err := wp.generateMissingDerivatives(ctx, img, masterPath, derivativeDir, ext, resolutions, paths); err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("incompatible: failed to generate any derivatives")
	}

	return wp.ensurePrimaryPath(paths), nil
}

func (wp *Plugin) getResolutionsForDerivatives() []Resolution {
	monitors, err := wp.os.GetMonitors()
	if err != nil || len(monitors) == 0 {
		log.Printf("Warning: No monitors found (or error: %v). Using Safe Fallback (1920x1080).", err)
		monitors = []Monitor{{ID: 0, Name: "Fallback", Rect: image.Rect(0, 0, 1920, 1080)}}
	}
	return GetUniqueResolutions(monitors)
}

// applyDerivativePaths points an image at its derivatives for display.
func applyDerivativePaths(img *provider.Image, derivativePaths map[string]string) {
	img.DerivativePaths = derivativePaths

	// Set a default FilePath for legacy compatibility (Monitor 0)
	if path, ok := derivativePaths["primary"]; ok {
		img.FilePath = path
		return
	}
	// Fallback to first available
	for _, path := range derivativePaths {
		img.FilePath = path
		break
	}
}

// frameDecider is implemented by processors that can say whether they would draw a frame.
type frameDecider interface {
	WouldFrame(providerID string, imgWidth, imgHeight, targetWidth, targetHeight int, opts provider.TuningOptions) bool
}

// recordFramedFlagsForReuse sets the framed flag for derivatives that are reused rather than
// rendered, since that flag is otherwise only written while rendering.
func (wp *Plugin) recordFramedFlagsForReuse(img provider.Image, resolutions []Resolution, paths map[string]string) {
	decider, ok := wp.imgProcessor.(frameDecider)
	if !ok || img.ProcessingFlags == nil {
		return
	}
	for _, res := range resolutions {
		resDir := fmt.Sprintf("%dx%d", res.Width, res.Height)
		if _, reused := paths[resDir]; !reused {
			continue
		}
		if decider.WouldFrame(img.Provider, img.Width, img.Height, res.Width, res.Height, img.GetTuning(resDir)) {
			img.ProcessingFlags["VirtualFramed:"+resDir] = true
		} else {
			delete(img.ProcessingFlags, "VirtualFramed:"+resDir)
		}
	}
}

// saveDerivative writes a derivative under a temporary name and renames it into place. Derivatives
// are kept and reused, so a file cut short by a crash must never appear at the final path.
func saveDerivative(img image.Image, targetPath string) error {
	ext := filepath.Ext(targetPath)
	tmpPath := strings.TrimSuffix(targetPath, ext) + ".tmp" + ext
	if err := imaging.Save(img, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (wp *Plugin) checkExistingDerivatives(imgID, ext, derivativeDir string, resolutions []Resolution) (map[string]string, bool) {
	paths := make(map[string]string)
	allExist := true

	for _, res := range resolutions {
		resDir := fmt.Sprintf("%dx%d", res.Width, res.Height)
		fullDir := filepath.Join(derivativeDir, resDir)
		targetPath, _ := wp.fm.GetDerivativePath(imgID, ext, fullDir)

		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			allExist = false
			// We continue to populate paths for those that DO exist, or just break?
			// The original logic broke on first missing, but we need to know WHICH exist if we want to be partial.
			// However, original logic restarted generation for ALL if one was missing (inefficient but simple).
			// Let's stick to "if any missing, we proceed to generation phase".
			// But wait, the generation phase might re-check existence.
		} else {
			paths[resDir] = targetPath
			// Mark primary
			for _, mid := range res.Monitors {
				if mid == 0 {
					paths["primary"] = targetPath
				}
			}
		}
	}
	return paths, allExist
}

func (wp *Plugin) generateMissingDerivatives(ctx context.Context, img provider.Image, masterPath, derivativeDir, ext string, resolutions []Resolution, paths map[string]string) error {
	// Large originals are decoded one at a time (A2 §12.9 #8).
	release, err := acquireMasterDecode(ctx, masterPath)
	if err != nil {
		return err
	}
	defer release()

	srcImg, err := openMaster(masterPath)
	if err != nil {
		return fmt.Errorf("failed to open master %s: %w", masterPath, err)
	}

	for _, res := range resolutions {
		if err := ctx.Err(); err != nil {
			return err
		}
		resDir := fmt.Sprintf("%dx%d", res.Width, res.Height)

		// Skip if we already found it in the check phase?
		// The check phase populated 'paths' even if allExist was false.
		if _, ok := paths[resDir]; ok {
			continue
		}

		fullDir := filepath.Join(derivativeDir, resDir)
		targetPath, err := wp.fm.GetDerivativePath(img.ID, ext, fullDir)
		if err != nil {
			log.Printf("Error getting derivative path for %s: %v", resDir, err)
			continue
		}

		// Double check existence (persistence race or just safety)
		if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
			paths[resDir] = targetPath
			continue
		}

		// Compatibility check
		if err := wp.imgProcessor.CheckCompatibility(srcImg.Bounds().Dx(), srcImg.Bounds().Dy(), res.Width, res.Height); err != nil && !errors.Is(err, ErrRequiresVirtualFraming) {
			log.Debugf("Skipping derivative for %s: incompatible: %v", resDir, err)
			continue
		}

		// Generate
		var framed bool
		fitCtx := context.WithValue(ctx, provider.VirtualFramedKey, &framed)
		fitCtx = context.WithValue(fitCtx, provider.ProviderIDKey, img.Provider)
		processedImg, err := wp.imgProcessor.FitImage(fitCtx, srcImg, res.Width, res.Height, img.GetTuning(resDir))

		if err == nil {
			if framed {
				if img.ProcessingFlags == nil {
					img.ProcessingFlags = make(map[string]bool)
				}
				img.ProcessingFlags["VirtualFramed:"+resDir] = true
			} else if img.ProcessingFlags != nil {
				delete(img.ProcessingFlags, "VirtualFramed:"+resDir)
			}
		}
		if err != nil {
			log.Printf("Error fitting image for %s: %v", resDir, err)
			if strings.Contains(err.Error(), "rejected:") || strings.Contains(err.Error(), "incompatible") {
				if img.ProcessingFlags == nil {
					img.ProcessingFlags = make(map[string]bool)
				}
				img.ProcessingFlags["incompatible:"+resDir] = true
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			log.Printf("Error creating directory for %s: %v", targetPath, err)
			continue
		}

		if err := saveDerivative(processedImg, targetPath); err != nil {
			log.Printf("Error saving derivative %s: %v", targetPath, err)
			continue
		}

		paths[resDir] = targetPath
		wp.updatePrimaryPath(paths, res, targetPath)
	}
	return nil
}

func (wp *Plugin) updatePrimaryPath(paths map[string]string, res Resolution, targetPath string) {
	for _, mid := range res.Monitors {
		if mid == 0 {
			paths["primary"] = targetPath
		}
	}
}

func (wp *Plugin) ensurePrimaryPath(paths map[string]string) map[string]string {
	if _, ok := paths["primary"]; !ok {
		for _, p := range paths {
			paths["primary"] = p
			break
		}
	}
	return paths
}
