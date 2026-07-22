package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Sunsoft4 (mapper 68, Sunsoft-4): banks PRG in a single 16 KiB window
// (the second 16 KiB is fixed to the last bank), CHR in four 2 KiB
// windows, and adds register-controlled single-screen mirroring. Its
// signature feature is using two CHR-ROM banks as nametables, which we
// serve through ReadNT/WriteNT.
//
// The board's licensing timer and external/expansion PRG mode (an
// anti-piracy measure that briefly unmaps PRG until $6000-$7FFF is
// written) is not modelled: legitimate dumps run with internal ROM, which
// is the default here.
type Sunsoft4 struct {
	base

	prgBank  byte    // 16 KiB bank at $8000
	chrBanks [4]byte // 2 KiB CHR windows
	ntRegs   [2]byte // CHR-ROM banks usable as nametables
	ntMode   bool    // use CHR ROM for nametables
	mirror   cartridge.Mirroring
}

// NewSunsoft4 wires the board.
func NewSunsoft4(c *cartridge.Cartridge) *Sunsoft4 {
	return &Sunsoft4{base: makeBase(c), mirror: c.Mirroring}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sunsoft4) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF] // fixed last 16 KiB
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBank&0x0F), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sunsoft4) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
		return
	}
	if addr < 0x8000 {
		return
	}
	switch addr & 0xF000 {
	case 0x8000:
		m.chrBanks[0] = v
	case 0x9000:
		m.chrBanks[1] = v
	case 0xA000:
		m.chrBanks[2] = v
	case 0xB000:
		m.chrBanks[3] = v
	case 0xC000:
		m.ntRegs[0] = v | 0x80
	case 0xD000:
		m.ntRegs[1] = v | 0x80
	case 0xE000:
		switch v & 0x03 {
		case 0:
			m.mirror = cartridge.Vertical
		case 1:
			m.mirror = cartridge.Horizontal
		case 2:
			m.mirror = cartridge.SingleLow
		case 3:
			m.mirror = cartridge.SingleHigh
		}
		m.ntMode = v&0x10 != 0
	case 0xF000:
		m.prgBank = v & 0x07
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sunsoft4) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>11&3]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sunsoft4) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>11&3]), 0x800, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Sunsoft4) Mirroring() cartridge.Mirroring { return m.mirror }

// ntPick maps a nametable slot to one of the two NT registers per the
// current mirroring, matching the reference's nametable update.
func (m *Sunsoft4) ntPick(table byte) byte {
	switch m.mirror {
	case cartridge.Vertical:
		return table & 0x01
	case cartridge.Horizontal:
		return (table & 0x02) >> 1
	case cartridge.SingleHigh:
		return 1
	default: // SingleLow
		return 0
	}
}

// ReadNT serves nametable fetches from CHR ROM when the board is in
// CHR-nametable mode; otherwise CIRAM handles them.
func (m *Sunsoft4) ReadNT(addr uint16) (byte, bool) {
	if !m.ntMode {
		return 0, false
	}
	reg := m.ntRegs[m.ntPick(byte(addr>>10&3))]
	return m.chrRead(int(reg), 0x400, addr), true
}

// WriteNT handles a nametable write the board intercepts.
func (m *Sunsoft4) WriteNT(addr uint16, v byte) bool {
	if !m.ntMode {
		return false
	}
	reg := m.ntRegs[m.ntPick(byte(addr>>10&3))]
	m.chrWrite(int(reg), 0x400, addr, v)
	return true
}

// NametablePage picks the CIRAM page when not using CHR nametables.
func (m *Sunsoft4) NametablePage(table byte) byte {
	switch m.mirror {
	case cartridge.Horizontal:
		return table >> 1
	case cartridge.SingleHigh:
		return 1
	case cartridge.SingleLow:
		return 0
	default: // Vertical
		return table & 1
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Sunsoft4) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	copy(s.Regs[1:5], m.chrBanks[:])
	s.Regs[5] = m.ntRegs[0]
	s.Regs[6] = m.ntRegs[1]
	s.Regs[7] = boolByte(m.ntMode)
	s.Regs[8] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sunsoft4) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	copy(m.chrBanks[:], s.Regs[1:5])
	m.ntRegs[0] = s.Regs[5]
	m.ntRegs[1] = s.Regs[6]
	m.ntMode = s.Regs[7] != 0
	m.mirror = cartridge.Mirroring(s.Regs[8])
}
