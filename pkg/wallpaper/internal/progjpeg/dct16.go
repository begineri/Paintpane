// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package progjpeg

// block16 holds one block of a progressive JPEG's coefficients between scans, in half the memory
// of a block. Coefficients of 8-bit JPEGs are at most 11 bits plus sign.
type block16 [blockSize]int16

func (c *block16) widen() (b block) {
	for i, v := range c {
		b[i] = int32(v)
	}
	return b
}

func (c *block16) narrow(b *block) {
	for i, v := range b {
		c[i] = int16(v)
	}
}
