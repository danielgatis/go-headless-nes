package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// UxROM (mapper 2) banks 16 KiB of PRG at $8000 through a discrete
// latch; the last bank is hardwired at $C000. CHR is 8 KiB of RAM.
// The board has no bus-conflict protection: the latch sees the write
// value ANDed with the ROM byte at the same address, and games are
// assembled so the two agree.
type UxROM struct {
	base
	bank byte
}

// NewUxROM wires a UxROM board.
func NewUxROM(c *cartridge.Cartridge) *UxROM {
	return &UxROM{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *UxROM) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.bank), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *UxROM) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.bank = v & m.ReadPRG(addr) // bus conflict
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *UxROM) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *UxROM) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *UxROM) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bank
}

// Restore loads the board's mapper-specific state from s.
func (m *UxROM) Restore(s *State) {
	m.restoreRAM(s)
	m.bank = s.Regs[0]
}
