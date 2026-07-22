package ppu

import "github.com/danielgatis/go-headless-nes/internal/serial"

// This file serializes the PPU State without reflection, covering every
// field (many unexported render-pipeline latches). state_serial_test.go
// round-trips it so a forgotten field fails immediately.

func (t *TileInfo) append(w *serial.Writer) {
	w.U16(t.TileAddr)
	w.Byte(t.LowByte)
	w.Byte(t.HighByte)
	w.Byte(t.PaletteOffset)
}
func (t *TileInfo) read(r *serial.Reader) {
	t.TileAddr = r.U16()
	t.LowByte = r.Byte()
	t.HighByte = r.Byte()
	t.PaletteOffset = r.Byte()
}

func (c *ControlFlags) append(w *serial.Writer) {
	w.U16(c.BackgroundPatternAddr)
	w.U16(c.SpritePatternAddr)
	w.Bool(c.VerticalWrite)
	w.Bool(c.LargeSprites)
	w.Bool(c.NmiOnVerticalBlank)
}
func (c *ControlFlags) read(r *serial.Reader) {
	c.BackgroundPatternAddr = r.U16()
	c.SpritePatternAddr = r.U16()
	c.VerticalWrite = r.Bool()
	c.LargeSprites = r.Bool()
	c.NmiOnVerticalBlank = r.Bool()
}

func (m *MaskFlags) append(w *serial.Writer) {
	w.Bool(m.Grayscale)
	w.Bool(m.BackgroundMask)
	w.Bool(m.SpriteMask)
	w.Bool(m.BackgroundEnabled)
	w.Bool(m.SpritesEnabled)
	w.Bool(m.IntensifyRed)
	w.Bool(m.IntensifyGreen)
	w.Bool(m.IntensifyBlue)
}
func (m *MaskFlags) read(r *serial.Reader) {
	m.Grayscale = r.Bool()
	m.BackgroundMask = r.Bool()
	m.SpriteMask = r.Bool()
	m.BackgroundEnabled = r.Bool()
	m.SpritesEnabled = r.Bool()
	m.IntensifyRed = r.Bool()
	m.IntensifyGreen = r.Bool()
	m.IntensifyBlue = r.Bool()
}

func (s *StatusFlags) append(w *serial.Writer) {
	w.Bool(s.SpriteOverflow)
	w.Bool(s.Sprite0Hit)
	w.Bool(s.VerticalBlank)
}
func (s *StatusFlags) read(r *serial.Reader) {
	s.SpriteOverflow = r.Bool()
	s.Sprite0Hit = r.Bool()
	s.VerticalBlank = r.Bool()
}

func (si *SpriteInfo) append(w *serial.Writer) {
	w.Bool(si.BackgroundPriority)
	w.Byte(si.SpriteX)
	w.Byte(si.LowByte)
	w.Byte(si.HighByte)
	w.Byte(si.PaletteOffset)
}
func (si *SpriteInfo) read(r *serial.Reader) {
	si.BackgroundPriority = r.Bool()
	si.SpriteX = r.Byte()
	si.LowByte = r.Byte()
	si.HighByte = r.Byte()
	si.PaletteOffset = r.Byte()
}

// Append writes the whole PPU State in a fixed field order.
func (s *State) Append(w *serial.Writer) {
	w.I32(s.Cycle)
	w.I16(s.Scanline)
	w.U64(s.Frame)

	w.U16(s.VideoRAMAddr)
	w.U16(s.TmpVideoRAMAddr)
	w.U16(s.HighBitShift)
	w.U16(s.LowBitShift)
	w.Byte(s.SpriteRAMAddr)
	w.Byte(s.OpenBus)
	w.Byte(s.XScroll)
	w.Bool(s.WriteToggle)

	w.Bool(s.NeedStateUpdate)
	w.Bool(s.RenderingEnabled)
	w.Bool(s.PrevRenderingEnabled)

	w.Bool(s.Sprite0Visible)
	w.Byte(s.SpriteCount)
	w.Byte(s.SecondaryOamAddr)
	w.Byte(s.OamCopybuffer)
	w.Bool(s.SpriteInRange)
	w.Bool(s.Sprite0Added)
	w.Byte(s.OverflowBugCounter)
	w.Bool(s.OamCopyDone)
	w.U16(s.PpuBusAddress)

	w.Byte(s.FirstVisibleSpriteAddr)
	w.Byte(s.LastVisibleSpriteAddr)

	s.Tile.append(w)
	w.Byte(s.CurrentTilePalette)
	w.Byte(s.PreviousTilePalette)
	w.Byte(s.PaletteRAMMask)

	w.U32(s.SpriteIndex)
	w.U16(s.UpdateVramAddr)
	w.Byte(s.UpdateVramAddrDelay)
	w.Bool(s.PreventVblFlag)

	s.Control.append(w)
	s.Mask.append(w)
	s.Status.append(w)

	for i := range s.SpriteTiles {
		s.SpriteTiles[i].append(w)
	}

	for _, v := range s.SpriteShifterList {
		w.U16(v)
	}
	w.Byte(s.NextSpriteShifter)
	w.U16(s.NextSpriteShifterCycle)
	w.Byte(s.ActiveSpriteShifters)
	w.Byte(s.CountingSpriteShifters)
	w.Byte(s.ExpiredSpriteShifters)
	w.Byte(s.DotSkipped)
	w.Bool(s.ProcessSprites)

	w.U16(s.MinimumDrawBgCycle)
	w.U16(s.MinimumDrawSpriteCycle)
	w.U16(s.MinimumDrawSpriteStandardCycle)

	w.Bool(s.NeedVideoRAMIncrement)
	w.Bool(s.AllowFullPpuAccess)

	w.Byte(s.MemoryDataReadStateMachine)
	w.Byte(s.MemoryDataWriteStateMachine)
	w.Byte(s.MemoryDataWriteLatch)
	w.Byte(s.MemoryReadBuffer)
	w.U32(s.IgnoreVramRead)

	for _, v := range s.OpenBusDecayStamp {
		w.U32(v)
	}

	w.Bytes(s.PaletteRAM[:])
	w.Bytes(s.SpriteRAM[:])
	w.Bytes(s.SecondarySpriteRAM[:])
	w.Bytes(s.VRAM[:])

	w.Bool(s.A12)
	w.Byte(s.A12LowDots)
	w.Byte(s.BusDataPins)
	w.U64(s.MasterClock)
}

// Read restores the whole PPU State in Append's order.
func (s *State) Read(r *serial.Reader) {
	s.Cycle = r.I32()
	s.Scanline = r.I16()
	s.Frame = r.U64()

	s.VideoRAMAddr = r.U16()
	s.TmpVideoRAMAddr = r.U16()
	s.HighBitShift = r.U16()
	s.LowBitShift = r.U16()
	s.SpriteRAMAddr = r.Byte()
	s.OpenBus = r.Byte()
	s.XScroll = r.Byte()
	s.WriteToggle = r.Bool()

	s.NeedStateUpdate = r.Bool()
	s.RenderingEnabled = r.Bool()
	s.PrevRenderingEnabled = r.Bool()

	s.Sprite0Visible = r.Bool()
	s.SpriteCount = r.Byte()
	s.SecondaryOamAddr = r.Byte()
	s.OamCopybuffer = r.Byte()
	s.SpriteInRange = r.Bool()
	s.Sprite0Added = r.Bool()
	s.OverflowBugCounter = r.Byte()
	s.OamCopyDone = r.Bool()
	s.PpuBusAddress = r.U16()

	s.FirstVisibleSpriteAddr = r.Byte()
	s.LastVisibleSpriteAddr = r.Byte()

	s.Tile.read(r)
	s.CurrentTilePalette = r.Byte()
	s.PreviousTilePalette = r.Byte()
	s.PaletteRAMMask = r.Byte()

	s.SpriteIndex = r.U32()
	s.UpdateVramAddr = r.U16()
	s.UpdateVramAddrDelay = r.Byte()
	s.PreventVblFlag = r.Bool()

	s.Control.read(r)
	s.Mask.read(r)
	s.Status.read(r)

	for i := range s.SpriteTiles {
		s.SpriteTiles[i].read(r)
	}

	for i := range s.SpriteShifterList {
		s.SpriteShifterList[i] = r.U16()
	}
	s.NextSpriteShifter = r.Byte()
	s.NextSpriteShifterCycle = r.U16()
	s.ActiveSpriteShifters = r.Byte()
	s.CountingSpriteShifters = r.Byte()
	s.ExpiredSpriteShifters = r.Byte()
	s.DotSkipped = r.Byte()
	s.ProcessSprites = r.Bool()

	s.MinimumDrawBgCycle = r.U16()
	s.MinimumDrawSpriteCycle = r.U16()
	s.MinimumDrawSpriteStandardCycle = r.U16()

	s.NeedVideoRAMIncrement = r.Bool()
	s.AllowFullPpuAccess = r.Bool()

	s.MemoryDataReadStateMachine = r.Byte()
	s.MemoryDataWriteStateMachine = r.Byte()
	s.MemoryDataWriteLatch = r.Byte()
	s.MemoryReadBuffer = r.Byte()
	s.IgnoreVramRead = r.U32()

	for i := range s.OpenBusDecayStamp {
		s.OpenBusDecayStamp[i] = r.U32()
	}

	r.ReadBytes(s.PaletteRAM[:])
	r.ReadBytes(s.SpriteRAM[:])
	r.ReadBytes(s.SecondarySpriteRAM[:])
	r.ReadBytes(s.VRAM[:])

	s.A12 = r.Bool()
	s.A12LowDots = r.Byte()
	s.BusDataPins = r.Byte()
	s.MasterClock = r.U64()
}
