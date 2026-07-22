package nes

import "image/color"

// Palette maps the 64 NES color indices that Video emits to RGBA, using
// the Nestopia NTSC colors. Frontends that want a different look can
// ignore it and index the framebuffer with their own table.
var Palette = func() (p [64]color.RGBA) {
	rgb := [64]uint32{
		0x666666, 0x002A88, 0x1412A7, 0x3B00A4, 0x5C007E, 0x6E0040, 0x6C0600, 0x561D00,
		0x333500, 0x0B4800, 0x005200, 0x004F08, 0x00404D, 0x000000, 0x000000, 0x000000,
		0xADADAD, 0x155FD9, 0x4240FF, 0x7527FE, 0xA01ACC, 0xB71E7B, 0xB53120, 0x994E00,
		0x6B6D00, 0x388700, 0x0C9300, 0x008F32, 0x007C8D, 0x000000, 0x000000, 0x000000,
		0xFFFEFF, 0x64B0FF, 0x9290FF, 0xC676FF, 0xF36AFF, 0xFE6ECC, 0xFE8170, 0xEA9E22,
		0xBCBE00, 0x88D800, 0x5CE430, 0x45E082, 0x48CDDE, 0x4F4F4F, 0x000000, 0x000000,
		0xFFFEFF, 0xC0DFFF, 0xD3D2FF, 0xE8C8FF, 0xFBC2FF, 0xFEC4EA, 0xFECCC5, 0xF7D8A5,
		0xE4E594, 0xCFEF96, 0xBDF4AB, 0xB3F3CC, 0xB5EBF2, 0xB8B8B8, 0x000000, 0x000000,
	}
	for i, v := range rgb {
		p[i] = color.RGBA{R: byte(v >> 16), G: byte(v >> 8), B: byte(v), A: 0xFF}
	}
	return p
}()

// VideoRGBA renders the current framebuffer through Palette as 8-bit
// RGBA, VideoWidth*VideoHeight pixels, ready for a GPU texture upload.
// It reuses dst when it has enough capacity and allocates otherwise, so
// a frontend that feeds each frame's result back in allocates once.
// Like Video, the data is only current until the next RunFrame.
func (c *Console) VideoRGBA(dst []byte) []byte {
	const n = VideoWidth * VideoHeight * 4
	if cap(dst) < n {
		dst = make([]byte, n)
	}
	dst = dst[:n]
	for i, ci := range c.core.Framebuffer() {
		px := Palette[ci&0x3F]
		dst[4*i+0] = px.R
		dst[4*i+1] = px.G
		dst[4*i+2] = px.B
		dst[4*i+3] = px.A
	}
	return dst
}
