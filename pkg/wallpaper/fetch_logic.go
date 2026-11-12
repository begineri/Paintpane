package wallpaper

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// FetchNewImages iterates over active queries and submits new image jobs to the pipeline.
// If force is true, it proceeds even if another fetch is in progress (ignoring the debounce lock).
// If providerID is specified, only queries for that provider are fetched.
func (wp *Plugin) FetchNewImages(force bool, providerID ...string) {
	targetProvider := ""
	if len(providerID) > 0 {
		targetProvider = providerID[0]
	}

	// Special-case Favorites for on-the-fly responsiveness
	isFavRequest := targetProvider == "Favorites"

	if force || isFavRequest || wp.fetchingInProgress.CompareAndSwap(false, true) {
		go func() {
			if !isFavRequest && !force {
				defer wp.fetchingInProgress.Set(false)
			}
			log.Debugf("Starting image fetch (Target: %s)...", func() string {
				if targetProvider == "" {
					return "ALL"
				}
				return targetProvider
			}())

			wp.downloadMutex.RLock()
			if wp.cfg == nil || wp.jobSubmitter == nil {
				wp.downloadMutex.RUnlock()
				log.Println("Fetch skipped - plugin not fully activated yet (pipeline not ready).")
				return
			}
			wp.cfg.mu.Lock()
			queries := make([]ImageQuery, len(wp.cfg.Queries))
			copy(queries, wp.cfg.Queries)
			wp.cfg.mu.Unlock()
			wp.downloadMutex.RUnlock()

			totalQueued := util.NewSafeInt()
			var answered, newImages atomic.Int64 // for the start-up "nothing new today" mark
			activeSources := make(map[string]bool)
			var sourcesMutex sync.Mutex

			// Initialize the global fetch context to allow remote aborts
			fetchCtx := wp.StartFetchContext()

			newImageBudget := wp.newImageBudgetPerQuery(queries, targetProvider)

			// Semaphore to limit concurrent fetches
			sem := make(chan struct{}, 5)
			var wg sync.WaitGroup

			for _, q := range queries {
				if !q.Active {
					continue
				}

				// Targeted Fetch filter
				if targetProvider != "" && q.Provider != targetProvider {
					continue
				}

				p, ok := wp.providers[q.Provider]
				if !ok {
					log.Printf("Provider %s not found for query %s", q.Provider, q.ID)
					continue
				}

				wg.Add(1)
				go func(q ImageQuery, p provider.ImageProvider) {
					defer wg.Done()

					// Pattern: Early Exit (Circuit Breaker)
					if tp, ok := p.(provider.ThrottledProvider); ok {
						if tp.IsThrottled() {
							log.Printf("Provider %s is currently throttled. Skipping fetch for query %s.", p.ID(), q.ID)
							return
						}
					}

					// Pattern: Pacing Penalty OUTSIDE of CPU semaphore
					// Wait freely without holding any execution lock so we don't starve fast providers!
					if limiter := wp.getAPILimiter(p); limiter != nil {
						log.Debugf("[Pacing] Waiting for API rate limiter slot for provider %s...", p.ID())
						if err := limiter.Wait(fetchCtx); err != nil {
							log.Printf("Provider %s fetch aborted due to context cancellation during pacing: %v", p.ID(), err)
							return
						}
					}

					log.Debugf("[Fetch] %s Waiting for slot... (Lane Load: %d/%d)", p.ID(), len(sem), cap(sem))
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }()
					case <-fetchCtx.Done():
						return
					}

					ok, added := wp.fetchFromProvider(fetchCtx, q, p, isFavRequest, newImageBudget, &sourcesMutex, activeSources, totalQueued)
					if ok {
						answered.Add(1)
					}
					newImages.Add(int64(added))
				}(q, p)
			}

			wg.Wait()

			// Only a whole round of every active query, run to the end, says whether the sources
			// have anything new (startupFetch).
			if targetProvider == "" && !isFavRequest && fetchCtx.Err() == nil && (wp.interrupt == nil || !wp.interrupt.Value()) {
				wp.noteFetchRound(int(answered.Load()), int(newImages.Load()))
			}

			// Batch Reshuffle Optimization (User Approach):
			// Signal monitors to update their shuffle lists only after the entire batch is processed.
			if totalQueued.Value() > 0 && (wp.interrupt == nil || !wp.interrupt.Value()) {
				log.Debugf("[Fetch] Processed %d new images. Broadcasting shuffle update to monitors...", totalQueued.Value())
				wp.dispatch(-1, CmdUpdateShuffle)
			} else {
				log.Println("Fetch returned 0 new images from all active queries.")
			}

			log.Println("Fetch cycle completed.")
		}()
	} else {
		log.Println("Fetch skipped - already in progress.")
	}
}

// RefreshImagesAndPulse triggers a fetch and then updates the wallpaper.
func (wp *Plugin) RefreshImagesAndPulse() {
	go func() {
		// Optimization: Removed Master Reset (Page 1) to prevent cache churn on Settings Apply.
		// Pages are now persistent per session or until Nightly Refresh.

		// Ensure any blocked fetches are aborted so the new cycle can begin
		wp.CancelFetchContext()

		// Robust Sync: Reconcile store and invalidate stale derivatives
		wp.syncStoreWithConfig()

		// Bring the existing library back without waiting for its pages to be fetched again
		go wp.rebuildMissingDerivatives()

		// Ensure tray menu is rebuilt (e.g. if Favorites was toggled)
		if wp.manager != nil {
			wp.manager.RebuildTrayMenu()
		}

		// Trigger immediate fetch (Forced because we just applied settings)
		wp.FetchNewImages(true)

		// Wait for images using event driven notification (up to 60s to account for large slow fetches)
		ctx, cancel := context.WithTimeout(wp.ctx, 60*time.Second)
		defer cancel()

		log.Debugf("[Init] Waiting for images before initial pulse...")
		if err := wp.store.WaitForImages(ctx); err == nil {
			log.Debugf("[Init] Images available. Triggering initial pulse.")
			// Use dispatch directly to bypass Stagger logic (Force Immediate)
			wp.dispatch(-1, CmdNext)
		} else {
			log.Println("[Init] Initial pulse timeout. Triggering anyway.")
			wp.dispatch(-1, CmdNext)
		}
	}()
}

// cacheLimit returns the configured maximum number of cached images. It falls back the
// same way ImageStore.Sync does for an unrecognized setting.
func (wp *Plugin) cacheLimit() int {
	if limit := wp.cfg.GetCacheSize().Size(); limit > 0 {
		return limit
	}
	return 50
}

// newImageBudgetPerQuery returns how many new images each active query may download in
// this cycle. A cycle only tops the cache up with images still waiting to be shown,
// split evenly so that fast providers don't crowd out rate-limited ones. Without this,
// one cycle downloads every active collection in full, whatever the cache limit is.
func (wp *Plugin) newImageBudgetPerQuery(queries []ImageQuery, targetProvider string) int {
	activeQueries := 0
	for _, q := range queries {
		if q.Active && (targetProvider == "" || q.Provider == targetProvider) {
			activeQueries++
		}
	}
	room := wp.cacheLimit() - wp.store.ReadyUnseenCount()
	if activeQueries == 0 || room <= 0 {
		return 0
	}
	return (room + activeQueries - 1) / activeQueries // Round up so small rooms still fetch.
}

// fetchFromProvider fetches the query's current page and queues its images. It reports whether the
// source answered (with or without images) and how many images new to the library it queued.
func (wp *Plugin) fetchFromProvider(fetchCtx context.Context, q ImageQuery, p provider.ImageProvider, isFavRequest bool, newImageBudget int, sourcesMutex *sync.Mutex, activeSources map[string]bool, totalQueued *util.SafeCounter) (answered bool, newImages int) {
	// While a source's image server cannot be reached its downloads are paused (A2 §12.9 #4).
	// Fetching would only queue more jobs behind the pause and move past its pages, so the
	// query is left for a later cycle.
	if wait := wp.downloadBackoff().pausedFor(p.ID()); wait > 0 {
		log.Printf("Skipping %s (query %s) for now: its downloads are paused for %v", p.ID(), q.ID, wait.Round(time.Second))
		return false, 0
	}

	// Get or create per-query page counter
	wp.downloadMutex.Lock()
	pg, ok := wp.queryPages[q.ID]
	if !ok {
		pg = util.NewSafeIntWithValue(1)
		wp.queryPages[q.ID] = pg
	}
	wp.downloadMutex.Unlock()

	page := pg.Value()
	log.Debugf("Fetching from provider: %s (Query: %s, Page: %d)", q.Provider, q.Description, page)

	// Add timeout to prevent hangs (increased to 60s to allow sequential API scraping loops to complete)
	ctx, cancel := context.WithTimeout(fetchCtx, 60*time.Second)
	defer cancel()

	// Rate limit API calls via PacedProvider interface
	// Pattern updated: Pacing penalty is now paid BEFORE grabbing the global semaphore in FetchNewImages.

	images, err := p.FetchImages(ctx, q.URL, page)
	if err != nil {
		log.Printf("Provider %s fetch failed: %v", q.Provider, err)
		return false, 0
	}
	if len(images) == 0 {
		log.Printf("Provider %s returned no new images for query %s.", q.Provider, q.ID)
		// Safe Page Wrapping: Auto-reset to page 1 for all providers to ensure they cycle through content correctly.
		// This ensures that "Nightly Refresh" and "Starvation Fetches" eventually see new content or loop back to favorites.
		if page > 1 {
			pg.Set(1)
			wp.saveQueryPages()
			log.Debugf("Query %s: End of content reached on page %d. Wrapping back to page 1.", q.ID, page)
		}
		return true, 0
	}

	log.Debugf("[Fetch] Provider %s returned %d images. Submitting to pipeline.", q.Provider, len(images))

	// Track source
	sourcesMutex.Lock()
	activeSources[p.ID()] = true
	sourcesMutex.Unlock()

	queuedForThisQuery := 0
	newQueued := 0
	budgetExhausted := false
	aborted := false
	// Favorites are local files the user chose to keep, so the cache budget doesn't apply.
	budgeted := !isFavRequest && q.ID != FavoritesQueryID

	// Defense in depth: the pipeline may not be ready yet (pre-Activate window) or may
	// have been torn down by a concurrent Deactivate. Bail out without advancing pagination
	// instead of dereferencing a nil jobSubmitter interface.
	wp.downloadMutex.RLock()
	submitter := wp.jobSubmitter
	wp.downloadMutex.RUnlock()
	if submitter == nil {
		log.Printf("Fetch aborted for query %s: pipeline not available (plugin activating or deactivating).", q.ID)
		return true, 0
	}

	// Instantiate the background cancellation context for this specific query
	queryCtx := wp.GetOrCreateQueryContext(q.ID)

	for _, img := range images {
		// Critical Fix: Tag image with its source query ID so Sync knows it's active.
		img.SourceQueryID = q.ID

		// *** NAMESPACING Middleware ***
		// Ensure ID is unique across providers by prefixing it.
		// Local providers use filesystem paths (already unique), all others need namespacing.
		if p.Type() != provider.TypePersonal {
			prefix := p.ID() + "_"
			if !strings.HasPrefix(img.ID, prefix) {
				img.ID = prefix + img.ID
			}
		}

		if !isFavRequest && wp.cfg.InAvoidSet(img.ID) {
			log.Debugf("Skipping blocked image: %s", img.ID)
			continue
		}

		// Pattern: Deduplication (At-the-Gate)
		// Deadlock Break: We only skip an image if it exists AND already has derivatives for your current monitors.
		// This allow "Backlog Healing": if you have 1000 images but 0 derivatives, this allows them back into the pipeline.
		existing, known := wp.store.GetByID(img.ID)
		if known {
			// Images cached before ratings were stored would otherwise never receive one,
			// because fully processed images are skipped below.
			if existing.Purity == "" && img.Purity != "" {
				wp.store.SetPurity(img.ID, img.Purity)
			}
			if wp.allMonitorDerivativesExist(existing) {
				log.Debugf("Skipping image already in store with all derivatives: %s", img.ID)
				continue
			}
			log.Debugf("Image %s exists but is missing derivatives. Allowing re-processing for backlog healing.", img.ID)
			// Merge existing metadata (like already probed dimensions) into the fetch-result image
			img.MergeExistingMetadata(existing)
		} else if budgeted {
			// Keep looping rather than breaking: later images on this page may still need healing.
			if newQueued >= newImageBudget {
				budgetExhausted = true
				continue
			}
			newQueued++
		}
		job := DownloadJob{
			Ctx:      queryCtx,
			Image:    img,
			Provider: p,
		}
		// Submit blocking (until buffer clears or fetchCtx aborts)
		if submitter.Submit(fetchCtx, job) {
			totalQueued.Increment()
			queuedForThisQuery++
			if !known {
				newImages++
			}
		} else {
			// If submit returns false, the pipeline stopped or the fetch cycle was aborted via fetchCtx.
			// Do not attempt to process the rest of the images, and do NOT advance the pagination!
			log.Printf("Fetch aborted or pipeline full. Dropping job and pausing fetch for query %s.", q.ID)
			aborted = true
			break
		}
	}

	// Stay on this page if it still has images we didn't have budget for, or that were not queued
	// because the fetch was stopped; pages are kept across restarts, so moving on would skip them.
	if len(images) > 0 && !budgetExhausted && !aborted {
		pg.Increment()
		wp.saveQueryPages() // Persist pagination state
		log.Debugf("Query %s: Successfully processed page %d (Found: %d, Queued: %d). Incrementing to page %d", q.ID, page, len(images), queuedForThisQuery, pg.Value())
	}
	return true, newImages
}

// allMonitorDerivativesExist checks if the image has a processed file for every unique monitor resolution.
func (wp *Plugin) allMonitorDerivativesExist(img provider.Image) bool {
	if len(img.DerivativePaths) == 0 {
		return false
	}

	resolutions := wp.getResolutionsForDerivatives()
	if len(resolutions) == 0 {
		return true // No monitors to satisfy
	}

	for _, res := range resolutions {
		resKey := fmt.Sprintf("%dx%d", res.Width, res.Height)
		if _, ok := img.DerivativePaths[resKey]; !ok {
			// Also check incompatibility tag correctly
			tagKey := "incompatible:" + resKey
			if img.ProcessingFlags[tagKey] {
				continue // Already verified incompatible for this resolution
			}
			return false
		}
	}

	return true
}
