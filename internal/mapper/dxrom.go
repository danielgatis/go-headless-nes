package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// DxROM (mapper 206, Namco 118) is the MMC3's ancestor: the same
// $8000/$8001 index/data banking scheme but with no IRQ counter, no
// mirroring control, no PRG swap mode and no CHR inversion, those
// select bits are simply not connected.
type DxROM struct {
	base

	bankSelect byte
	banks      [8]byte
}

// NewDxROM wires a DxROM board.
func NewDxROM(c *cartridge.Cartridge) *DxROM {
	return &DxROM{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *DxROM) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.openBus()
	}
	var bank int
	switch addr >> 13 & 3 {
	case 0:
		bank = int(m.banks[6] & 0x0F)
	case 1:
		bank = int(m.banks[7] & 0x0F)
	case 2:
		bank = -2
	default:
		bank = -1
	}
	return m.win(m.prg, bank, 0x2000)[addr&0x1FFF]
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *DxROM) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	switch addr & 0xE001 {
	case 0x8000:
		m.bankSelect = v & 7
	case 0x8001:
		m.banks[m.bankSelect] = v
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *DxROM) ReadCHR(addr uint16) byte {
	bank, size := m.chrWindow(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *DxROM) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrWindow(addr)
	m.chrWrite(bank, size, addr, v)
}

func (m *DxROM) chrWindow(addr uint16) (bank, size int) {
	if addr < 0x1000 {
		r := addr >> 11 & 1
		return int(m.banks[r]&0x3E) >> 1, 2048
	}
	return int(m.banks[2+(addr>>10)&3] & 0x3F), 1024
}

// Mirroring reports the board's current nametable mirroring.
func (m *DxROM) Mirroring() cartridge.Mirroring { return m.mirroring }

// Save writes the board's mapper-specific state into s.
func (m *DxROM) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bankSelect
	copy(s.Regs[1:9], m.banks[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *DxROM) Restore(s *State) {
	m.restoreRAM(s)
	m.bankSelect = s.Regs[0]
	copy(m.banks[:], s.Regs[1:9])
}
