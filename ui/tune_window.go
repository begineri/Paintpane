package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/sysinfo"
	spiceui "github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// Preview area of the Tune Image window, in logical units.
const (
	tunePreviewMaxW float32 = 360
	tunePreviewMaxH float32 = 230

	tunePadding float32 = 12 // around the window body
	tuneGap     float32 = 10 // between preview, mode switch, notes and cards

	// The preview is always the shape of the screen and shows the desktop result (J2, J3), so it
	// does not change size when a frame is added. It is rendered at twice its size so it stays
	// sharp on HiDPI screens.
	tuneRenderScale = 2

	// The picture the preview is made from is decoded at most this big.
	tuneSourceMax = 1000

	tunePreviewRadius float32 = 6
)

// tuneRequest is everything the Tune Image window needs.
type tuneRequest struct {
	previewPath string
	opts        provider.TuningOptions // effective options: the starting state
	screenW     int                    // the display's size in pixels; zero means 16:9
	screenH     int
	// render draws the picture as the wallpaper will look at w×h pixels. When nil, a simple
	// stand-in is used: the framed render for a framed image, otherwise a centred fill.
	render func(src image.Image, w, h int, opts provider.TuningOptions) image.Image
	// global is the display mode set for all images. The window offers the same three for this one
	// (P26); choosing global's makes the image follow the setting again. Inherit reads as Full.
	global provider.DisplayOverride
	// autoPosition says where the automatic crop keeps the picture along the cropped axis, in percent.
	autoPosition func(src image.Image, w, h int) (float64, bool)
	onSelect     func(opts provider.TuningOptions, onDone func())
	// onResize is called when the content's size changes, so the window can fit it.
	onResize func(fyne.Size)
}

// ShowTuneImagePopup opens the Tune Image window for one display: a preview of the desktop result,
// this image's display mode (Full artwork / Museum frame / Smart crop), the picture dragged to
// choose the part kept when cropped, and the frame settings when framed.
func (sa *SpiceApp) ShowTuneImagePopup(monitorID int, previewPath string, _ provider.TuningOptions, effectiveOpts provider.TuningOptions, _ [9]string, _ [9]provider.CropAnchor, preview spiceui.TunePreview, _ bool, onSelect func(opts provider.TuningOptions, onDone func()), onClose func()) {
	// Guard: skip if OpenGL is unavailable.
	if !sysinfo.CanCreateWindows() {
		utilLog.Println("OpenGL unavailable — tune image window cannot be created")
		ShowNativeFallbackAlert(i18n.T("Graphics error"), i18n.T("Paintpane requires OpenGL 2.0+ or hardware acceleration to show windows. The application will continue to run in the background. Please try rebooting your machine to restore graphics functionality."))
		return
	}

	sa.os.TransformToForeground()

	title := fmt.Sprintf("%s — %s %d", i18n.T("Tune image"), i18n.T("Display"), monitorID+1)

	// Create window with OpenGL panic recovery (matches CreatePreferencesWindow pattern)
	var w fyne.Window
	func() {
		defer func() {
			if r := recover(); r != nil {
				errStr := fmt.Sprintf("%v", r)
				utilLog.Printf("Recovered from tune image window creation panic: %v", errStr)
				if strings.Contains(strings.ToLower(errStr), "apiunavailable") || strings.Contains(strings.ToLower(errStr), "wgl") || strings.Contains(strings.ToLower(errStr), "opengl") {
					ShowNativeFallbackAlert(i18n.T("Graphics error"), i18n.T("Paintpane requires OpenGL 2.0+ or hardware acceleration to show windows. The application will continue to run in the background. Please try rebooting your machine to restore graphics functionality."))
				}
			}
		}()
		w = sa.NewWindow(title)
	}()
	if w == nil {
		utilLog.Println("Tune image window creation failed (returned nil or recovered)")
		return
	}

	w.SetFixedSize(true)
	// The tray stays usable; its items that change the wallpaper close this
	// window first (T-L).
	sa.tuneWindow = w

	content := newTunePanel(tuneRequest{
		previewPath:  previewPath,
		opts:         effectiveOpts,
		screenW:      preview.ScreenW,
		screenH:      preview.ScreenH,
		render:       preview.Render,
		global:       preview.Global,
		autoPosition: preview.AutoPosition,
		onSelect:     onSelect,
		onResize: func(size fyne.Size) {
			// The window is fixed-size; grow or shrink it when the frame rows show or hide.
			w.Resize(size)
		},
	})
	w.SetContent(content)
	w.Resize(content.MinSize())
	w.CenterOnScreen()

	w.SetOnClosed(func() {
		if sa.tuneWindow == w {
			sa.tuneWindow = nil
		}
		sa.os.TransformToBackground()
		if onClose != nil {
			onClose()
		}
	})

	w.Show()
	styleTitleBar(w)
}

// newTunePanel builds the content of the Tune Image window.
func newTunePanel(req tuneRequest) fyne.CanvasObject {
	opts := req.opts
	processing := false

	// This image's display mode (P26): its own choice, or the global mode it follows. Choosing the
	// global mode's segment follows the setting again, so opts.Display stays Inherit then. The older
	// frame switch is folded in and cleared, so what the window sends never pins it.
	global := req.global
	if global == provider.DisplayInherit {
		global = provider.DisplayFull
	}
	mode := tuneDisplayMode(global, opts)
	setMode := func(m provider.DisplayOverride) {
		mode = m
		opts.FrameOverride = provider.FrameOverrideInherit
		if m == global {
			opts.Display = provider.DisplayInherit
		} else {
			opts.Display = m
		}
	}
	setMode(mode)
	isFramed := func() bool { return mode == provider.DisplayFramed }
	// Only a cropped picture has a part to choose: it is dragged (J3).
	isCrop := func() bool { return mode == provider.DisplayCrop }

	// --- Preview, dragged to choose the part kept (J3) ---
	source := loadTuneSource(req.previewPath)
	previewSize := tunePreviewSize(req.screenW, req.screenH)
	renderW, renderH := int(previewSize.Width)*tuneRenderScale, int(previewSize.Height)*tuneRenderScale
	var shown *canvas.Image
	var preview fyne.CanvasObject
	if source != nil {
		shown = canvas.NewImageFromImage(nil)
		shown.FillMode = canvas.ImageFillStretch
		shown.ScaleMode = canvas.ImageScaleSmooth
		shown.CornerRadius = tunePreviewRadius
		preview = shown
	} else {
		// No picture (nothing shown yet, or the file cannot be read): one line, centred in the
		// screen-shaped box. The text must not wrap: a centred wrapping text gets its narrowest
		// width and stood one character per line.
		bg := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
		bg.CornerRadius = theme.InputRadiusSize()
		note := newSecondaryText(i18n.T("Preview unavailable"))
		note.Wrapping = fyne.TextWrapOff
		preview = container.NewStack(bg, container.NewCenter(note))
	}

	var refresh func()
	var apply func()
	var root *fyne.Container

	// How far the picture can slide, and where it sits now (percent along the cropped axis).
	scrW, scrH := req.screenW, req.screenH
	if scrW <= 0 || scrH <= 0 {
		scrW, scrH = 16, 9
	}
	var axis provider.CropAxis
	var travel, cropLen int // the slide, and the length of the kept part, in source pixels
	pos := 50.0
	autoPos := 50.0
	if source != nil {
		sb := source.Bounds()
		axis, travel = provider.CropTravel(sb.Dx(), sb.Dy(), scrW, scrH)
		if travel <= 1 {
			// Shrinking the picture to preview it rounds its shape by a pixel; that is not a crop.
			axis, travel = provider.CropNone, 0
		}
		aspect := float64(scrW) / float64(scrH)
		if axis == provider.CropHorizontal {
			cropLen = int(float64(sb.Dy()) * aspect)
		} else {
			cropLen = int(float64(sb.Dx()) / aspect)
		}
		// Only a cropped picture has an automatic position to ask for. One switched to Smart crop
		// here learns it once the crop is on screen (see apply).
		if req.autoPosition != nil && axis != provider.CropNone && isCrop() {
			if p, ok := req.autoPosition(source, scrW, scrH); ok {
				autoPos = p
			}
		}
		pos = autoPos
		if p, ok := opts.CropPosition(sb.Dx(), sb.Dy(), scrW, scrH); ok {
			pos = p
		}
	}
	canDrag := func() bool {
		return source != nil && axis != provider.CropNone && isCrop() && !processing
	}

	var renderPreview func()
	var resetBtn *widget.Button
	var resetRow *fyne.Container
	surface := newDragSurface(axis)
	surface.canDrag = canDrag
	var dragStart float64
	surface.onStart = func() { dragStart = pos }
	surface.onDrag = func(delta float32) {
		// The picture follows the pointer, so the kept part moves the other way.
		length := previewSize.Width
		if axis == provider.CropVertical {
			length = previewSize.Height
		}
		moved := float64(delta) / float64(length) * float64(cropLen) / float64(travel) * 100
		pos = math.Min(100, math.Max(0, pos-moved))
		opts.SetCropOffset(pos)
		renderPreview()
		resetRow.Show()
		if req.onResize != nil && root != nil {
			req.onResize(root.MinSize())
		}
	}
	surface.onEnd = func() {
		if pos != dragStart {
			apply()
		}
	}
	previewBox := container.New(&fixedSizeLayout{size: previewSize}, preview, surface)
	// The preview shows the desktop result for the current options (J2, J3). A cropped picture is
	// always drawn at pos, also for the automatic crop: pos is then where the wallpaper on screen
	// really is (autoPosition), whereas running the automatic analysis again on the small copy
	// shown here can choose another part of the picture than it did on the full original.
	renderPreview = func() {
		if source == nil {
			return
		}
		shownOpts := opts
		shownOpts.Display = mode // the renderer does not know the global mode the window was given
		if axis != provider.CropNone && isCrop() {
			shownOpts.SetCropOffset(pos)
		}
		res := renderTunePreview(req.render, source, renderW, renderH, shownOpts)
		if res == nil {
			return
		}
		shown.Image = res
		shown.Refresh()
	}

	// --- Applying overlay ---
	overlayBg := canvas.NewRectangle(color.NRGBA{A: 0x99})
	overlayText := canvas.NewText(i18n.T("Applying..."), color.White)
	overlayText.TextStyle = fyne.TextStyle{Bold: true}
	overlay := container.NewStack(overlayBg, container.NewCenter(overlayText))
	overlay.Hide()

	// --- This image's display mode (P26): the global mode's three choices, in the same order ---
	modes := []provider.DisplayOverride{provider.DisplayFull, provider.DisplayFramed, provider.DisplayCrop}
	modeIndex := func(m provider.DisplayOverride) int {
		for i, x := range modes {
			if x == m {
				return i
			}
		}
		return 0
	}
	modeControl := newSegmented([]string{i18n.T("Full artwork"), i18n.T("Museum frame"), i18n.T("Smart crop")}, modeIndex(mode), nil)
	modeControl.OnChanged = func(i int) {
		if processing || modes[i] == mode {
			modeControl.Selected = modeIndex(mode)
			modeControl.Refresh()
			return
		}
		setMode(modes[i])
		// A new mode starts from the automatic crop, as adding a frame did.
		opts.ResetCrop()
		pos = autoPos
		apply()
	}

	// --- Reset: back to the global mode and the automatic crop. Shown when this image has its own
	// mode or the picture has been moved. ---
	// The shared "Reset" key reads 重置 (the blocked-images button); here the owner wants 恢复.
	resetBtn = widget.NewButton(i18n.TMap("Reset", map[string]string{"zh": "恢复", "zh-Hant": "恢復"}), func() {
		if processing {
			return
		}
		setMode(global)
		opts.ResetCrop()
		pos = autoPos
		apply()
	})
	resetBtn.Importance = widget.LowImportance
	resetRow = container.NewCenter(resetBtn)

	// --- Frame rows ---
	sizeValue := widget.NewLabel("")
	sizeSlider := widget.NewSlider(50, 100)
	sizeSlider.Step = 1
	sizeSlider.SetValue(opts.FrameSize * 100)
	sizeValue.SetText(fmt.Sprintf("%d%%", int(sizeSlider.Value)))
	sizeSlider.OnChanged = func(v float64) {
		sizeValue.SetText(fmt.Sprintf("%d%%", int(v)))
		if isFramed() {
			opts.FrameSize = v / 100
			renderPreview()
		}
	}
	sizeSlider.OnChangeEnded = func(v float64) {
		opts.FrameSize = v / 100
		apply()
	}
	sizeControl := container.NewBorder(nil, nil, nil, sizeValue, sizeSlider)

	matSwitch := newToggleSwitch(opts.Matting == provider.MattingOverrideOn, nil)
	matSwitch.OnChanged = func(on bool) {
		if on {
			opts.Matting = provider.MattingOverrideOn
		} else {
			opts.Matting = provider.MattingOverrideOff
		}
		apply()
	}

	wallOptions := []string{i18n.T("Matches the artwork"), i18n.T("Neutral gray")}
	wallSelect := widget.NewSelect(wallOptions, nil)
	if opts.WallColor == provider.WallColorOverrideNeutral {
		wallSelect.SetSelectedIndex(1)
	} else {
		wallSelect.SetSelectedIndex(0)
	}
	wallSelect.OnChanged = func(string) {
		if wallSelect.SelectedIndex() == 1 {
			opts.WallColor = provider.WallColorOverrideNeutral
		} else {
			opts.WallColor = provider.WallColorOverrideAlgorithmic
		}
		apply()
	}

	frameCard := newCard(
		newSettingRow(newRowLabel(i18n.T("Frame size")), nil, sizeControl, true),
		newSettingRow(newRowLabel(i18n.T("Paper mat")), nil, matSwitch, false),
		newSettingRow(newRowLabel(i18n.T("Wall color")), nil, newBorderedSelect(wallSelect), true),
	)

	refresh = func() {
		if modeControl.Selected != modeIndex(mode) {
			modeControl.Selected = modeIndex(mode)
			modeControl.Refresh()
		}

		renderPreview()
		surface.Refresh()
		moved := isCrop() && axis != provider.CropNone && !opts.IsAutoCrop()
		setVisible(resetRow, opts.Display != provider.DisplayInherit || moved)
		setVisible(frameCard, isFramed())
		if req.onResize != nil && root != nil {
			req.onResize(root.MinSize())
		}
	}

	apply = func() {
		processing = true
		overlay.Show()
		refresh()
		if req.onSelect == nil {
			processing = false
			overlay.Hide()
			return
		}
		sent, sentMode := opts, mode
		req.onSelect(sent, func() {
			// Back on the automatic crop, ask again where the picture now on screen sits, so the
			// preview shows that and not a guess (the window may have opened on a dragged picture,
			// or on one shown whole that was just switched to Smart crop).
			newAuto, haveAuto := 0.0, false
			if source != nil && axis != provider.CropNone && req.autoPosition != nil &&
				sent.IsAutoCrop() && sentMode == provider.DisplayCrop {
				newAuto, haveAuto = req.autoPosition(source, scrW, scrH)
			}
			fyne.Do(func() {
				if haveAuto {
					autoPos = newAuto
					if opts.IsAutoCrop() {
						pos = autoPos
					}
				}
				processing = false
				overlay.Hide()
				refresh()
			})
		})
	}

	refresh()

	// The mode control is there under every global mode (P26, replacing J4's Add frame button).
	// With no picture to show there is nothing to change either: the window is the empty preview
	// alone.
	parts := []fyne.CanvasObject{container.NewCenter(previewBox)}
	if source != nil {
		parts = append(parts, container.NewCenter(modeControl), resetRow, frameCard)
	}
	body := vstack(tuneGap, parts...)
	// The window is as wide as the preview area, or wider when the mode labels need it (German,
	// for one, spells them out at about 480 px).
	width := tunePreviewMaxW + 48
	if source != nil {
		width = fyne.Max(width, modeControl.MinSize().Width+2*tunePadding)
	}
	sized := container.New(&fixedWidthLayout{width: width},
		inset(body, tunePadding, tunePadding, tunePadding, tunePadding))
	root = container.NewStack(sized, overlay)
	return root
}

// tuneDisplayMode is how the image with opts is shown under the global mode: its own choice, else
// the older frame switch (on frames it; off shows it without a frame, which under Museum frame
// means whole), else the global mode.
func tuneDisplayMode(global provider.DisplayOverride, opts provider.TuningOptions) provider.DisplayOverride {
	if opts.Display != provider.DisplayInherit {
		return opts.Display
	}
	switch opts.FrameOverride {
	case provider.FrameOverrideForceOn:
		return provider.DisplayFramed
	case provider.FrameOverrideForceOff:
		if global == provider.DisplayFramed {
			return provider.DisplayFull
		}
	}
	return global
}

func setVisible(o fyne.CanvasObject, visible bool) {
	if visible {
		o.Show()
	} else {
		o.Hide()
	}
}

// loadTuneSource decodes the picture to preview, scaled down to at most
// tuneSourceMax on its longer side. It returns nil when the file cannot be read.
func loadTuneSource(path string) image.Image {
	if path == "" {
		return nil
	}
	src, err := imaging.Open(path, imaging.AutoOrientation(true))
	if err != nil {
		utilLog.Printf("Tune image: cannot open preview %s: %v", path, err)
		return nil
	}
	b := src.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return nil
	}
	return imaging.Fit(src, tuneSourceMax, tuneSourceMax, imaging.Lanczos)
}

// tunePreviewSize is the preview's size: the shape of the screen, as large as fits in the
// preview area (360×203 for 16:9). Zero means 16:9.
func tunePreviewSize(screenW, screenH int) fyne.Size {
	if screenW <= 0 || screenH <= 0 {
		screenW, screenH = 16, 9
	}
	scale := fyne.Min(tunePreviewMaxW/float32(screenW), tunePreviewMaxH/float32(screenH))
	// Whole pixels, so the render size at twice the preview is exact.
	return fyne.NewSize(float32(int(float32(screenW)*scale+0.5)), float32(int(float32(screenH)*scale+0.5)))
}

// renderTunePreview draws src as the wallpaper will look at w×h for the options. A nil render
// uses a stand-in: the framed render for a framed image, the whole image on grey for one shown
// whole, otherwise a centred fill. Whatever the renderer returns is brought to exactly w×h.
func renderTunePreview(render func(image.Image, int, int, provider.TuningOptions) image.Image, src image.Image, w, h int, opts provider.TuningOptions) image.Image {
	var out image.Image
	if render != nil {
		out = render(src, w, h, opts)
	}
	if out == nil {
		switch {
		case opts.Display == provider.DisplayFramed || opts.FrameOverride == provider.FrameOverrideForceOn:
			if framed, err := wallpaper.RenderFramePreview(src, w, h, opts); err == nil {
				out = framed
			}
		case opts.Display == provider.DisplayFull:
			out = imaging.PasteCenter(imaging.New(w, h, color.NRGBA{0x5b, 0x5b, 0x5b, 0xff}), imaging.Fit(src, w, h, imaging.Lanczos))
		}
		if out == nil {
			out = src
		}
	}
	if b := out.Bounds(); b.Dx() != w || b.Dy() != h {
		out = imaging.Fill(out, w, h, imaging.Center, imaging.Lanczos)
	}
	return out
}

// fixedSizeLayout gives every child the same fixed size.
type fixedSizeLayout struct{ size fyne.Size }

func (f *fixedSizeLayout) MinSize(_ []fyne.CanvasObject) fyne.Size { return f.size }

func (f *fixedSizeLayout) Layout(objects []fyne.CanvasObject, _ fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(f.size)
	}
}

// ---------------------------------------------------------------------------
// Drag surface
// ---------------------------------------------------------------------------

// dragSurface is a transparent layer over the preview. Dragging it along the cropped axis
// (up and down for a tall picture, left and right for a wide one) slides the kept part (J3).
type dragSurface struct {
	widget.BaseWidget
	axis     provider.CropAxis
	canDrag  func() bool
	onStart  func()
	onDrag   func(delta float32) // movement along the axis, in logical pixels
	onEnd    func()
	dragging bool
}

var (
	_ fyne.Draggable     = (*dragSurface)(nil)
	_ desktop.Cursorable = (*dragSurface)(nil)
)

func newDragSurface(axis provider.CropAxis) *dragSurface {
	d := &dragSurface{axis: axis}
	d.ExtendBaseWidget(d)
	return d
}

func (d *dragSurface) allowed() bool {
	return d.axis != provider.CropNone && (d.canDrag == nil || d.canDrag())
}

func (d *dragSurface) Dragged(ev *fyne.DragEvent) {
	if !d.allowed() && !d.dragging {
		return
	}
	if !d.dragging {
		d.dragging = true
		if d.onStart != nil {
			d.onStart()
		}
	}
	delta := ev.Dragged.DX
	if d.axis == provider.CropVertical {
		delta = ev.Dragged.DY
	}
	if d.onDrag != nil {
		d.onDrag(delta)
	}
}

func (d *dragSurface) DragEnd() {
	if !d.dragging {
		return
	}
	d.dragging = false
	if d.onEnd != nil {
		d.onEnd()
	}
}

// Cursor shows the direction the picture moves in. Fyne has no grab cursor, so the resize
// cursors stand in for it.
func (d *dragSurface) Cursor() desktop.Cursor {
	if !d.allowed() {
		return desktop.DefaultCursor
	}
	if d.axis == provider.CropVertical {
		return desktop.VResizeCursor
	}
	return desktop.HResizeCursor
}

func (d *dragSurface) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
