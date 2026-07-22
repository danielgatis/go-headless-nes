package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// CNROM (mapper 3) is NROM with an 8 KiB CHR ROM bank latch. Like
// UxROM it has bus conflicts: the latch sees the write ANDed with ROM.
type CNROM struct {
	base
	bank byte
}

// NewCNROM wires a CNROM board.
func NewCNROM(c *cartridge.Cartridge) *CNROM {
	return &CNROM{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *CNROM) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return m.prg[int(addr-0x8000)%len(m.prg)]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *CNROM) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.bank = v & m.ReadPRG(addr) // bus conflict
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *CNROM) ReadCHR(addr uint16) byte { return m.chrRead(int(m.bank), 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *CNROM) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.bank), 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *CNROM) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bank
}

// Restore loads the board's mapper-specific state from s.
func (m *CNROM) Restore(s *State) {
	m.restoreRAM(s)
	m.bank = s.Regs[0]
}
