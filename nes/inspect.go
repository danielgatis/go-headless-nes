package nes

import "github.com/danielgatis/go-headless-nes/internal/errs"

// PeekVRAM reads one byte of the PPU address space ($0000-$3FFF) as a
// rendering fetch would resolve it: pattern tables through the mapper,
// nametables through CIRAM mirroring, palette RAM from the internal latch.
// It backs the pattern-table, nametable and palette viewers. On latch-based
// CHR boards (MMC2/MMC4) reading a pattern byte can flip the CHR latch, the
// same as a real fetch; read raw CHR through ReadCHR to avoid that.
func (c *Console) PeekVRAM(addr uint16) byte { return c.core.PPU.PeekVRAM(addr) }

// PokeVRAM writes one byte of the PPU address space ($0000-$3FFF): pattern
// tables on a CHR-RAM board, nametables through CIRAM mirroring, or palette
// RAM. Writes to pattern space on a CHR-ROM board are dropped, as on
// hardware.
func (c *Console) PokeVRAM(addr uint16, value byte) { c.core.PPU.PokeVRAM(addr, value) }

// PPUState is the decoded picture-unit state a register viewer shows: the
// two internal scroll addresses, the fine-X and write toggle, the raster
// position, and the decoded $2000/$2001/$2002 flags.
type PPUState struct {
	V, T          uint16 // the internal VRAM address (v) and its latch (t)
	FineX         byte   // fine-x scroll (0-7)
	WriteToggle   bool   // the $2005/$2006 shared toggle (w)
	Scanline, Dot int    // current raster position
	Frame         uint64

	// $2000 control
	NMIEnabled            bool
	LargeSprites          bool
	VerticalWrite         bool
	BackgroundPatternAddr uint16
	SpritePatternAddr     uint16

	// $2001 mask
	BackgroundEnabled bool
	SpritesEnabled    bool
	Grayscale         bool

	// $2002 status
	VerticalBlank  bool
	Sprite0Hit     bool
	SpriteOverflow bool
	Status         byte // the packed $2002 byte
}

// PPUState reports the current decoded picture-unit state.
func (c *Console) PPUState() PPUState {
	p := c.core.PPU
	return PPUState{
		V: p.VideoRAMAddr, T: p.TmpVideoRAMAddr, FineX: p.XScroll,
		WriteToggle: p.WriteToggle,
		Scanline:    int(p.Scanline), Dot: int(p.Cycle), Frame: p.Frame,
		NMIEnabled:            p.Control.NmiOnVerticalBlank,
		LargeSprites:          p.Control.LargeSprites,
		VerticalWrite:         p.Control.VerticalWrite,
		BackgroundPatternAddr: p.Control.BackgroundPatternAddr,
		SpritePatternAddr:     p.Control.SpritePatternAddr,
		BackgroundEnabled:     p.Mask.BackgroundEnabled,
		SpritesEnabled:        p.Mask.SpritesEnabled,
		Grayscale:             p.Mask.Grayscale,
		VerticalBlank:         p.Status.VerticalBlank,
		Sprite0Hit:            p.Status.Sprite0Hit,
		SpriteOverflow:        p.Status.SpriteOverflow,
		Status:                p.StatusByte(),
	}
}

// CartInfo is the fixed description of the loaded cartridge a "cartridge
// information" panel shows.
type CartInfo struct {
	MapperID   uint16 // iNES/NES 2.0 mapper number
	Submapper  byte   // NES 2.0 submapper; 0 otherwise
	PRGSize    int    // program ROM size in bytes
	CHRSize    int    // character ROM size in bytes; 0 means CHR RAM
	CHRRAM     bool   // the board provides CHR RAM instead of CHR ROM
	Mirroring  string // "Horizontal", "Vertical", "SingleLow", ...
	HasBattery bool   // battery-backed save RAM present
	Region     Region // TV system declared by the header
}

// CartInfo reports the loaded cartridge's fixed properties.
func (c *Console) CartInfo() CartInfo {
	cart := c.core.Cart
	return CartInfo{
		MapperID:   cart.MapperID,
		Submapper:  cart.Submapper,
		PRGSize:    len(cart.PRG),
		CHRSize:    len(cart.CHR),
		CHRRAM:     len(cart.CHR) == 0,
		Mirroring:  cart.Mirroring.String(),
		HasBattery: cart.HasBattery,
		Region:     Region(cart.Region),
	}
}

// OAM copies the 256 bytes of primary object-attribute memory: the sprite
// table, four bytes per sprite (Y, tile, attributes, X). The returned
// slice is a copy; edit it and hand it back through SetOAM.
func (c *Console) OAM() []byte {
	oam := c.core.PPU.SpriteRAM
	return oam[:]
}

// SetOAM overwrites primary OAM from data (up to 256 bytes; a shorter
// slice leaves the tail untouched).
func (c *Console) SetOAM(data []byte) {
	copy(c.core.PPU.SpriteRAM[:], data)
}

// PaletteRAM copies the 32 palette entries ($3F00-$3F1F): the background
// palette, then the sprite palette. Each byte is an NES color index.
func (c *Console) PaletteRAM() []byte {
	pal := c.core.PPU.PaletteRAM
	return pal[:]
}

// SetPaletteRAM overwrites palette RAM from data (up to 32 bytes).
func (c *Console) SetPaletteRAM(data []byte) {
	copy(c.core.PPU.PaletteRAM[:], data)
}

// SetRegister writes one CPU register by name: "A", "X", "Y", "SP", "P"
// (the eight-bit registers, whose value is taken modulo 256) or "PC" (the
// full sixteen-bit program counter). It is how a debugger's variable
// editor and a cheat both change CPU state. An unknown name is an error.
func (c *Console) SetRegister(name string, value uint16) error {
	r := &c.core.CPU.Reg
	switch name {
	case "A":
		r.A = byte(value)
	case "X":
		r.X = byte(value)
	case "Y":
		r.Y = byte(value)
	case "SP":
		r.SP = byte(value)
	case "P":
		r.P = byte(value)
	case "PC":
		r.PC = value
	default:
		return errs.Errorf("unknown register %q", name)
	}
	return nil
}
