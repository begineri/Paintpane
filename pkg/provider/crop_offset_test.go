package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCropTravel(t *testing.T) {
	axis, travel := CropTravel(1000, 3000, 1920, 1080) // tall
	assert.Equal(t, CropVertical, axis)
	assert.Equal(t, 3000-562, travel)

	axis, travel = CropTravel(4000, 1000, 1920, 1080) // wide
	assert.Equal(t, CropHorizontal, axis)
	assert.Equal(t, 4000-1777, travel)

	// Close to the screen's shape: still draggable, with a small travel.
	axis, travel = CropTravel(1920, 1100, 1920, 1080)
	assert.Equal(t, CropVertical, axis)
	assert.Equal(t, 20, travel)
	axis, travel = CropTravel(2000, 1080, 1920, 1080)
	assert.Equal(t, CropHorizontal, axis)
	assert.Equal(t, 80, travel)

	// Exactly the screen's shape: nothing to drag.
	axis, travel = CropTravel(3840, 2160, 1920, 1080)
	assert.Equal(t, CropNone, axis)
	assert.Zero(t, travel)
	axis, _ = CropTravel(0, 0, 1920, 1080)
	assert.Equal(t, CropNone, axis)
}

func TestCropPosition(t *testing.T) {
	var auto TuningOptions
	_, ok := auto.CropPosition(1000, 3000, 1920, 1080)
	assert.False(t, ok, "automatic")

	dragged := TuningOptions{}
	dragged.SetCropOffset(37)
	p, ok := dragged.CropPosition(1000, 3000, 1920, 1080)
	assert.True(t, ok)
	assert.Equal(t, 37.0, p)
	_, ok = dragged.CropPosition(3840, 2160, 1920, 1080)
	assert.False(t, ok, "nothing to choose when the shapes match")

	// An offset of exactly 0 is a choice, not automatic.
	zero := TuningOptions{}
	zero.SetCropOffset(0)
	p, ok = zero.CropPosition(1000, 3000, 1920, 1080)
	assert.True(t, ok)
	assert.Zero(t, p)

	// Anchors: a tall picture keeps the anchor's row, a wide one its column.
	p, _ = TuningOptions{Anchor: AnchorBottomLeft}.CropPosition(1000, 3000, 1920, 1080)
	assert.Equal(t, 100.0, p)
	p, _ = TuningOptions{Anchor: AnchorBottomLeft}.CropPosition(4000, 1000, 1920, 1080)
	assert.Equal(t, 0.0, p)
	p, _ = TuningOptions{Anchor: AnchorMiddleCenter}.CropPosition(4000, 1000, 1920, 1080)
	assert.Equal(t, 50.0, p)

	// A dragged offset beats an anchor, and a reset returns to automatic.
	both := TuningOptions{Anchor: AnchorTopCenter}
	both.SetCropOffset(80)
	assert.Equal(t, AnchorAuto, both.Anchor)
	both.ResetCrop()
	assert.True(t, both.IsAutoCrop())
}
