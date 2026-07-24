package ppu

// Per-dot rendering pipeline.cpp:
// tile fetches (LoadTileInfo), the sprite-evaluation state machine
// (ProcessSpriteEvaluation), sprite tile loading (LoadSprite/
// LoadSpriteTileInfo), the sprite shifters (ProcessSpriteShifters),
// pixel composition (GetPixelColor/DrawPixel) and the delayed-state
// machine (UpdateState).

// reverseByte reverses the bit order of a byte.
func reverseByte(b byte) byte {
	b = (b&0xF0)>>4 | (b&0x0F)<<4
	b = (b&0xCC)>>2 | (b&0x33)<<2
	b = (b&0xAA)>>1 | (b&0x55)<<1
	return b
}

func highestBitIndex(v byte) byte {
	i := byte(0)
	for v > 1 {
		v >>= 1
		i++
	}
	return i
}

// --- PPU bus access ---

func (p *PPU) readVram(addr uint16) byte {
	p.setBusAddress(addr)
	v := p.mapperReadVram(addr)
	// The address and data buses share the AD0-7 pins: after the read the
	// low pins carry the byte just fetched.
	p.BusDataPins = v
	return v
}

// readBusData performs the data half of a two-cycle fetch: the address was
// latched on the previous (ALE) dot, so the read uses the bus as-is.
func (p *PPU) readBusData() byte {
	v := p.mapperReadVram(p.PpuBusAddress & 0x3FFF)
	p.BusDataPins = v
	return v
}

// mapperReadVram / mapperWriteVram resolve the PPU address space
// ($0000-$3FFF): pattern tables via the board, nametables via CIRAM
// mirroring, palette RAM directly.
func (p *PPU) mapperReadVram(addr uint16) byte {
	addr &= 0x3FFF
	if addr < 0x2000 {
		return p.board.ReadCHR(addr)
	}
	// The PPU bus for $2000-$3FFF addresses CIRAM: $3000-$3FFF mirrors
	// $2000-$2FFF. Palette RAM is a separate internal latch read directly
	// by the register/pixel paths, not over this bus — so a $2007 palette
	// read loads its buffer with the underlying nametable byte at $2Fxx.
	if p.ntSrc != nil {
		if v, ok := p.ntSrc.ReadNT(0x2000 | (addr & 0x0FFF)); ok {
			return v
		}
	}
	return p.VRAM[p.vramIndex(addr)]
}

func (p *PPU) mapperWriteVram(addr uint16, value byte) {
	addr &= 0x3FFF
	if addr < 0x2000 {
		p.board.WriteCHR(addr, value)
		return
	}
	if p.ntSrc != nil && p.ntSrc.WriteNT(0x2000|(addr&0x0FFF), value) {
		return
	}
	p.VRAM[p.vramIndex(addr)] = value
}

// --- Scroll helpers (nesdev) ---

func (p *PPU) incVerticalScrolling() {
	addr := p.VideoRAMAddr
	if addr&0x7000 != 0x7000 {
		addr += 0x1000
	} else {
		addr &^= 0x7000
		y := (addr & 0x03E0) >> 5
		switch y {
		case 29:
			y = 0
			addr ^= 0x0800
		case 31:
			y = 0
		default:
			y++
		}
		addr = (addr &^ 0x03E0) | (y << 5)
	}
	p.VideoRAMAddr = addr
}

func (p *PPU) incHorizontalScrolling() {
	addr := p.VideoRAMAddr
	if addr&0x001F == 31 {
		addr = (addr &^ 0x001F) ^ 0x0400
	} else {
		addr++
	}
	p.VideoRAMAddr = addr
}

func (p *PPU) getNameTableAddr() uint16 { return 0x2000 | (p.VideoRAMAddr & 0x0FFF) }

func (p *PPU) getAttributeAddr() uint16 {
	return 0x23C0 | (p.VideoRAMAddr & 0x0C00) | ((p.VideoRAMAddr >> 4) & 0x38) | ((p.VideoRAMAddr >> 2) & 0x07)
}

// --- Background tile fetch ---

// loadTileInfo runs the background fetch cadence as hardware does, in
// two-dot pairs: the odd dot latches the fetch address onto the bus (the
// ALE phase), the even dot reads the data from whatever address is on the
// bus. Splitting the fetch this way lets well-timed $2006 writes and
// $2007 reads corrupt the in-flight fetch, exactly as on the 2C02.
func (p *PPU) loadTileInfo() {
	if p.Cycle&0x01 != 0 {
		p.loadTileInfoOdd()
	} else {
		p.loadTileInfoEven()
	}
}

// loadTileInfoOdd is the address (ALE) phase of each two-dot fetch.
func (p *PPU) loadTileInfoOdd() {
	switch (p.Cycle >> 1) & 0x03 {
	case 0:
		p.setBusAddress(p.getNameTableAddr())
	case 1:
		p.setBusAddress(p.getAttributeAddr())
	case 2:
		p.setBusAddress(p.Tile.TileAddr)
	case 3:
		p.setBusAddress(p.Tile.TileAddr + 8)
	}
}

// loadTileInfoEven is the data phase of each two-dot fetch.
func (p *PPU) loadTileInfoEven() {
	switch (p.Cycle >> 1) & 0x03 {
	case 0:
		// Last dot of the 8-dot sequence (dot 8, 16, ...): finish the high
		// bit-plane read, then reload the low byte of the shifters.
		p.Tile.HighByte = p.readBusData()
		p.HighBitShift = (p.HighBitShift & 0xFF00) | uint16(p.Tile.HighByte)
		p.LowBitShift = (p.LowBitShift & 0xFF00) | uint16(p.Tile.LowByte)
		p.PreviousTilePalette = p.CurrentTilePalette
		p.CurrentTilePalette = p.Tile.PaletteOffset
	case 1:
		tileIndex := uint16(p.readBusData())
		p.Tile.TileAddr = (tileIndex << 4) | (p.VideoRAMAddr >> 12) | p.Control.BackgroundPatternAddr
	case 2:
		shift := ((p.VideoRAMAddr >> 4) & 0x04) | (p.VideoRAMAddr & 0x02)
		p.Tile.PaletteOffset = ((p.readBusData() >> shift) & 0x03) << 2
	case 3:
		p.Tile.LowByte = p.readBusData()
	}
}

func (p *PPU) shiftTileRegisters() {
	p.LowBitShift <<= 1
	p.HighBitShift <<= 1
	p.HighBitShift |= 1
}

// --- Sprite loading ---

func (p *PPU) loadSprite(spriteY, tileIndex, attributes, spriteX byte) {
	backgroundPriority := attributes&0x20 == 0x20
	horizontalMirror := attributes&0x40 == 0x40
	verticalMirror := attributes&0x80 == 0x80

	var spriteSizeMask uint16 = 7
	if p.Control.LargeSprites {
		spriteSizeMask = 15
	}

	// Pre-render uses the truncated 8-bit line number 261.
	scanline8Bit := byte(p.Scanline)
	if p.Scanline < 0 {
		scanline8Bit = 261 & 0xFF // pre-render truncated to 8 bits (=5)
	}
	rangeResult := uint16(scanline8Bit) - uint16(spriteY)
	if verticalMirror {
		rangeResult ^= spriteSizeMask
	}

	var tileAddr uint16
	if p.Control.LargeSprites {
		tileAddr = (((uint16(tileIndex) & 0x01) << 12) | ((uint16(tileIndex) &^ 0x01) << 4)) + ((rangeResult & 0x08) << 1) + (rangeResult & 0x07)
	} else {
		tileAddr = (p.Control.SpritePatternAddr | (uint16(tileIndex) << 4)) + (rangeResult & 0x07)
	}

	info := &p.SpriteTiles[p.SpriteIndex]
	info.BackgroundPriority = backgroundPriority
	info.PaletteOffset = ((attributes & 0x03) << 2) | 0x10
	info.LowByte = p.readVram(tileAddr)
	info.HighByte = p.readVram(tileAddr + 8)
	info.SpriteX = spriteX

	inRange := rangeResult <= spriteSizeMask
	if inRange {
		if horizontalMirror {
			info.LowByte = reverseByte(info.LowByte)
			info.HighByte = reverseByte(info.HighByte)
		}
		p.SpriteShifterList[p.SpriteIndex] = ((uint16(spriteX) + 1) << 4) | uint16(p.SpriteIndex)
		p.ExpiredSpriteShifters &^= 1 << p.SpriteIndex
		p.SpriteCount++
		p.ProcessSprites = true
	} else {
		info.LowByte = 0
		info.HighByte = 0
		p.SpriteShifterList[p.SpriteIndex] = spriteShifterDone
	}
	p.SpriteIndex++
}

func (p *PPU) loadSpriteTileInfo() {
	p.SpriteIndex = uint32(p.Cycle-257) >> 3
	base := p.SpriteIndex * 4
	p.loadSprite(
		p.SecondarySpriteRAM[base],
		p.SecondarySpriteRAM[base+1],
		p.SecondarySpriteRAM[base+2],
		p.SecondarySpriteRAM[base+3],
	)
}

// --- Pixel composition ---

func (p *PPU) getPixelColor() byte {
	offset := p.XScroll
	var backgroundColor byte
	var spriteBgColor byte

	if uint16(p.Cycle) > p.MinimumDrawBgCycle {
		spriteBgColor = byte(((p.LowBitShift<<offset)&0x8000)>>15) | byte(((p.HighBitShift<<offset)&0x8000)>>14)
		backgroundColor = spriteBgColor
	}

	spriteIndex := -1
	var spriteColor byte
	if p.ProcessSprites && p.PrevRenderingEnabled {
		remainingShifters := p.ActiveSpriteShifters
		if p.DotSkipped != 0 {
			remainingShifters = 0xFF
		}

		for remainingShifters != 0 {
			i := highestBitIndex(remainingShifters)
			remainingShifters &^= 1 << i

			sprite := &p.SpriteTiles[i]
			currColor := ((sprite.HighByte >> 6) & 0x2) | (sprite.LowByte >> 7)
			if currColor != 0 {
				spriteIndex = int(i)
				spriteColor = currColor
			}
			sprite.HighByte <<= 1
			sprite.LowByte <<= 1

			if sprite.HighByte|sprite.LowByte == 0 {
				p.ActiveSpriteShifters &^= 1 << i
				p.updateProcessSpritesFlag()
			}
		}

		if uint16(p.Cycle) > p.MinimumDrawSpriteCycle && p.Mask.SpritesEnabled {
			if p.SpriteCount > 8 && spriteColor == 0 {
				for si := byte(8); si < p.SpriteCount; si++ {
					sprite := &p.SpriteTiles[si]
					shift := uint32(p.Cycle) - uint32(sprite.SpriteX) - 1
					if shift < 8 {
						spriteColor = ((sprite.LowByte<<shift)&0x80)>>7 | ((sprite.HighByte<<shift)&0x80)>>6
						if spriteColor != 0 {
							spriteIndex = int(si)
							break
						}
					}
				}
			}

			if spriteColor != 0 {
				if p.Sprite0Visible && spriteIndex == 0 && spriteBgColor != 0 &&
					p.Cycle != 256 && p.Mask.BackgroundEnabled && !p.Status.Sprite0Hit &&
					uint16(p.Cycle) > p.MinimumDrawSpriteStandardCycle {
					p.Status.Sprite0Hit = true
				}

				if backgroundColor == 0 || !p.SpriteTiles[spriteIndex].BackgroundPriority {
					return p.SpriteTiles[spriteIndex].PaletteOffset + spriteColor
				}
			}
		}
	}

	if uint16(offset)+uint16((p.Cycle-1)&0x07) < 8 {
		return p.PreviousTilePalette + backgroundColor
	}
	return p.CurrentTilePalette + backgroundColor
}

// drawPixel writes the visible pixel.
// framebuffer holds the palette-RAM value (0-63 color index).
func (p *PPU) drawPixel() {
	pos := (int(p.Scanline) << 8) + int(p.Cycle) - 1
	if pos < 0 || pos >= len(p.framebuffer) {
		return
	}
	if p.isRenderingEnabled() || (p.VideoRAMAddr&0x3F00) != 0x3F00 {
		color := p.getPixelColor()
		if color&0x03 != 0 {
			p.framebuffer[pos] = p.PaletteRAM[color] & p.PaletteRAMMask
		} else {
			p.framebuffer[pos] = p.PaletteRAM[0] & p.PaletteRAMMask
		}
	} else {
		// Forced-blank with v in $3F00-$3FFF shows that palette entry.
		p.framebuffer[pos] = p.PaletteRAM[p.VideoRAMAddr&0x1F] & p.PaletteRAMMask
	}
}

// --- Sprite shifters ---

func (p *PPU) updateProcessSpritesFlag() {
	p.ProcessSprites = p.SpriteCount != 0 || p.ActiveSpriteShifters != 0 || p.DotSkipped != 0
}

func (p *PPU) processSpriteShifters() {
	if p.NextSpriteShifterCycle == uint16(p.Cycle) {
		for uint32(p.SpriteShifterList[p.NextSpriteShifter]>>4) == uint32(p.Cycle) {
			bit := byte(1) << (p.SpriteShifterList[p.NextSpriteShifter] & 7)
			if p.CountingSpriteShifters&bit != 0 {
				p.ActiveSpriteShifters |= bit
				p.ExpiredSpriteShifters |= bit
				p.CountingSpriteShifters &^= bit
			}
			p.NextSpriteShifter++
		}
		p.NextSpriteShifterCycle = p.SpriteShifterList[p.NextSpriteShifter] >> 4
	}
}

// --- Sprite evaluation state machine ---

func (p *PPU) processSpriteEvaluationStart() {
	p.Sprite0Added = false
	p.SpriteInRange = false
	p.SecondaryOamAddr = 0
	p.OverflowBugCounter = 0
	p.OamCopyDone = false
	// Evaluation can start on any OAM byte (per OAMADDR) and reads that byte
	// as sprite 0's Y. The OAM address itself is left as-is (possibly
	// misaligned); only the aligned visible-sprite span is tracked here.
	p.FirstVisibleSpriteAddr = p.SpriteRAMAddr & 0xFC
	p.LastVisibleSpriteAddr = p.FirstVisibleSpriteAddr
}

func (p *PPU) spriteHeight() int {
	if p.Control.LargeSprites {
		return 16
	}
	return 8
}

func (p *PPU) processSpriteEvaluation() {
	if p.Cycle < 65 {
		// Clear secondary OAM during cycles 1-64.
		p.OamCopybuffer = 0xFF
		p.SecondarySpriteRAM[p.SecondaryOamAddr&0x1F] = 0xFF
		if p.Cycle&1 == 0 {
			p.SecondaryOamAddr++
		}
		return
	}

	if p.Cycle&0x01 != 0 {
		if p.Cycle == 65 {
			p.processSpriteEvaluationStart()
		}
		p.OamCopybuffer = p.SpriteRAM[p.SpriteRAMAddr]
		return
	}

	spriteAddrH := p.SpriteRAMAddr >> 2
	spriteAddrL := p.SpriteRAMAddr & 3
	height := p.spriteHeight()

	if p.OamCopyDone {
		spriteAddrH = (spriteAddrH + 1) & 0x3F
		p.OamCopybuffer = p.SecondarySpriteRAM[p.SecondaryOamAddr&0x1F]
	} else {
		if !p.SpriteInRange && int(p.Scanline) >= int(p.OamCopybuffer) && int(p.Scanline) < int(p.OamCopybuffer)+height {
			p.SpriteInRange = !p.OamCopyDone
		}

		if p.SecondaryOamAddr < 0x20 {
			p.SecondarySpriteRAM[p.SecondaryOamAddr] = p.OamCopybuffer

			if p.SpriteInRange {
				if p.Cycle == 66 {
					p.Sprite0Added = true
				}
				spriteAddrL++
				p.SecondaryOamAddr++

				if spriteAddrL >= 4 {
					spriteAddrH = (spriteAddrH + 1) & 0x3F
					spriteAddrL = 0
					if spriteAddrH == 0 {
						p.OamCopyDone = true
					}
				}

				if p.SecondaryOamAddr&0x03 == 0 {
					// Done copying all 4 bytes of this sprite.
					p.SpriteInRange = false
					p.LastVisibleSpriteAddr = (spriteAddrH - 1) * 4
					if spriteAddrL != 0 {
						inRange := int(p.Scanline) >= int(p.OamCopybuffer) && int(p.Scanline) < int(p.OamCopybuffer)+height
						if !inRange {
							spriteAddrL = 0
						}
					}
				}
			} else {
				spriteAddrH = (spriteAddrH + 1) & 0x3F
				spriteAddrL = 0
				if spriteAddrH == 0 {
					p.OamCopyDone = true
				}
			}
		} else {
			p.OamCopybuffer = p.SecondarySpriteRAM[p.SecondaryOamAddr&0x1F]
			switch {
			case p.OamCopyDone:
				spriteAddrH = (spriteAddrH + 1) & 0x3F
				spriteAddrL = 0
			case p.SpriteInRange:
				p.Status.SpriteOverflow = true
				spriteAddrL++
				if spriteAddrL == 4 {
					spriteAddrH = (spriteAddrH + 1) & 0x3F
					spriteAddrL = 0
				}
				if p.OverflowBugCounter == 0 {
					p.OverflowBugCounter = 3
				} else if p.OverflowBugCounter > 0 {
					p.OverflowBugCounter--
					if p.OverflowBugCounter == 0 {
						p.OamCopyDone = true
						spriteAddrL = 0
					}
				}
			default:
				spriteAddrH = (spriteAddrH + 1) & 0x3F
				spriteAddrL = (spriteAddrL + 1) & 0x03
				if spriteAddrH == 0 {
					p.OamCopyDone = true
				}
			}
		}
	}
	p.SpriteRAMAddr = (spriteAddrL & 0x03) | (spriteAddrH << 2)
}

// --- Scanline dispatch ---

// processScanline runs cycles 1..340 of a visible/pre-render scanline.
func (p *PPU) processScanline() {
	switch {
	case p.Cycle <= 256:
		if p.PrevRenderingEnabled {
			if p.Scanline >= 0 {
				p.processSpriteShifters()
				p.drawPixel()
				p.processSpriteEvaluation()
				p.shiftTileRegisters()
			} else if p.Cycle == 1 {
				p.Status.VerticalBlank = false
				p.clearNmiFlag()
			}
			if p.Cycle&0x07 == 0 {
				p.incHorizontalScrolling()
				if p.Cycle == 256 {
					p.incVerticalScrolling()
				}
			}
			p.loadTileInfo()
		} else {
			p.processRenderingDisabledPixel()
		}

	case p.Cycle >= 257 && p.Cycle <= 320:
		if p.PrevRenderingEnabled {
			p.Sprite0Visible = p.Sprite0Added
			p.SpriteRAMAddr = 0
			switch (p.Cycle - 257) % 8 {
			case 0:
				p.readVram(p.getNameTableAddr())
			case 2:
				p.readVram(p.getNameTableAddr())
			case 4:
				p.loadSpriteTileInfo()
			}
			if p.Scanline == -1 && p.Cycle >= 280 && p.Cycle <= 304 {
				p.VideoRAMAddr = (p.VideoRAMAddr &^ 0x7BE0) | (p.TmpVideoRAMAddr & 0x7BE0)
			}
			// OAM2ADDR increments during the first 4 dots of each set of 8
			// (257-260, 265-268, ...); the reset during dot 256.5-257.0
			// overrides the first increment, and cycle 321 adds one more
			// after the last set.
			if (p.Cycle-1)&4 == 0 {
				p.SecondaryOamAddr++
			}
		}
		if p.Cycle == 257 {
			p.SpriteIndex = 0
			p.SpriteCount = 0
			if p.PrevRenderingEnabled {
				p.VideoRAMAddr = (p.VideoRAMAddr &^ 0x041F) | (p.TmpVideoRAMAddr & 0x041F)
				p.SecondaryOamAddr = 0
			}
		}

	case p.Cycle >= 321 && p.Cycle <= 336:
		if p.PrevRenderingEnabled {
			switch p.Cycle {
			case 321:
				p.SecondaryOamAddr++
			case 328, 336:
				p.incHorizontalScrolling()
			}
			p.shiftTileRegisters()
			p.loadTileInfo()
		}

	case p.Cycle == 337:
		if p.isRenderingEnabled() {
			p.Tile.TileAddr = uint16(p.readVram(p.getNameTableAddr()))
		}

	case p.Cycle == 339:
		p.ActiveSpriteShifters = 0
		if p.isRenderingEnabled() {
			p.Tile.TileAddr = uint16(p.readVram(p.getNameTableAddr()))

			for i := 0; i < 8; i++ {
				bit := byte(1) << (p.SpriteShifterList[i] & 0x07)
				if p.SpriteShifterList[i] != spriteShifterDone && p.ExpiredSpriteShifters&bit == 0 {
					p.CountingSpriteShifters |= bit
				}
			}

			if p.dotSkip && p.Scanline == -1 && p.Cycle == 339 && p.Frame&0x01 != 0 {
				// Odd-frame skipped dot (NTSC): jump from 339 to 340->0.
				p.Cycle = 340
				p.DotSkipped = 3
				p.NeedStateUpdate = true
				for i := 0; i < 8; i++ {
					p.SpriteShifterList[i] += 1 << 4
				}
				p.NextSpriteShifterCycle++
			}
		}

		for i := 0; i < 8; i++ {
			if p.SpriteShifterList[i] != spriteShifterDone {
				bit := byte(1) << (p.SpriteShifterList[i] & 0x07)
				if p.CountingSpriteShifters&bit == 0 {
					p.ActiveSpriteShifters |= bit
				}
			}
		}
		p.updateProcessSpritesFlag()
	}
}

// processRenderingDisabledPixel handles cycles 1-256 with rendering off.
func (p *PPU) processRenderingDisabledPixel() {
	if p.Scanline >= 0 {
		p.processSpriteShifters()
		p.drawPixel()
	} else if p.Cycle == 1 {
		p.Status.VerticalBlank = false
		p.clearNmiFlag()
	}
}

// --- Delayed state machine ---

// corruptOamRow copies one 8-byte OAM row (and its secondary-OAM slot) over
// another, the hardware corruption that toggling rendering mid-scanline
// causes. A no-op when source and destination coincide.
func (p *PPU) corruptOamRow(sourceRow, destRow byte) {
	if sourceRow == destRow {
		return
	}
	src := int(sourceRow) << 3
	dst := int(destRow) << 3
	copy(p.SpriteRAM[dst:dst+8], p.SpriteRAM[src:src+8])
	p.SecondarySpriteRAM[destRow] = p.SecondarySpriteRAM[sourceRow]
}

func (p *PPU) updateState() {
	p.NeedStateUpdate = false

	// Rendering-enabled is committed with a 1-cycle delay.
	if p.PrevRenderingEnabled != p.RenderingEnabled {
		p.PrevRenderingEnabled = p.RenderingEnabled
		if p.Scanline < 240 {
			// Toggling rendering mid-scanline corrupts an OAM row: the
			// OAM1/OAM2 rows swap contents (direction depends on whether
			// rendering was just enabled or disabled).
			if p.Cycle >= 257 || p.Cycle&1 == 0 {
				if p.PrevRenderingEnabled {
					p.corruptOamRow(p.SpriteRAMAddr>>3, p.SecondaryOamAddr&0x1F)
				} else {
					p.corruptOamRow(p.SecondaryOamAddr&0x1F, p.SpriteRAMAddr>>3)
				}
			}
			if !p.PrevRenderingEnabled {
				// Rendering disabled mid-screen: bus address returns to v.
				p.setBusAddress(p.VideoRAMAddr & 0x3FFF)
				if p.Cycle >= 65 && p.Cycle <= 256 {
					// Glitchy OAMADDR increment when disabling during eval.
					p.SpriteRAMAddr++
				}
			}
		}
	}

	if p.RenderingEnabled != (p.Mask.BackgroundEnabled || p.Mask.SpritesEnabled) {
		p.RenderingEnabled = p.Mask.BackgroundEnabled || p.Mask.SpritesEnabled
		p.NeedStateUpdate = true
	}

	if p.UpdateVramAddrDelay > 0 {
		p.UpdateVramAddrDelay--
		if p.UpdateVramAddrDelay == 0 {
			p.VideoRAMAddr = p.UpdateVramAddr
			// The glitchy updates corrupt V and T together; keep them synced.
			p.TmpVideoRAMAddr = p.VideoRAMAddr
			if p.Scanline >= 240 || !p.isRenderingEnabled() {
				p.setBusAddress(p.VideoRAMAddr & 0x3FFF)
			} else {
				// Mid-rendering the write drives the high address pins from
				// the new v immediately, but AD0-7 keep the value latched at
				// the last ALE — the in-flight fetch reads a hybrid address.
				p.setBusAddress((p.VideoRAMAddr & 0x3F00) | (p.PpuBusAddress & 0x00FF))
			}
		} else {
			p.NeedStateUpdate = true
		}
	}

	if p.IgnoreVramRead > 0 {
		p.IgnoreVramRead--
		if p.IgnoreVramRead > 0 {
			p.NeedStateUpdate = true
		}
	}

	if p.NeedVideoRAMIncrement {
		// Delay the $2007 vram increment by 1 PPU cycle.
		p.NeedVideoRAMIncrement = false
		p.updateVideoRAMAddr()
	}

	if p.MemoryDataReadStateMachine > 0 {
		p.MemoryDataReadStateMachine--
		if p.MemoryDataReadStateMachine == 0 {
			addr := p.PpuBusAddress & 0x3FFF
			if p.aleDot() {
				// The buffer fill collides with the ALE phase of a background
				// fetch: with ALE and /RD asserted together the address latch
				// holds the AD0-7 pins (still carrying the previous dot's
				// data) instead of the new low byte, so both this read and
				// the fetch in flight use the hybrid address.
				addr = (addr & 0x3F00) | uint16(p.BusDataPins)
			}
			p.MemoryReadBuffer = p.readVram(addr)
			p.NeedVideoRAMIncrement = true
		}
		p.NeedStateUpdate = true
	}

	if p.MemoryDataWriteStateMachine > 0 {
		p.MemoryDataWriteStateMachine--
		if p.MemoryDataWriteStateMachine == 0 {
			switch {
			case p.PpuBusAddress&0x3FFF >= 0x3F00:
				p.writePaletteRAM(p.PpuBusAddress, p.MemoryDataWriteLatch)
			case p.Scanline >= 240 || !p.isRenderingEnabled():
				p.mapperWriteVram(p.PpuBusAddress&0x3FFF, p.MemoryDataWriteLatch)
			default:
				// During rendering the LSB of the address is written instead.
				p.mapperWriteVram(p.PpuBusAddress&0x3FFF, byte(p.PpuBusAddress&0xFF))
			}
			p.NeedVideoRAMIncrement = true
		}
		p.NeedStateUpdate = true
	}

	if p.DotSkipped != 0 {
		p.DotSkipped--
		if p.DotSkipped != 0 {
			p.NeedStateUpdate = true
		}
		p.updateProcessSpritesFlag()
	}
}

// aleDot reports whether the current dot is the address (ALE) phase of a
// background fetch — an odd dot inside the fetch regions of a visible or
// pre-render scanline with rendering enabled.
func (p *PPU) aleDot() bool {
	return p.PrevRenderingEnabled && p.Scanline < 240 && p.Cycle&1 == 1 &&
		p.Cycle >= 1 && (p.Cycle <= 256 || (p.Cycle >= 321 && p.Cycle <= 336))
}

// updateVideoRAMAddr performs the $2007 address post-increment.
func (p *PPU) updateVideoRAMAddr() {
	if p.Scanline >= 240 || !p.isRenderingEnabled() {
		inc := uint16(1)
		if p.Control.VerticalWrite {
			inc = 32
		}
		p.VideoRAMAddr = (p.VideoRAMAddr + inc) & 0x7FFF
		// Clock A12 via the bus address (needed by the MMC3 IRQ counter).
		p.setBusAddress(p.VideoRAMAddr & 0x3FFF)
	} else {
		// During rendering the access does a coarse-X + Y increment.
		p.incHorizontalScrolling()
		p.incVerticalScrolling()
	}
}
