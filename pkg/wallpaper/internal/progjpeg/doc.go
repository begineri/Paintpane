// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package progjpeg is Go's image/jpeg decoder (Go 1.27.3) changed to keep a progressive JPEG's
// coefficients between scans as int16 instead of int32. image/jpeg holds every coefficient of a
// progressive image until its last scan, 4 bytes per sample: a 63 MP 4:4:4 panorama needs 760 MB
// for them alone. JPEG coefficients of 8-bit images fit in 16 bits, so the decoded image is the
// same and the buffer half the size.
//
// Changes from image/jpeg: progCoeffs uses block16 (dct16.go); the package does not register
// itself with image.RegisterFormat; CMYK images are drawn with image/draw instead of the internal
// imageutil package; the encoder is left out.
package progjpeg
