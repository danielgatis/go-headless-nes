package ppu

import (
	"github.com/danielgatis/go-headless-nes/internal/bus"
	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

// Ranges maps the PPU into the CPU address space at $2000-$3FFF (mirrored
// every 8 bytes) as a memory handler.
func (p *PPU) Ranges() *bus.Ranges {
	r := bus.NewRanges()
	r.Add(bus.OpAny, 0x2000, 0x3FFF)
	return r
}

// ReadReg reads a PPU register (with side effects).
func (p *PPU) ReadReg(addr uint16) byte { return p.ReadRegister(addr) }

// WriteReg writes a PPU register.
func (p *PPU) WriteReg(addr uint16, v byte) { p.WriteRegister(addr, v) }

// PeekReg reads a PPU register without side effects (for debuggers).
func (p *PPU) PeekReg(addr uint16) byte { return p.PeekRegister(addr) }

// CPU-visible register interface ($2000-$2007, mirrored through $3FFF)
// and the PPU address space (register R/W and palette/open-bus handling).

// openBusDecayFrames is how many frames a PPU I/O-latch bit holds a 1
// before decaying (> 3 frames; conservative). Counted by completed frame.
const openBusDecayFrames = 3

// setOpenBus drives value onto the latch bits selected by mask and
// re-arms their per-bit decay stamps; unselected live bits decay after
// openBusDecayFrames.
func (p *PPU) setOpenBus(mask, value byte) {
	if mask == 0xFF {
		p.OpenBus = value
		for i := 0; i < 8; i++ {
			p.OpenBusDecayStamp[i] = uint32(p.Frame)
		}
		return
	}
	openBus := uint16(p.OpenBus) << 8
	for i := 0; i < 8; i++ {
		openBus >>= 1
		if mask&0x01 != 0 {
			if value&0x01 != 0 {
				openBus |= 0x80
			} else {
				openBus &= 0xFF7F
			}
			p.OpenBusDecayStamp[i] = uint32(p.Frame)
		} else if uint32(p.Frame)-p.OpenBusDecayStamp[i] > openBusDecayFrames {
			openBus &= 0xFF7F
		}
		value >>= 1
		mask >>= 1
	}
	p.OpenBus = byte(openBus)
}

// applyOpenBus fills the masked-off bits of value with open bus.
func (p *PPU) applyOpenBus(mask, value byte) byte {
	p.setOpenBus(^mask, value)
	return value | (p.OpenBus & mask)
}

// decayOpenBus is a per-dot no-op placeholder: the reference decays by frame
// count inside setOpenBus, not per dot. Kept so Tick's call site is
// stable and future per-dot models can hook here.
func (p *PPU) decayOpenBus() {}

// ReadRegister services a CPU read of a PPU register.
func (p *PPU) ReadRegister(addr uint16) byte {
	openBusMask := byte(0xFF)
	returnValue := byte(0)

	switch addr & 0x07 {
	case 2: // Status
		p.WriteToggle = false
		returnValue = statusByte(p.Status)
		p.updateStatusFlag()
		openBusMask = 0x1F

	case 4: // SpriteData
		if p.Scanline <= 239 && p.isRenderingEnabled() {
			if (p.Cycle >= 257 && p.Cycle <= 340) || p.Cycle == 0 {
				p.OamCopybuffer = p.SecondarySpriteRAM[p.SecondaryOamAddr&0x1F]
			}
			returnValue = p.OamCopybuffer
		} else {
			returnValue = p.readSpriteRAM(p.SpriteRAMAddr)
		}
		openBusMask = 0x00

	case 7: // VideoMemoryData
		switch {
		case !p.AllowFullPpuAccess:
			openBusMask = 0x00
			returnValue = 0
		case p.IgnoreVramRead != 0:
			openBusMask = 0xFF
		default:
			returnValue = p.MemoryReadBuffer
			// The read buffer updates 2 PPU cycles after the CPU read ends.
			p.MemoryDataReadStateMachine = 5
			if (p.PpuBusAddress & 0x3FFF) >= 0x3F00 {
				returnValue = (p.readPaletteRAM(p.PpuBusAddress) & p.PaletteRAMMask) | (p.OpenBus & 0xC0)
				openBusMask = 0xC0
			} else {
				openBusMask = 0x00
			}
			p.IgnoreVramRead = 6
			p.NeedStateUpdate = true
		}
	}
	return p.applyOpenBus(openBusMask, returnValue)
}

// PeekRegister is ReadRegister without side effects, for debuggers.
func (p *PPU) PeekRegister(addr uint16) byte {
	openBusMask := byte(0xFF)
	returnValue := byte(0)
	switch addr & 0x07 {
	case 2:
		returnValue = statusByte(p.Status)
		if p.Scanline == p.nmiScanline && p.Cycle < 3 {
			returnValue &= 0x7F
		}
		openBusMask = 0x1F
	case 4:
		if p.Scanline <= 239 && p.isRenderingEnabled() {
			// During visible-line rendering, $2004 returns the value the
			// sprite-evaluation hardware currently has latched.
			returnValue = p.OamCopybuffer
		} else {
			returnValue = p.SpriteRAM[p.SpriteRAMAddr]
		}
		openBusMask = 0x00
	case 7:
		returnValue = p.MemoryReadBuffer
		if (p.VideoRAMAddr & 0x3FFF) >= 0x3F00 {
			returnValue = (p.readPaletteRAM(p.VideoRAMAddr) & p.PaletteRAMMask) | (p.OpenBus & 0xC0)
			openBusMask = 0xC0
		} else {
			openBusMask = 0x00
		}
	}
	return returnValue | (p.OpenBus & openBusMask)
}

// StatusByte exposes the packed $2002 status bits (for debuggers/tracing).
func (p *PPU) StatusByte() byte { return statusByte(p.Status) }

// statusByte packs the three PPU status flags into $2002's top bits.
func statusByte(s StatusFlags) byte {
	var v byte
	if s.SpriteOverflow {
		v |= 0x20
	}
	if s.Sprite0Hit {
		v |= 0x40
	}
	if s.VerticalBlank {
		v |= 0x80
	}
	return v
}

// updateStatusFlag clears the vblank flag on a $2002 read and handles the
// read-one-clock-before-set race.
func (p *PPU) updateStatusFlag() {
	p.Status.VerticalBlank = false
	p.clearNmiFlag()
	if p.Scanline == p.nmiScanline && p.Cycle == 0 {
		// Reading one PPU clock before the flag rises reads it clear and
		// prevents it from setting (or an NMI) that frame.
		p.PreventVblFlag = true
	}
}

// WriteRegister services a CPU write to a PPU register.
func (p *PPU) WriteRegister(addr uint16, v byte) {
	p.setOpenBus(0xFF, v)

	switch addr & 0x07 {
	case 0: // Control
		p.setControlRegister(v)
	case 1: // Mask
		p.setMaskRegister(v)
	case 2: // Status: read-only
	case 3: // SpriteAddr
		p.SpriteRAMAddr = v
	case 4: // SpriteData
		if p.Scanline >= 240 || !p.isRenderingEnabled() {
			if p.SpriteRAMAddr&0x03 == 0x02 {
				v &= 0xE3
			}
			p.writeSpriteRAM(p.SpriteRAMAddr, v)
			p.SpriteRAMAddr++
		} else {
			// During rendering: no OAM write, glitchy high-6-bit increment.
			p.SpriteRAMAddr = (p.SpriteRAMAddr + 4) & 0xFC
		}
	case 5: // ScrollOffsets
		if !p.AllowFullPpuAccess {
			return
		}
		if p.WriteToggle {
			p.TmpVideoRAMAddr = (p.TmpVideoRAMAddr &^ 0x73E0) | (uint16(v&0xF8) << 2) | (uint16(v&0x07) << 12)
		} else {
			p.XScroll = v & 0x07
			p.TmpVideoRAMAddr = (p.TmpVideoRAMAddr &^ 0x001F) | uint16(v>>3)
		}
		p.WriteToggle = !p.WriteToggle
	case 6: // VideoMemoryAddr
		if !p.AllowFullPpuAccess {
			return
		}
		if p.WriteToggle {
			p.TmpVideoRAMAddr = (p.TmpVideoRAMAddr &^ 0x00FF) | uint16(v)
			// The vram address update is delayed 3 PPU cycles (Visual NES).
			p.NeedStateUpdate = true
			p.UpdateVramAddrDelay = 3
			p.UpdateVramAddr = p.TmpVideoRAMAddr
		} else {
			p.TmpVideoRAMAddr = (p.TmpVideoRAMAddr &^ 0xFF00) | (uint16(v&0x3F) << 8)
		}
		p.WriteToggle = !p.WriteToggle
	case 7: // VideoMemoryData
		// The write reaches VRAM 2 PPU cycles after the CPU write ends.
		p.MemoryDataWriteStateMachine = 5
		p.MemoryDataWriteLatch = v
		p.NeedStateUpdate = true
	}
}

func (p *PPU) setControlRegister(v byte) {
	if !p.AllowFullPpuAccess {
		return
	}
	nameTable := uint16(v & 0x03)
	p.TmpVideoRAMAddr = (p.TmpVideoRAMAddr &^ 0x0C00) | (nameTable << 10)

	p.Control.VerticalWrite = v&0x04 == 0x04
	if v&0x08 == 0x08 {
		p.Control.SpritePatternAddr = 0x1000
	} else {
		p.Control.SpritePatternAddr = 0x0000
	}
	if v&0x10 == 0x10 {
		p.Control.BackgroundPatternAddr = 0x1000
	} else {
		p.Control.BackgroundPatternAddr = 0x0000
	}
	p.Control.LargeSprites = v&0x20 == 0x20
	p.Control.NmiOnVerticalBlank = v&0x80 == 0x80

	// Toggling NMI enable during vblank pulses /NMI (multiple NMIs).
	if !p.Control.NmiOnVerticalBlank {
		p.clearNmiFlag()
	} else if p.Control.NmiOnVerticalBlank && p.Status.VerticalBlank {
		p.setNmiFlag()
	}
}

func (p *PPU) setMaskRegister(v byte) {
	if !p.AllowFullPpuAccess {
		return
	}
	p.Mask.Grayscale = v&0x01 == 0x01
	p.Mask.BackgroundMask = v&0x02 == 0x02
	p.Mask.SpriteMask = v&0x04 == 0x04
	p.Mask.BackgroundEnabled = v&0x08 == 0x08
	p.Mask.SpritesEnabled = v&0x10 == 0x10
	p.Mask.IntensifyBlue = v&0x80 == 0x80
	p.Mask.IntensifyRed = v&0x20 == 0x20
	p.Mask.IntensifyGreen = v&0x40 == 0x40

	if p.RenderingEnabled != (p.Mask.BackgroundEnabled || p.Mask.SpritesEnabled) {
		p.NeedStateUpdate = true
	}
	p.updateMinimumDrawCycles()
	p.updateColorBitMasks()
}

// --- OAM ---

func (p *PPU) readSpriteRAM(addr byte) byte { return p.SpriteRAM[addr] }
func (p *PPU) writeSpriteRAM(addr, v byte)  { p.SpriteRAM[addr] = v }

// --- Palette RAM ---

func (p *PPU) readPaletteRAM(addr uint16) byte {
	addr &= 0x1F
	if addr == 0x10 || addr == 0x14 || addr == 0x18 || addr == 0x1C {
		addr &^= 0x10
	}
	return p.PaletteRAM[addr]
}

func (p *PPU) writePaletteRAM(addr uint16, value byte) {
	addr &= 0x1F
	value &= 0x3F
	switch addr {
	case 0x00, 0x10:
		p.PaletteRAM[0x00] = value
		p.PaletteRAM[0x10] = value
	case 0x04, 0x14:
		p.PaletteRAM[0x04] = value
		p.PaletteRAM[0x14] = value
	case 0x08, 0x18:
		p.PaletteRAM[0x08] = value
		p.PaletteRAM[0x18] = value
	case 0x0C, 0x1C:
		p.PaletteRAM[0x0C] = value
		p.PaletteRAM[0x1C] = value
	default:
		p.PaletteRAM[addr] = value
	}
}

// --- Nametable mirroring ---

// vramIndex maps a nametable address to CIRAM through the board's
// mirroring. Four-screen boards address all 4 KiB directly.
func (p *PPU) vramIndex(addr uint16) uint16 {
	addr &= 0x0FFF
	table := addr / 0x400
	offset := addr & 0x03FF
	if p.ntPager != nil {
		return uint16(p.ntPager.NametablePage(byte(table)))*0x400 + offset
	}
	switch p.board.Mirroring() {
	case cartridge.Horizontal:
		return table/2*0x400 + offset
	case cartridge.Vertical:
		return table%2*0x400 + offset
	case cartridge.SingleLow:
		return offset
	case cartridge.SingleHigh:
		return 0x400 + offset
	default: // four-screen
		return addr
	}
}
