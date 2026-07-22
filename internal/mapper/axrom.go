package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// AxROM (mapper 7) banks 32 KiB of PRG at $8000 and selects which
// single nametable is visible — Battletoads-style boards. CHR is 8 KiB
// of RAM. (The AMROM revision has bus conflicts; ANROM does not. We
// model the conflict-free revision, which every mapper-7 game runs on.)
type AxROM struct {
	base
	reg byte
}

// NewAxROM wires an AxROM board.
func NewAxROM(c *cartridge.Cartridge) *AxROM {
	return &AxROM{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *AxROM) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return window(m.prg, int(m.reg&7), 0x8000)[addr&0x7FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *AxROM) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.reg = v
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *AxROM) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *AxROM) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *AxROM) Mirroring() cartridge.Mirroring {
	if m.reg&0x10 != 0 {
		return cartridge.SingleHigh
	}
	return cartridge.SingleLow
}

// Save writes the board's mapper-specific state into s.
func (m *AxROM) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.reg
}

// Restore loads the board's mapper-specific state from s.
func (m *AxROM) Restore(s *State) {
	m.restoreRAM(s)
	m.reg = s.Regs[0]
}
