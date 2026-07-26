package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// The MMC3 multicarts wrap a stock MMC3 with an outer-bank register:
// the inner MMC3 banks pass through a translation before they reach
// the ROM. Negative (fixed) PRG banks translate with all their bits
// set, exactly as the unwired upper address lines behave on hardware.

// canWriteWRAM reports the MMC3's $A001 write gate, which the
// multicart registers at $6000-$7FFF sit behind.
func (m *MMC3) canWriteWRAM() bool { return m.ramEnabled && !m.ramWriteProtect }

// MMC3Multi37 (mapper 37, Super Mario Bros + Tetris + World Cup).
type MMC3Multi37 struct {
	MMC3

	block byte
}

// NewMMC3Multi37 wires the board.
func NewMMC3Multi37(c *cartridge.Cartridge) *MMC3Multi37 {
	return &MMC3Multi37{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Multi37) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if m.canWriteWRAM() {
			m.block = v & 0x07
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Multi37) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	page := m.prgBank(addr) & 0x3F
	switch {
	case m.block <= 2:
		page &= 0x07
	case m.block == 3:
		page = page&0x07 | 0x08
	case m.block == 7:
		page = page&0x07 | 0x20
	default: // 4-6
		page = page&0x0F | 0x10
	}
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3Multi37) chrPage(addr uint16) int {
	page := m.chrPage1K(addr)
	if m.block >= 4 {
		page |= 0x80
	}
	return page
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Multi37) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Multi37) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3Multi37) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.block
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Multi37) Restore(s *State) {
	m.MMC3.Restore(s)
	m.block = s.Regs[17]
}

// MMC3Multi44 (mapper 44, Super Big 7-in-1): the block select rides on
// the MMC3's own $A001 register.
type MMC3Multi44 struct {
	MMC3

	block byte
}

// NewMMC3Multi44 wires the board.
func NewMMC3Multi44(c *cartridge.Cartridge) *MMC3Multi44 {
	return &MMC3Multi44{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Multi44) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 && addr&0xE001 == 0xA001 {
		m.block = v & 0x07
		if m.block == 7 {
			m.block = 6
		}
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Multi44) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	page := m.prgBank(addr) & 0x3F
	if m.block <= 5 {
		page &= 0x0F
	} else {
		page &= 0x1F
	}
	page |= int(m.block) * 0x10
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3Multi44) chrPage(addr uint16) int {
	page := m.chrPage1K(addr)
	if m.block <= 5 {
		page &= 0x7F
	} else {
		page &= 0xFF
	}
	return page | int(m.block)*0x80
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Multi44) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Multi44) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3Multi44) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.block
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Multi44) Restore(s *State) {
	m.MMC3.Restore(s)
	m.block = s.Regs[17]
}

// MMC3Multi45 (mapper 45): four sequential writes to $6000 set the
// outer registers (CHR offset/mask, PRG offset/mask and a lock bit).
type MMC3Multi45 struct {
	MMC3

	regIndex byte
	reg      [4]byte
}

// NewMMC3Multi45 wires the board.
func NewMMC3Multi45(c *cartridge.Cartridge) *MMC3Multi45 {
	m := &MMC3Multi45{MMC3: *NewMMC3(c)}
	// Some games draw with CHR RAM before initializing the registers.
	m.banks = [8]byte{0, 2, 4, 5, 6, 7, 0, 0}
	m.reg[2] = 0x0F
	return m
}

func (m *MMC3Multi45) locked() bool { return m.reg[3]&0x40 != 0 }

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Multi45) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if !m.locked() {
			m.reg[m.regIndex] = v
			m.regIndex = (m.regIndex + 1) & 0x03
			return
		}
		// Locked: the window behaves as normal work RAM again.
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Multi45) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	page := m.prgBank(addr) & 0x3F
	page &= 0x3F ^ int(m.reg[3]&0x3F)
	page |= int(m.reg[1])
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3Multi45) chrPage(addr uint16) int {
	page := m.chrPage1K(addr)
	if m.chr != nil {
		page &= 0xFF >> (0x0F - (m.reg[2] & 0x0F))
		page |= int(m.reg[0]) | int(m.reg[2]&0xF0)<<4
	}
	return page
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Multi45) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Multi45) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3Multi45) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.regIndex
	copy(s.Regs[18:22], m.reg[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Multi45) Restore(s *State) {
	m.MMC3.Restore(s)
	m.regIndex = s.Regs[17]
	copy(m.reg[:], s.Regs[18:22])
}

// MMC3Multi47 (mapper 47, Super Spike V'Ball + Nintendo World Cup).
type MMC3Multi47 struct {
	MMC3

	block byte
}

// NewMMC3Multi47 wires the board.
func NewMMC3Multi47(c *cartridge.Cartridge) *MMC3Multi47 {
	return &MMC3Multi47{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Multi47) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if m.canWriteWRAM() {
			m.block = v & 0x01
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Multi47) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	page := m.prgBank(addr)&0x0F | int(m.block)<<4
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3Multi47) chrPage(addr uint16) int {
	return m.chrPage1K(addr)&0x7F | int(m.block)<<7
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Multi47) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Multi47) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3Multi47) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.block
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Multi47) Restore(s *State) {
	m.MMC3.Restore(s)
	m.block = s.Regs[17]
}

// MMC3Multi49 (mapper 49, Super HiK 4-in-1): a $6000 register selects
// the block plus an alternate 32 KiB PRG mode.
type MMC3Multi49 struct {
	MMC3

	block   byte
	prgReg  byte
	prgMode bool
}

// NewMMC3Multi49 wires the board.
func NewMMC3Multi49(c *cartridge.Cartridge) *MMC3Multi49 {
	return &MMC3Multi49{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Multi49) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if m.canWriteWRAM() {
			m.block = (v >> 6) & 0x03
			m.prgReg = (v >> 4) & 0x03
			m.prgMode = v&0x01 != 0
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Multi49) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	slot := int((addr - 0x8000) >> 13)
	var page int
	if m.prgMode {
		page = m.prgBank(addr)&0x0F | int(m.block)*0x10
	} else {
		page = int(m.prgReg)*4 + slot
	}
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3Multi49) chrPage(addr uint16) int {
	return m.chrPage1K(addr)&0x7F | int(m.block)*0x80
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Multi49) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Multi49) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3Multi49) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.block
	s.Regs[18] = m.prgReg
	s.Regs[19] = boolByte(m.prgMode)
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Multi49) Restore(s *State) {
	m.MMC3.Restore(s)
	m.block = s.Regs[17]
	m.prgReg = s.Regs[18]
	m.prgMode = s.Regs[19] != 0
}

// MMC3Multi52 (mapper 52, Mario Party 7-in-1): a $6000 register with a
// lock bit that turns the window back into work RAM.
type MMC3Multi52 struct {
	MMC3

	extra byte
}

// NewMMC3Multi52 wires the board.
func NewMMC3Multi52(c *cartridge.Cartridge) *MMC3Multi52 {
	return &MMC3Multi52{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Multi52) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if m.canWriteWRAM() {
			if m.extra&0x80 == 0 {
				m.extra = v
			} else {
				m.writePRGRAM(addr, v)
			}
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Multi52) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	page := m.prgBank(addr) & 0x3F
	if m.extra&0x08 != 0 {
		page = page&0x0F | int(m.extra&0x07)<<4
	} else {
		page = page&0x1F | int(m.extra&0x06)<<4
	}
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3Multi52) chrPage(addr uint16) int {
	page := m.chrPage1K(addr)
	if m.extra&0x40 != 0 {
		page = page&0x7F | (int(m.extra&0x04)|int(m.extra>>4)&0x03)<<7
	} else {
		page = page&0xFF | (int(m.extra&0x04)|int(m.extra>>4)&0x02)<<7
	}
	return page
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Multi52) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Multi52) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3Multi52) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.extra
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Multi52) Restore(s *State) {
	m.MMC3.Restore(s)
	m.extra = s.Regs[17]
}
