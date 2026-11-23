# Fyne 2.7.4, patched for Paintpane

This is `fyne.io/fyne/v2` v2.7.4 as published (`go mod download fyne.io/fyne/v2@v2.7.4`), used
through the `replace` in the top-level `go.mod`. Fyne's licence (BSD 3-Clause) is in `LICENSE`.

Left out, because Paintpane does not build them: `*_test.go`, `testdata/`, `cmd/`, `tools/`,
`.github/`, `.gitignore`, `staticcheck.conf` and the contributor documents (`CHANGELOG.md`,
`CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, `SECURITY.md`). `internal/build/` is kept in git by an
exception in the top-level `.gitignore`, whose `build/` rule would otherwise drop it.

Every change is marked `PAINTPANE:` in the code: `grep -rn PAINTPANE: third_party/fyne` lists
them all. Nothing else differs from the published module.

## Changes

### 1. The event loop slows down while no window is visible

| File | Where | Change |
| :--- | :--- | :--- |
| `internal/driver/glfw/loop.go` | `runGL` | Keeps an `eventPace` next to the event ticker. It calls `pace.update(d.windows)` after every frame, and after every queued function while the loop is slow. |
| `internal/driver/glfw/loop.go` | new `idleEventTick`, `eventPace`, `eventPace.update` | Ticker at 60 Hz while any window is visible; `idleEventTick` (4 Hz) otherwise. |
| `internal/driver/glfw/loop_pace_test.go` | new file | Tests for the above. Run with `cd third_party/fyne && go test ./internal/driver/glfw/ -run Pace`; `go test ./...` at the top level does not reach this module. |

**Why:** Upstream polls GLFW events 60 times a second all the time. In a tray app with no window
open that woke the locked main thread 60 times a second all day: about 0.4% of a core, the
largest idle cost.

**Measured:** side-by-side release builds, full library, 10 minutes. Idle CPU fell from 0.51% to
0.10% of a core, and to 0.10–0.11% together with the display check in the app (2026-11-21).
Settings window opening time and memory were unchanged.

**Why it is safe:**
- Work queued from other goroutines (`fyne.Do`, `Show`) is still run at once.
- The loop switches back to 60 Hz as soon as a window is visible.
- Hidden helper windows still get their events four times a second: close requests, resizes and
  the render cache keep-alive walk.

### 2. The hidden systray monitor window has no OpenGL context

| File | Where | Change |
| :--- | :--- | :--- |
| `internal/driver/glfw/window_desktop.go` | `type window` | New field `noGL`. |
| `internal/driver/glfw/window_desktop.go` | `create` | For a `noGL` window: GLFW hint `ClientAPI = NoAPI`, and the hint is set back to `OpenGLAPI` afterwards. It gets a painter that is never initialised, so `Close` can range over its (no) textures. |
| `internal/driver/glfw/window.go` | `RunWithContext` | Returns at once for a `noGL` window, which has no context to make current. |
| `internal/driver/glfw/driver_desktop.go` | `runSystray` | Marks the `SystrayMonitor` window `noGL`. |

**Why:** Fyne creates the hidden `SystrayMonitor` window only to learn when the app is asked to
quit. It never draws, but its OpenGL context loaded the GPU driver (here NVIDIA's `nvoglv64.dll`)
for the life of the app.

**Measured:** side-by-side release builds, full library (2026-11-21).

| | Before | After |
| :--- | ---: | ---: |
| Private memory while no window had been opened | 260–266 MB | 72–81 MB |
| Private working set | 111–113 MB | 16–18 MB |
| Threads | 67–71 | 24–29 |
| Handles | about 2,000 | about 600 |
| After a window has been opened and closed (the driver stays loaded) | 283–290 MB | 194–196 MB |

**Cost, accepted by the owner on 2026-11-22:** with no context left, the driver initialises again
for every new window. Dev builds, measured 2026-11-22 against the build without this change:

| Window | Before | After |
| :--- | ---: | ---: |
| Settings, first open | 0.52 s | 0.74–0.80 s |
| Settings, opened again | 0.26–0.31 s | 0.38–0.55 s |
| Tune Image | 0.49–0.56 s | 0.59–0.70 s |

Two outliers of the build without the change (3.6 s and 1.9 s) are left out of the "Before" column.
The tray menu was not measured, because it takes the focus. Its windows are created again a minute
after it closes, so it is expected to take about as much longer.

**Why it is safe:** Every GL path in the GLFW driver either runs only for visible windows
(`repaintWindow`, `doShow`) or goes through `RunWithContext`. The monitor window is never shown.

## Updating Fyne

1. `go mod download fyne.io/fyne/v2@<new>` and replace this folder with the new module, leaving
   out the same files.
2. Carry over each change listed above (search the old copy for `PAINTPANE:`). Drop a change if
   upstream now does the same: polling only while a window is visible, or a systray window
   without a context.
3. Update the version in the top-level `go.mod` `require` line and in this file. Then measure idle
   CPU and memory again, side by side with the previous build.
