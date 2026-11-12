# Design: Artwork Display Modes

**Status:** Approved for implementation
**Branch:** `feat/artwork-display-mode`
**Related:** [image_pipeline_deep_dive.md](image_pipeline_deep_dive.md), [internal_developer_context.md](internal_developer_context.md)

## 1. Problem

Spice crops most artwork to fill the screen. The Virtual Framer only frames an image when its aspect ratio is far from the monitor's (`virtual_framer.go`, `FitImage`); everything else goes to `SmartImageProcessor`, which crops.

Measured by applying the current rules to one user's 122 downloaded museum masters (2560x1600, Flexibility + Face Crop):

| Outcome | Images | Notes |
| :--- | :--- | :--- |
| Cropped | 87 (71%) | Keeps 76% of the painting on average |
| Cropped by more than 30% | 25 | Worst case loses about half |
| Framed, shown whole | 35 (29%) | |

For a museum wallpaper, losing part of a painting defeats the purpose. Users can force a frame per image through Tune Image, but there is no global setting.

## 2. Goals and non-goals

**Goals**

- A global choice of how artwork is presented, switchable from Preferences and in one click from the tray.
- In the two new modes, no image is ever cropped.
- Full Artwork is the default, including for existing users who have never chosen a mode. A mode someone has saved is never changed.

**Non-goals**

- A fast first visit to a mode. The first time a mode is used, its wallpapers still have to be generated from the originals (see section 6).
- A blurred backdrop. It was evaluated and rejected as visually distracting, but `renderFullArtwork` leaves room to add it later.
- Per-provider display modes. `MuseumFraming` exists in config but has no UI, and this work does not change it.

## 3. The three modes

| Mode | Behaviour | Average screen area used by the painting (same 122 images) |
| :--- | :--- | :--- |
| **Smart Crop** | The original behaviour: crop, or auto-frame on a large mismatch | 100% (but only 76% of the cropped paintings is visible) |
| **Full Artwork** (default) | Scale to fit the screen, fill the rest with the wall colour | 68% |
| **Museum Frame** | Every image goes in the gallery frame | 36% at default settings; about 61% at 100% frame size without matting |

## 4. Decision rules

Tune Image overrides stay per-image and always win over the global mode, with one change: **"unframe" must not bring cropping back in the new modes.** It means "show without a frame".

| Global mode | Override: Inherit | Override: Force On | Override: Force Off |
| :--- | :--- | :--- | :--- |
| Smart Crop | Existing rules | Frame | Crop |
| Full Artwork | Full | Frame | Full |
| Museum Frame | Frame | Frame | Full |

**Per-image display mode (P26, owner 2026-11-11).** Tune Image now chooses an image's whole display mode, not only a frame: `TuningOptions.Display` is Inherit, Full, Framed or Crop, and when it is set it wins over the global mode and over Force On / Force Off (`displayModeFor` in `virtual_framer.go`). So Smart Crop is reachable for one image under Full Artwork and Museum Frame, and Full Artwork or Museum Frame for one image under Smart Crop. The table above still applies to tuning saved with only the frame switch; the window folds that switch into `Display` and no longer sends it. With Smart Fit off, an image set to Crop is still cropped by `SmartImageProcessor`.

Invariant: an image whose effective mode is Full Artwork or Museum Frame never reaches `SmartImageProcessor.FitImage`, except for the existing tiny-image passthrough (images of 10px or less on a side).

`gallery_generator.go` calls the framer with Force On and `TightCrop`, so it is unaffected.

## 5. Design

### 5.1 Configuration

`ArtworkDisplayMode` is a new int enum in `config.go`. Unknown values read as Smart Crop.

The default is applied by `loadFromPrefs`, which sets Full Artwork before decoding the saved config. A saved config without the field therefore gets the default, and a saved value, including an explicit Smart Crop, is kept. The enum's zero value stays Smart Crop, so a bare `Config{}` does not change behaviour.

- Stored as the JSON field `artwork_display_mode`, next to the other `virtual_*` settings.
- **Must be added to the clone in `Config.save()`**, or it is silently dropped on save.
- Accessed through `GetArtworkDisplayMode` / `SetArtworkDisplayMode` under `c.mu`, because pipeline workers read it while the tray writes it.

### 5.2 Rendering (`virtual_framer.go`)

- `FitImage` applies the table in section 4.
- `renderFullArtwork` scales to fit (`min(targetW/w, targetH/h)`), upscaling if needed, and centres the result on a canvas filled by the existing `calculateWallColor`. The "Virtual Wall Color" setting therefore applies to this mode too.
- `VirtualFramedKey` is set only when a frame is really drawn, so the Tune Image checkbox stays truthful.
- `CheckCompatibility` returns `nil` in the new modes, so no image is rejected and Tune Image does not lock the frame toggle. Size errors still pass through, matching the existing rule.

Low-resolution images are upscaled and may look soft. This affected 1 of the 122 sampled images and is accepted.

### 5.3 Derivative paths

`getBaseDerivativeDir` returns `fitted/full/standard` or `fitted/framed/standard` in the new modes. This keeps the existing `fitted/<mode>/<type>` depth, and the file manager's recursive walks already cover it.

**The download directory may be a link.** A user can move the cache to another drive by replacing `wallpaper_downloads` with a junction. On at least one Windows machine, creating a new directory through such a junction fails with `ERROR_ALREADY_EXISTS` (183), while creating files works and creating the same directory on the target works. It reproduces for any junction placed in `%LOCALAPPDATA%\spice` and for no junction placed elsewhere; the underlying cause was not identified. Existing modes rarely notice because their directories already exist, but these modes need new ones. Every derivative then fails, and a job with a provider deletes its master on failure. `NewFileManager` therefore resolves a linked root once and works on the target directly. The `cache` directory can be moved the same way, so its users go through `config.GetCacheDir`, which resolves it too.

**Smart Fit "Disabled" must not bypass the new modes.** Today that setting makes `ensureDerivative` return the master without calling `FitImage`, which would leave the OS to crop it. The display mode check therefore comes first.

### 5.4 Cache invalidation

Two new flags join the processing hash: `DisplayFull` and `DisplayFramed`. Both are `false` in Smart Crop, and `flagsMatch` treats a missing key as `false`, so images processed before this feature still count as Smart Crop.

An existing user who upgrades moves to the default on the first start, which is a first visit to Full Artwork: their wallpapers are generated in the background, and their Smart Crop files are kept. `Activate` therefore also starts `rebuildMissingDerivatives`. Otherwise a user with "Change wallpaper on start" turned off would have an empty library until its pages were fetched again.

This relies on two fixes from `d83511a`, without which Museum Frame would regenerate the whole library repeatedly: `flagsMatch` no longer treats `VirtualFramed:<WxH>` as a setting, and the downloader and `syncStoreWithConfig` now share `wp.targetProcessingFlags()`. The new flags only need adding to that function.

### 5.5 Switching UI

Both entry points call `SetArtworkDisplayMode` and then the existing refresh path, `RefreshImagesAndPulse`.

- **Preferences:** a selector at the top of the fitting settings, with `NeedsRefresh: true`. Smart Fit Mode, Face Crop, Face Boost and "Download & Frame Mismatched Images" are disabled unless the mode is Smart Crop, because they have no effect otherwise.
- **Tray:** an "Artwork Display" submenu with the three modes and a check mark on the current one. Choosing the current mode does nothing.

### 5.6 Tune Image

- The menu item is shown when Smart Fit is active **or** the mode is not Smart Crop.
- `ShowTuneImagePopup` gains a `lockAnchors` parameter. It is true in the new modes, because there is no crop to anchor.
- The frame toggle works as in section 4. Frame styling controls follow the toggle as they do today.
- Since P26 the frame toggle is replaced by a Full artwork / Museum frame / Smart crop control for the image, shown under every global mode; `lockAnchors` is no longer read by the window, which gets the global mode in `TunePreview.Global`.

## 6. Cost of switching

Each mode already has its own derivative directory, so a switch keeps the files of the mode it leaves. Coming back to a mode reuses them without decoding anything. Only the first visit to a mode has to render.

`Sync` therefore separates two cases when an image's flags do not match:

- **Only `DisplayFull` or `DisplayFramed` differ:** `ImageActionRepoint` forgets the image's paths and keeps its files.
- **Anything else differs:** `ImageActionInvalidate` deletes its derivatives across all directories, as before. Settings such as frame size or wall color are not part of the path, so files from every mode would be stale.

Reused images must not wait in the pipeline queue. The queue has only 4 workers, shared with renders and downloads, and on the real cache only 50 of 228 reusable images were back after 2 minutes. So `rebuildMissingDerivatives` first reattaches every image whose files all exist, which only checks that the files are there, and queues the rest afterwards.

Keeping files raises three issues:

- **Disk use:** each visited mode keeps its own copy of the library's wallpapers. This is accepted, since masters are far larger. Evicting an image still removes all of its files, and changing any other processing setting clears them.
- **Tune Image:** tuning belongs to the image and applies in every mode, so tuning one image deletes its files for that resolution in the other modes. They are regenerated on the next visit.
- **The framed flag:** `VirtualFramed:<WxH>` is only written when rendering. When a file is reused, `VirtualFramer.WouldFrame` supplies the same answer from the image's dimensions, so the Tune Image checkbox stays truthful. It shares its decision with `FitImage`.

Derivatives are also written to a temporary name and renamed into place. A file interrupted mid-write would otherwise be reused for as long as it is kept.

Two more rules protect kept files during rapid switching:

- **A job finishing after a switch must not win.** It would point the image back at the old mode, and nothing would correct it because the image then looks complete. The store remembers the display mode of the last `Sync`, and the pipeline drops results stamped with another one. The rebuild started by that `Sync` supplies the right result.
- **A failed job with no provider deletes nothing.** It used to delete the image's derivatives, which would now wipe every mode's files whenever a switch cancelled a rebuild. Saves are atomic, so a failure leaves no partial file to clean up.

**The existing recovery path is not enough for a switch.** Images only get new derivatives through "backlog healing" in `fetchFromProvider`, and that only sees images on the page being fetched. Pagination has moved on, so the next fetch returns different images, and everything already in the library stays invisible until its page comes round again.

So `RefreshImagesAndPulse` also rebuilds the library from local masters, with no network access:

1. Wait for the asynchronous derivative deletion to finish, or it could delete freshly rebuilt files. Deletion batches now run one at a time and `WaitForDerivativeDeletes` waits for the latest one. `WaitTimeout` is for shutdown only, because a `WaitGroup` must not be waited on while new work can still be added.
2. Submit a provider-less `DownloadJob` for each image that has a master, is missing a derivative for a current monitor, and is not tagged incompatible with it.

A newer switch cancels the rebuild in progress. `ProcessImageJob` normally deletes the master when a job fails, which is right for a partial download but would destroy library images here, so a job with no provider leaves its files alone.

`ImageStore.replace` must also notify waiters. After a switch the monitor finds no images and waits for a store update, but re-processed images used to arrive silently, so the wallpaper stayed on the old image. Previously a newly fetched image usually woke the monitor by chance; a local rebuild has no new images.

`MonitorController.next` must also discard a stale shuffle deck. It only rebuilt the deck when its length differed from the resolution bucket. After a switch the deck can hold the same number of IDs as the partly rebuilt bucket but different images, so the monitor picked an image with no derivative, reported its file missing, and kept the old wallpaper. Switching twice before the next rotation made this likely, because the deck is still small from the first switch. The deck is now rebuilt if it contains any ID that is not in the bucket.

Rebuild jobs must also carry their own copy of the image's maps. `store.List()` copies images shallowly, and `ProcessImageJob` writes to `ProcessingFlags`, so a shared map would race with the store's background save, which can crash the app.

This also improves every other setting that uses the refresh path, such as Face Crop.

A first visit is limited by the pipeline's worker pool, so a large library takes a noticeable time (see section 7.2).

## 7. Test plan

| Area | Test |
| :--- | :--- |
| Decision table | Every cell in section 4, asserting whether `next.FitImage` is called and whether the framed flag is set |
| Full render | Output is exactly the target size; art is uncropped for portrait, landscape, matching and upscaled images; bars use the wall colour |
| Compatibility | Portrait on landscape returns `nil` in the new modes; Smart Crop results are unchanged |
| Config | The mode survives a save and load round trip; unknown values read as Smart Crop; the default is Full Artwork when nothing is saved or the saved config has no mode; a saved Smart Crop is kept |
| Derivative dirs | Each mode returns the expected path, including with Smart Fit Disabled |
| Flags | Images from before this feature count as Smart Crop; upgrading rebuilds the library at startup without a refresh; switching mode invalidates; a framed image stays valid across repeated `Sync()`; a new download matches the sync target |
| Retention | A display-mode change keeps files and renders nothing on return; any other setting change deletes them; tuning removes the same-resolution files of other modes only, recognising the current file through a junction path; reused images report the right framed flag; interrupted writes leave no file |
| Rebuild | Invalidated images with a master are resubmitted without a provider; images that are complete, incompatible or missing a master are not |
| Tray | Three entries, correct check mark, Tune Image visibility |
| i18n | `make check-i18n` passes |

`artwork_display_e2e_test.go` runs the real plugin, pipeline, monitor actors, store and image processors over a temp directory, with a recording stand-in for the desktop and any network request failing the test. It covers switching through every mode on two monitors, rapid switching, restart and Tune Image.

Then run `go test ./...`, `gofmt`, and a Windows build, and verify in the real app that the tray menu appears and switching changes the desktop wallpaper in all three modes.

### 7.1 Off-screen verification results

Run with other sessions using the desktop, so nothing touched the real wallpaper, cache or settings. Timings are from 24 copies of real masters (54 MB) at 2560x1600, using the pipeline's default limit of 4 workers:

| Switch to | Wallpaper changed after | All 24 rebuilt after |
| :--- | :--- | :--- |
| Full Artwork | 0.9 s | 6.1 s |
| Museum Frame | 1.4 s | 11.4 s |
| Smart Crop | 1.1 s | 7.0 s |

These tests use a temp directory, which is why they missed the junction failure in section 5.3.

### 7.2 Real-app verification results

Run on the real desktop and the real cache (about 300 images, large masters, 2560x1600, 4 workers). The mode was changed in the saved configuration followed by a restart, because the automation could not operate Spice's own tray menu.

| Check | Result |
| :--- | :--- |
| Full Artwork on the desktop | Correct: the whole painting on the wall color |
| Museum Frame on the desktop | Correct: the whole painting in the frame |
| Wallpaper changed after restart | About 9 s |
| Library rebuilt in Full Artwork | 302 of 302; about 98% within 14 minutes, the last few by 21 minutes |
| Masters and store after the rebuild | Matched exactly, none lost |
| Directories on the junctioned cache | Created on the target, no errors |

Rebuilding is CPU-bound: about 3.8 cores were busy while the disk stayed nearly idle, and throughput was roughly 20 to 30 images a minute. This is far slower than section 7.1 suggests because real masters are much larger. Until a rebuild finishes, rotation only draws from images already rebuilt.

After files were kept per mode, returning to a visited mode on the same cache took:

| Returning to | Images reused | All usable after |
| :--- | :--- | :--- |
| Smart Crop | 251 | 9.0 s |
| Full Artwork | 63 | 9.3 s |
| Smart Crop | 252 | 6.2 s |

These times start when the app is launched, because the mode was changed by restarting, and the wallpaper changed about 5 s in on every run. A switch from the tray skips the startup but was not timed. Each mode's wallpapers took about 1 MB per image, so about 300 MB for this library. A first visit is unchanged.

**Not covered:** clicking the native Windows tray menu. Its entries, check mark and actions are covered by tests, but the menu itself still needs one manual try.

`-race` also reported two races in older code, now fixed. `SmartImageProcessor.lastStats` was written by concurrent workers, and it is now behind a mutex. `StartNightlyRefresh` read `stopNightlyRefresh` without a lock on every loop while `Deactivate` set it to nil, so the goroutine could end up waiting on a nil channel forever. It now keeps its own reference.

## 8. Documentation to update

- `docs/user_guide.md`: the new setting, the tray entry and Tune Image behaviour.
- `docs/image_pipeline_deep_dive.md`: derivative tree, metadata flags and framer rules.
- `docs/architecture.md`: the Smart Fit pipeline section.
