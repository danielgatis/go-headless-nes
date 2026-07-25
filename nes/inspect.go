package nes

import "github.com/danielgatis/go-headless-nes/internal/errs"

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
