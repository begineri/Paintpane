package provider

// CropAxis is the direction along which a picture is cropped to fill a screen.
type CropAxis int

const (
	// CropNone: the picture has exactly the screen's shape, so nothing is cropped.
	CropNone CropAxis = iota
	// CropHorizontal: the picture is wider than the screen; the kept part slides left and right.
	CropHorizontal
	// CropVertical: the picture is taller than the screen; the kept part slides up and down.
	CropVertical
)

// CropTravel reports the axis a picture of imgW×imgH is cropped along to fill tgtW×tgtH, and how
// far, in source pixels, the kept part can slide. It uses the same integer sizes as the crop
// itself, so a picture that is close to the screen's shape has a small travel and one with exactly
// its shape has none. The picture is never cropped along both axes.
func CropTravel(imgW, imgH, tgtW, tgtH int) (CropAxis, int) {
	if imgW <= 0 || imgH <= 0 || tgtW <= 0 || tgtH <= 0 {
		return CropNone, 0
	}
	aspect := float64(tgtW) / float64(tgtH)
	if float64(imgW)/float64(imgH) > aspect {
		if travel := imgW - int(float64(imgH)*aspect); travel > 0 {
			return CropHorizontal, travel
		}
		return CropNone, 0
	}
	if travel := imgH - int(float64(imgW)/aspect); travel > 0 {
		return CropVertical, travel
	}
	return CropNone, 0
}

// anchorOffset is where along the cropped axis an anchor keeps: 0, 50 or 100 percent. A picture
// cropped left and right keeps the anchor's column; one cropped up and down keeps its row.
func anchorOffset(a CropAnchor, axis CropAxis) (float64, bool) {
	if a < AnchorTopLeft || a > AnchorBottomRight || axis == CropNone {
		return 0, false
	}
	i := int(a - AnchorTopLeft)
	idx := i % 3
	if axis == CropVertical {
		idx = i / 3
	}
	return float64(idx) * 50, true
}

// CropPosition is the chosen position of the kept part along the cropped axis, 0 to 100 percent
// (0 keeps the left or top, 100 the right or bottom). ok is false when the choice is automatic,
// or when there is nothing to choose because the picture has the screen's shape. A dragged offset
// wins over an anchor; an anchor (from the keep-area shortcuts) counts as 0, 50 or 100.
func (o TuningOptions) CropPosition(imgW, imgH, tgtW, tgtH int) (percent float64, ok bool) {
	axis, _ := CropTravel(imgW, imgH, tgtW, tgtH)
	if axis == CropNone {
		return 0, false
	}
	if o.HasCropOffset {
		p := o.CropOffset
		if p < 0 {
			p = 0
		}
		if p > 100 {
			p = 100
		}
		return p, true
	}
	return anchorOffset(o.Anchor, axis)
}

// SetCropOffset records a dragged position and drops the anchor it replaces.
func (o *TuningOptions) SetCropOffset(percent float64) {
	o.HasCropOffset = true
	o.CropOffset = percent
	o.Anchor = AnchorAuto
}

// ResetCrop returns the crop to automatic.
func (o *TuningOptions) ResetCrop() {
	o.HasCropOffset = false
	o.CropOffset = 0
	o.Anchor = AnchorAuto
}

// IsAutoCrop reports whether the crop is left to the automatic choice.
func (o TuningOptions) IsAutoCrop() bool {
	return !o.HasCropOffset && (o.Anchor == AnchorAuto || o.Anchor == 0)
}
