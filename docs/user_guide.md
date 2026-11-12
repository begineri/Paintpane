# Spice User Guide

**Spice** is a premium wallpaper manager for Windows and macOS that keeps your desktop fresh, dynamic, and beautiful. It runs in the background as a system tray application — no windows to manage, just great wallpapers.

---

## Table of Contents
1. [The System Tray](#the-system-tray)
2. [Keyboard Shortcuts](#keyboard-shortcuts)
3. [Preferences: App Tab](#preferences-app-tab)
4. [Preferences: Wallpaper Tab](#preferences-wallpaper-tab)
   - [General](#general-settings)
   - [Smart Crop Anchors](#smart-crop-anchors)
   - [Online Sources](#online-sources)
   - [Local Sources](#local-sources)
     - [Local Favorites](#local-favorites)
5. [Multi-Display Setup](#multi-display-setup)
6. [Browser Extensions](#browser-extensions)

---

## The System Tray

Spice lives entirely in your system tray (Windows) or menu bar (macOS). Click the Spice icon to open the menu.

### Single Display Menu

When one display is connected, the tray menu shows actions directly:

| Item | Description |
| :--- | :--- |
| **Next Wallpaper** | Immediately advance to the next wallpaper |
| **Prev Wallpaper** | Go back to the previous wallpaper |
| **Shuffle** | Reshuffle the wallpaper rotation order on this display |
| **Pause Play** | Pause automatic rotation (changes to **Resume Play** when paused) |
| *(separator)* | |
| **Source:** | Shows the image provider (e.g., *Wallhaven*, *Met Museum*) |
| **By:** | Shows attribution — click to open the original image on the web |
| **Add to Favorites** | Save the current wallpaper locally *(only visible when Favorites is enabled)* |
| **Tune Image** | Open the Tune Image popup to manually toggle the Virtual Museum Frame or fine-tune crop anchors *(only visible when Smart Fit is active or the Artwork Display mode is not Smart Crop)* |
| **Artwork Display** | Switch between Smart Crop, Full Artwork and Museum Frame in one click. The current mode is checked. See [Artwork Display Modes](#artwork-display-modes) |
| **Delete And Block** | Delete the image from cache and prevent it from ever appearing again |
| *(separator)* | |
| **Preferences** | Open the settings window |
| **About Spice** | Version info and credits |
| **Quit** | Exit Spice completely |

### Multiple Displays

When more than one display is connected, Display 1 (the primary) keeps its controls at the **top level** of the menu. Every additional display gets its own **submenu** named *Display 2*, *Display 3*, etc. (with the device name appended if available, e.g., *Display 2 (DELL U2723D)*).

Each display submenu contains the same full set of controls — Next, Prev, Pause Play, Source, Attribution, Add to Favorites, and Delete And Block — all acting independently on that specific display.

> **Tip:** When Spice starts with multiple monitors, it staggers wallpaper changes by a random offset so they don't all flash at the same time (configurable in Settings).

---

## Keyboard Shortcuts
 
Spice is designed for power users who want total control without interrupting their flow. Our global hotkeys are instant, responsive, and cross-platform.

**Why it's cool:**
- **Zero-Touch Control**: Advance your wallpapers or save a favorite without ever leaving your IDE, browser, or game.
- **The "Magic Number" System**: Our unique targeting system lets you control specific monitors independently by simply holding a number key (1-9).
- **Universal Muscle Memory**: Whether you're on Windows or macOS, the shortcut logic remains consistent and intuitive.

> **Note for macOS:** Global actions use `Command` + `Control` (similar to Windows `Ctrl` + `Alt`). Targeted actions use `Command` (similar to Windows `Alt`).

### Global Actions — All Displays At Once
These shortcuts perform actions across every connected monitor simultaneously.

| Action | Windows | macOS |
| :--- | :--- | :--- |
| **Next Wallpaper** | `Ctrl` + `Alt` + `→` | `Cmd` + `Ctrl` + `→` |
| **Previous Wallpaper** | `Ctrl` + `Alt` + `←` | `Cmd` + `Ctrl` + `←` |
| **Shuffle (All Displays)** | `Ctrl` + `Alt` + `R` | `Cmd` + `Ctrl` + `R` |
| **Info (Primary Display)** | `Ctrl` + `Alt` + `I` | `Cmd` + `Ctrl` + `I` |
| **Sync / Detect Displays** | `Ctrl` + `Alt` + `D` | `Cmd` + `Ctrl` + `D` |
| **Open Preferences** | `Ctrl` + `Alt` + `O` | `Cmd` + `Ctrl` + `O` |

### Single-display Actions — Precision Monitor Control
Hold the display's **number key (1–9)** together with `Alt` (Windows) or `Command` (macOS) to act on one display only: Display 1 = key `1`, Display 2 = key `2`, and so on. They are controlled by **Single-display shortcuts (hold Alt + display number)** in Settings → General, which is off by default and only works while **Global shortcuts** is on. **View all shortcuts** on the same page opens a Shortcuts page that lists every key.

| Action | Windows | macOS |
| :--- | :--- | :--- |
| Next on Display *N* | `Alt` + `N` + `→` | `Command` + `N` + `→` |
| Previous on Display *N* | `Alt` + `N` + `←` | `Command` + `N` + `←` |
| **Block / Delete** | `Alt` + `N` + `↓` | `Command` + `N` + `↓` |
| **Add to Favorites** | `Alt` + `N` + `↑` | `Command` + `N` + `↑` |
| **Pause / Play** | `Alt` + `N` + `P` | `Command` + `N` + `P` |
| **Shuffle** | `Alt` + `N` + `R` | `Command` + `N` + `R` |
| **Info** | `Alt` + `N` + `I` | `Command` + `N` + `I` |
| **Keep the top** (tall picture) | `Alt` + `N` + `W` | `Command` + `N` + `W` |
| **Keep the left** (wide picture) | `Alt` + `N` + `A` | `Command` + `N` + `A` |
| **Keep the bottom** (tall picture) | `Alt` + `N` + `S` | `Command` + `N` + `S` |
| **Keep the right** (wide picture) | `Alt` + `N` + `D` | `Command` + `N` + `D` |
| **Keep the middle** | `Alt` + `N` + `E` | `Command` + `N` + `E` |
| **Back to automatic** | `Alt` + `N` + `Q` | `Command` + `N` + `Q` |

**Example:** To skip the wallpaper on your second monitor, press and hold `Alt` (Windows) or `Command` (macOS), tap `2`, and press `→`. 

### Customizing & Disabling Shortcuts
 
Spice gives you granular control over how hotkeys interact with your system. If you find that Spice shortcuts conflict with other software (like your IDE or web browser), you can turn them off in **Settings → General → Shortcuts**:

**Global shortcuts** is the master switch for all keyboard hotkeys, and it is on by default. It completely unregisters the app from your system's input loop; use it if you prefer using the tray menu exclusively. **Single-display shortcuts (hold Alt + display number)** controls the keys that act on one display (e.g., `Alt` + `1` + `→`); it is off by default and has no effect while Global shortcuts is off. Those keys only work while a number key is held, so plain `Alt` + `Arrow` in your browser ("Go Back/Forward") is not taken.

> **macOS Permissions**: Targeted shortcuts require Accessibility permissions. macOS will prompt you once when Spice first detects a number key press. If shortcuts stop working, ensure Spice is still enabled in **System Settings → Privacy & Security → Accessibility**.

> **Mac App Store Limitation**: Targeted (per-monitor) shortcuts are **not available** in the Mac App Store version of Spice. This is a platform restriction — the App Sandbox prevents Spice from detecting which number key is held simultaneously. Global shortcuts (`Cmd` + `Ctrl` + `Arrow`) work normally. If you need per-monitor control, use the tray menu submenus or download the DMG version from [GitHub](https://github.com/dixieflatline76/Spice/releases).

---

## Preferences: App Tab

Open via **Tray → Preferences** or `Ctrl` + `Alt` (Windows) / `Cmd` + `Control` (macOS) + `O`.

The **App** tab controls application-wide behaviour, independent of any wallpaper source.

> **Managing Startup (Windows & macOS):**
> * **Windows**: Turn **Start with Windows** on or off in General › Running. It is on when Paintpane has registered itself to start when you sign in. It is on by default: the first time you accept the licence agreement, Paintpane registers itself once. After that only the switch decides; turn it off and Paintpane never registers itself again, not even after an update. If you accepted the agreement of an earlier version, an update does not turn it on. If you installed Paintpane with the installer, its own **Start with Windows** option (ticked by default) decides instead. If you switch Paintpane off in Windows' own Startup apps page, the switch reads off and stays off, and turning it on switches it back on. After a start at sign-in, Paintpane waits one to two minutes before it downloads or processes anything.
> * **macOS**: Click **Manage in macOS Settings** to open System Settings Login Items (`LoginItems-Settings.extension`) to add or remove Spice.

| Setting | Description |
| :--- | :--- |
| **Start with Windows** (Windows) | Switch: start Paintpane in the background when you sign in. On by default (set once, when you first accept the licence agreement). On macOS the row is **Start at Login**, a button that opens System Settings. |
| **Enable System Notifications** | Toggle desktop toast notifications (e.g., "Paused Play", "Next Wallpaper"). Useful to turn off if they become distracting. |
| **Enable New Version Check** | Spice checks for updates once on startup and once per day. A tray indicator appears when a newer version is available. |
| **Enable global shortcuts** | Master switch for all keyboard hotkeys. Disable if the shortcuts conflict with another application. |
| **Theme** | Choose between *System* (follows OS dark/light mode), *Dark*, or *Light*. Changes apply immediately. |
| **Language** | Select between 12 supported languages (English, German, Spanish, French, Italian, Portuguese, Japanese, Russian, Ukrainian, Chinese Simplified, or Chinese Traditional). Choosing **System Default** follows your OS language. |

---

## Preferences: Wallpaper Tab

The **Wallpaper** plugin tab appears immediately after the App tab. It contains four sub-sections accessed via a side navigation bar: **General**, **Online**, **Local**, and **AI**.

### General Settings

| Setting | Description |
| :--- | :--- |
| **Wallpaper Change Frequency** | A flexible freeform minutes input that determines how often Spice automatically rotates. Type `15` for 15 minutes, or `1440` for daily. Set to `0` to disable automatic rotation entirely (you can still change manually via the tray or hotkeys). |
| **Cache size** | How many images to keep on disk: 100, 150 (the default), 300, 500, 1000, 5000 or 10000. A larger cache means more variety and fewer network requests, and more disk space. When the cache is full at start, Paintpane does not ask the sources for more. |
| **Artwork Display** | Chooses whether images are cropped to fill the screen or always shown whole — see [Artwork Display Modes](#artwork-display-modes). |
| **Smart Fit Mode** | Controls how Spice fits images to your screen — see below. Only applies when Artwork Display is set to *Smart Crop*. |
| **Enable Face Crop** | When Smart Fit is active, the cropper aggressively centers on the largest detected face. **Note**: This setting is automatically disabled if Smart Fit Mode is "Disabled". |
| **Enable Face Boost** | When Smart Fit is active, the cropper *hints* toward faces but also considers overall composition. **Note**: This setting is automatically disabled if Smart Fit Mode is "Disabled". |
| **Stagger monitor changes** | Adds a small random delay between wallpaper changes on each display during automatic rotation, preventing a distracting simultaneous flash across all screens. |
| **Museum Collection OTA** | Over-the-Air updates for museum collections. If enabled, occasionally synchronizes curation files from the cloud to receive new curated collections without updating the app. |
| **Refresh wallpapers nightly** | Spice quietly re-fetches images from all active sources once per night, keeping the cache fresh with new content without interrupting your day. |
| **Advanced → Behavior → Detect displays** | Manually tell Spice to re-detect all connected monitors. Use this if you plug in or unplug a display while Spice is running. |
| **Clear Wallpaper Cache** | Deletes all downloaded images from disk. You will need an internet connection before new wallpapers appear again. Requires confirmation. |
| **Blocked Images → Reset** | Clears the list of blocked images, allowing previously deleted images to be re-downloaded. Requires confirmation. |

#### Smart Fit Feature Matrix

The following table breaks down exactly how the engine processes images and controls the Tune Image UI depending on your chosen mode:

| Feature / Processing Stage | Quality Mode | Flexibility Mode |
| :--- | :--- | :--- |
| **Core Philosophy** | **Pristine Fit:** "Protect me from bad crops at all costs. Skip images if they don't fit perfectly." | **Maximum Variety:** "I accept aggressive crops and missing details to see more variety." |
| **Aspect Ratio Limits** | **Strict:** Rejects images unless they closely match your monitor's physical aspect ratio. | **Dynamic / Loose:** Accepts wide mismatches (e.g. a square image on an ultrawide monitor). |
| **Orientation Safety** | **Strict:** Will immediately reject portrait images on a landscape monitor (unless rescued by Museum Frame). | **Loose:** Will attempt to crop a portrait image to fit a landscape monitor (resulting in an extreme "slice"). |
| **Face Detection Crop** | **Conservative:** Will shift the crop slightly to keep a face in view, but refuses to ruin the composition. | **Aggressive:** Will ruthlessly crop out the rest of the image just to center the primary face. |
| **Entropy / Saliency Crop** | **Minimal:** Uses edge-detection just to avoid cropping out the most interesting 10% of the edges. | **Maximum:** Will scan the entire image for energy (complexity) and heavily crop to focus on the busiest region. |
| **Auto-Salvage (Framing)** | **High Frequency:** Many images fail the strict check and are automatically wrapped in a Museum Frame to prevent skipping them. | **Low Frequency:** Because limits are loose, images rarely fail, so auto-framing almost never triggers. |
| **Museum Mode Auto-Frame** | Applies to all museum art regardless of aspect ratio. | Applies to all museum art regardless of aspect ratio. |
| **Tune Image: Unframe** | **Contextual Lock:** You *cannot* remove the frame from an image if cropping it would violate your strict Quality limit. The checkbox is disabled. | **Always Unlocked:** You can rip the frame off *any* image and let the system aggressively chop it to pieces. |
| **Tune Image: Frame Styling**| **Unlocked:** You can adjust the frame size, paper matting, and wall color (even if the frame toggle is locked). | **Unlocked:** You can adjust all frame properties. |
| **Tune Image: Manual Anchor**| Adjusts the crop within safe, minor boundaries. | Allows extreme panning across the heavily cropped image (e.g. pan to the bottom of a portrait). |
| **Processing Speed** | **Fastest:** Instantly skips incompatible images before doing heavy processing. | **Slower:** Performs heavy math (entropy/face scanning) on almost every image to find a viable crop. |

---

### Artwork Display Modes

Artwork Display decides how every image is presented on your screen. Change it in Preferences or from the **Artwork Display** submenu in the tray.

| Mode | What you see |
| :--- | :--- |
| **Smart Crop** | The image fills the screen. Part of it is cropped; Spice never frames an image in this mode. Smart Fit settings apply. |
| **Full Artwork** (default) | The whole image, as large as it fits, on a plain wall color. Nothing is cropped. |
| **Museum Frame** | The whole image in a gallery frame on the wall. Nothing is cropped. Frame size, matting and wall color settings apply. |

Things to know:

- **The first switch to a mode takes a while.** Spice creates that mode's wallpapers from the originals in the background, which can take several minutes with a large cache. The wallpaper changes as soon as the first one is ready, and rotation uses the ones finished so far.
- **Switching back is quick.** Spice keeps each mode's wallpapers, so returning to a mode reuses them. This uses more disk space. Changing another processing setting, such as frame size or Face Crop, clears them.
- **Smart Fit settings do not apply** in Full Artwork and Museum Frame, so they are disabled in Preferences. This includes when Smart Fit Mode is *Disabled*.
- **Tune Image still works per image.** In Full Artwork and Museum Frame, **Add frame** puts that image in a frame, or takes it off, and nothing is cropped. In Smart Crop, Tune Image only chooses which part is kept.
- **Wall Color** also sets the background in Full Artwork.
- **Low-resolution images** are enlarged to fit and may look soft.

---

### Tune Image

**Tune Image** (from the tray menu, one display at a time) changes how a single picture is shown. The preview is always the shape of that screen and shows the desktop result.

**Smart Crop: choose the part to keep.** Drag the preview along the direction that is cropped: up and down for a picture taller than the screen, left and right for a wider one. The picture follows the pointer and stays where you let go, at any position. A picture close to the screen's shape can be dragged too, only a little; one with exactly the screen's shape has nothing to drag. After a drag, **Reset** returns to the automatic crop. There is no **Add frame** button in this mode, because Smart Crop never frames.

**Full Artwork and Museum Frame: Add frame.** The **Add frame** button puts this one picture in a gallery frame, or takes it off. Once framed, you can set the **Frame size**, the **Paper mat** and the **Wall color** for it. The paper mat is off by default; whatever you chose earlier is kept.

**Keyboard Shortcuts (WASD Layout):**
In Smart Crop you can also place the kept part from the keyboard, with the single-display pattern: hold the display's number key (1-9), with `Alt` (Windows) or `Command` (macOS).

| Key | Action |
| :--- | :--- |
| `W` | Keep the top (tall picture) |
| `A` | Keep the left (wide picture) |
| `E` | Keep the middle |
| `S` | Keep the bottom (tall picture) |
| `D` | Keep the right (wide picture) |
| `Q` | Back to automatic |

**Example:** To keep the top of the picture on Display 2, press and hold `Alt` (Windows) or `Command` (macOS), tap `2`, and press `W`. These keys set the kept part to the start, middle or end (0, 50 or 100 percent); a position you dragged to is replaced.

> **Note:** Tune Image needs **Smart Fit Mode** to be *Quality* or *Flexibility* for cropping. When Smart Fit is *Disabled*, images are not processed.

--- | :--- |
| `W` | Tune Up ↑ |
| `A` | Tune Left ← |
| `S` | Tune Down ↓ |
| `D` | Tune Right → |
| `E` | Tune Center ● |
| `Q` | Auto (pipeline default) |

**Example:** To anchor the wallpaper on Display 2 to the top of the image, press and hold `Alt` (Windows) or `Command` (macOS), tap `2`, and press `W`.

> **Note:** Tune Image features require **Smart Fit Mode** to be set to either *Quality* or *Flexibility*. When Smart Fit is *Disabled*, images are not processed.

---

### Online Sources

The **Online** tab lists each cloud and institutional image provider as an expandable accordion card. Click a provider name to expand its settings.

#### Wallhaven
 
Wallhaven is a premier destination for high-quality wallpapers. Spice integrates deeply with the Wallhaven API to provide a seamless search and synchronization experience.

**Authentication:**
Entering your API Key is highly recommended as it enables access to your private favorite collections and higher search quotas.
- **Verification**: Paste your key and click **Verify & Connect**. Spice performs an immediate live check.
- **Security**: Once verified, the key is masked (dots) and permanently **locked**.
- **Change/Remove**: To update the key, you must use the **Clear API Key** button, which performs a [Full Reset](#account-reset).

**Favorites Synchronization:**
This is the "killer feature" for Wallhaven power users. Instead of manually adding individual search URLs, Spice can mirror your entire Wallhaven account.
- **Setting it up**:
  1. Enter your **Wallhaven Username**.
  2. Click **Verify Username**. (Note: An API Key must be verified first).
  3. Once verified, the **Keep Favorites Synced** checkbox will become available.
- **The Magic**: When "Keep Favorites Synced" is enabled, Spice will automatically discover all your public favorite collections and add them as **Managed** queries.
- **Live Sync**: As you add or remove favorite collections on the Wallhaven website, Spice will detect the changes and automatically update its source list to match.

**Adding Manual Queries:**
If you want to track a specific search that isn't in your favorites:
1. Go to [wallhaven.cc](https://wallhaven.cc) and search for a topic.
2. Copy the browser URL (e.g., `https://wallhaven.cc/search?q=nature&categories=110`).
3. Back in Spice, click **Add Wallhaven URL**, paste the link, and give it a descriptive name.

<a name="account-reset"></a>
**Account Reset:**
Clicking **Clear API Key** is a destructive but necessary action if you wish to change accounts. It will:
1. Clear the stored API Key and Username.
2. Disable the synchronization engine.
3. **Remove all Managed collections** from your list to prevent data orphans.
*Note: Your manually added queries are left untouched.*

#### Pexels
 
Pexels is a high-quality stock photography provider known for its vibrant, modern imagery. Spice leverages the Pexels API to bring these professional photos directly to your desktop.

**How to Use:**
- **Verification**: Paste your free API key and click **Verify & Connect**. Like Wallhaven, Spice ensures the key is active before locking it for security.
- **Adding Queries**: Use the **Add Pexels Search** button to paste a URL from the [Pexels website](https://pexels.com). You can track specific search terms (e.g., "Minimalist Interiors") or follow hand-picked collections from top photographers.

#### Wikimedia Commons
 
Wikimedia Commons is a vast, dynamic repository of freely-licensed media from millions of contributors. Unlike static museum collections, Commons allows you to tap into a live stream of real-world history and discovery.

**The Power of Discovery:**
- **Search**: Use `search:nature` or paste a MediaSearch URL to fetch images matching specific topics.
- **Category Power**: Follow deep categories like `category:Deep space` or `category:Impressionist paintings` for a focused rotation.
- **Specific Files**: Want to stick with a single masterpiece? Enter a direct File URL (e.g., `file:File:Earth_Eastern_Hemisphere.jpg`).

> **Tip:** Support the community! You can contribute to their mission via the **Donate to Wikimedia** link in the provider settings.

#### Museum Sources (Online)
 
Art has no borders. Spice integrates with the world’s leading cultural institutions to transform your screen into a rotating gallery of historical masterpieces, all provided under Open Access (CC0) licenses.

**Available Museums:**
- **Metropolitan Museum of Art** (New York City, USA)
- **Art Institute of Chicago** (Chicago, IL, USA)
- **Cleveland Museum of Art** (Cleveland, OH, USA)
- **Rijksmuseum** (Amsterdam, Netherlands)
- **Statens Museum for Kunst** (Copenhagen, Denmark)
- **National Palace Museum** (Taipei, Taiwan)
- **J. Paul Getty Museum** (Los Angeles, CA, USA)

**The "Director's Cut" Collections:**
Each museum provides curated collections designed to showcase institutional highlights:
- **Arts of Asia**: From ancient ceramics to modern prints.
- **The Impressionists**: Iconic works from Monet, Degas, and Renoir.
- **High Resolution Art**: Masterpieces chosen for their exceptional detail and scale.

**How to Use:**
1. Open **Preferences → Wallpaper → Online**.
2. Expand a museum card.
3. Use the **Map** link to "Plan a Visit" and explore the institution's location.
4. (Optional) Check **"Download & Frame Mismatched Images"** to automatically salvage extreme portraits/landscapes for this specific museum by wrapping them in the Virtual Museum Frame.
5. Toggle the collections you want and click **Apply**.

#### Google Photos
 
Google Photos is for your personal memories. Spice uses the **Google Photos Picker API**, ensuring a high-privacy integration.

**Why it's cool:**
- **Manual Picker**: Instead of giving Spice access to your entire library, the browser-based Picker flow lets you hand-select exactly which albums or media items are available.
- **Privacy First**: Spice never sees your credentials; authorization is handled securely through the official Google Auth flow in your default browser.
- **High Resolution**: Images are downloaded at their original resolution (where supported) to ensure your memories look stunning on high-DPI displays.

**How to Use:**
1. Open **Preferences → Wallpaper → Online → Google Photos**. (Note: May require beta allowlisting).
2. Click **Connect to Google Photos**.
3. In your browser, select the albums or specific photos you want to use.
4. Back in Spice, toggle your Google Photos source and click **Apply**.

---

### Local Sources
 
Sometimes the best wallpapers are the ones you already own or have carefully curated into your favorites. These can be managed in the **Local** tab of the Spice Preferences.

#### Local Favorites
The "Favorites" provider is the heart of Spice's localized content. It acts as a persistent archive of images you've "loved" from other providers.

**The "Killer" Features:**
- **Persistent Storage**: When you favorite an image (via the Tray Menu or Keyboard Shortcut), Spice downloads a permanent copy to your local storage. Even if the original collection on Wallhaven or Pexels is deleted, your favorite remains.
- **Auto-Syncing**: Your Favorites source is active by default. As soon as you "heart" an image, it's added to your rotation instantly.
- **Deep Integration**: Favorited images preserve their original metadata, so you can still use the "Open in Browser" feature to visit the original source.

**How to Use:**
- **Adding**: Click the Heart icon in the tray menu or use `Alt` + `N` + `↑` (Windows) / `Command` + `N` + `↑` (macOS).
- **Managing**: Go to the **Local** tab to see your library. You can toggle your favorites on/off or clear your entire collection using the **Clear** button.
- **Quick-Access**: Click the **"❤️ Personal Favorites"** name in the list to immediately open your local favorites folder in the OS file explorer.

#### Local Folders
Want to use your own photography or personal collection?
1. Open **Preferences → Wallpaper → Local**.
2. Click **Add Folder** to select a directory on your computer.
   - **Note for Windows Users:** Due to OS limitations, the folder picker requires you to select a specific file. Navigate to your desired folder, click on **any image file** inside it, and click "Open". Spice will automatically add the entire folder containing that image to your rotation, not just the single file.
3. Spice will search for high-resolution images (`.jpg`, `.png`, `.webp`) and add them to your rotation.
4. **Tip**: Click the folder name in the list to open that directory directly in your file explorer.

---

## Multi-Display Setup

Spice fully supports any number of connected monitors. Each display is managed independently.

### Automatic Detection

When Spice starts, it detects all connected displays automatically. If you connect or disconnect a display while Spice is running:

1. Open **Settings → Advanced → Detect displays**, or
2. Use the hotkey `Ctrl` + `Alt` + `D` (Windows) / `Cmd` + `Control` + `D` (macOS).

### Per-Display Control

- **Each display gets its own wallpaper queue.** Spice fetches from your active sources and distributes images across monitors.
- **Pausing is per-display.** You can pause Display 2 while Display 1 keeps rotating.
- **Favorites and Delete are per-display.** The action always applies to the wallpaper currently shown on the targeted display.

### Tray Menu with Multiple Displays

- **Display 1** controls appear at the **top level** of the tray menu (no submenu needed).
- **Display 2, 3, …** each have their own **submenu** with identical controls.

### Stagger Setting

With the **Stagger monitor changes** option enabled (recommended for 2+ displays), Spice introduces a random delay between each display's automatic rotation. This prevents all screens from flashing at the same moment, creating a more pleasant, natural feel.

---

## Browser Extensions
 
The Spice Browser Extension is the ultimate companion for discovery. It bridges the gap between your web browser and your desktop, allowing you to sync new collections with zero configuration.

### Why it's cool:
- **LiveSync Discovery**: The extension is "alive." As you browse supported sites like Wallhaven or Pexels, it silently scans the page for compatible search and collection URLs.
- **The Pulsing Signal**: When a valid source is found, the Spice icon in your toolbar will **pulse green**. This is your immediate visual signal that a "Masterpiece Collection" has been detected.
- **One-Click Synchronization**: Clicking the extension opens a small popup where you can instantly add the discovered URL as a new Spice query. No copying, pasting, or manual setup required.
- **Zero Privacy Sacrifice**: The extension communicates with Spice over a secure local WebSocket (`localhost`). Your browsing habits and selected images are never sent to a cloud server.

### Installation
Spice is available for all major desktop browsers:
- **Chrome / Edge / Brave**: Install from the [Chrome Web Store](https://chromewebstore.google.com).
- **Firefox**: Install from [Firefox AMO](https://addons.mozilla.org).
- **Safari**: Included with the Spice macOS app. Activate it in **Safari → Settings → Extensions**.

### Supported Sites
The extension is pre-tuned for high-resolution discovery on:
- **Wallhaven**: Search results, top lists, and personal favorites.
- **Pexels**: Curated collections and modern photography searches.
- **Wikimedia Commons**: Specific categories and MediaSearch topics.

> **Pro Tip:** Keep Spice running in your system tray! The extension needs the desktop app to be open to receive its synchronization signals.
