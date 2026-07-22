package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// The discrete latch boards: one or two 74-series registers, no IRQs,
// no fancy timing. Each type below is one physical board family.

// ColorDreams (mapper 11): one latch selects a 32 KiB PRG bank (low
// bits) and an 8 KiB CHR bank (high nibble). Has bus conflicts.
type ColorDreams struct {
	base
	reg byte
}

// NewColorDreams constructs the corresponding board.
func NewColorDreams(c *cartridge.Cartridge) *ColorDreams {
	return &ColorDreams{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *ColorDreams) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.reg&3), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *ColorDreams) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.reg = v & m.ReadPRG(addr) // bus conflict
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *ColorDreams) ReadCHR(addr uint16) byte { return m.chrRead(int(m.reg>>4), 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *ColorDreams) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.reg>>4), 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *ColorDreams) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *ColorDreams) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// GxROM (mapper 66): one latch, 32 KiB PRG bank in bits 4-5 and 8 KiB
// CHR bank in bits 0-1. Has bus conflicts.
type GxROM struct {
	base
	reg byte
}

// NewGxROM constructs the corresponding board.
func NewGxROM(c *cartridge.Cartridge) *GxROM {
	return &GxROM{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *GxROM) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.reg>>4&3), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *GxROM) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.reg = v & m.ReadPRG(addr) // bus conflict
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *GxROM) ReadCHR(addr uint16) byte { return m.chrRead(int(m.reg&3), 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *GxROM) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.reg&3), 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *GxROM) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *GxROM) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// Camerica (mapper 71): UxROM-alike with the bank latch at $C000-$FFFF.
// The BF9097 board (Fire Hawk, NES 2.0 submapper 1) adds one-screen
// mirroring control at $8000-$9FFF bit 4; the common BF9093 decodes
// nothing there.
type Camerica struct {
	base
	fireHawk bool // BF9097: the mirroring register exists
	bank     byte
	single   bool // a $8000-$9FFF write engaged one-screen mode
	high     bool
}

// NewCamerica constructs the corresponding board.
func NewCamerica(c *cartridge.Cartridge) *Camerica {
	return &Camerica{base: makeBase(c), fireHawk: c.Submapper == 1}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Camerica) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.bank), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Camerica) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0xC000:
		m.bank = v
	case addr >= 0x8000 && addr < 0xA000:
		if m.fireHawk {
			m.single = true
			m.high = v&0x10 != 0
		}
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Camerica) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Camerica) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Camerica) Mirroring() cartridge.Mirroring {
	if m.single {
		if m.high {
			return cartridge.SingleHigh
		}
		return cartridge.SingleLow
	}
	return m.mirroring
}

// Save writes the board's mapper-specific state into s.
func (m *Camerica) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bank
	s.Regs[1] = boolByte(m.single)
	s.Regs[2] = boolByte(m.high)
}

// Restore loads the board's mapper-specific state from s.
func (m *Camerica) Restore(s *State) {
	m.restoreRAM(s)
	m.bank = s.Regs[0]
	m.single = s.Regs[1] != 0
	m.high = s.Regs[2] != 0
}

// NINA03 (mappers 79, 146 as NINA-03/06; 113 as the multicart revision):
// the register sits in the $4100-$5FFF expansion range, decoded when
// address bit 8 is set. The multicart revision (113) widens the CHR bank
// to 4 bits and adds a register-controlled mirroring bit; the plain board
// (79/146) ignores both.
type NINA03 struct {
	base
	reg       byte
	multicart bool
}

// NewNINA03 constructs the corresponding board.
func NewNINA03(c *cartridge.Cartridge) *NINA03 {
	m := &NINA03{base: makeBase(c), multicart: c.MapperID == 113}
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *NINA03) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.reg>>3&1), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *NINA03) WritePRG(addr uint16, v byte) {
	if addr >= 0x4100 && addr < 0x6000 && addr&0x0100 != 0 {
		m.reg = v
	}
}

func (m *NINA03) chrBank() int {
	if m.multicart {
		return int(m.reg&0x07) | int(m.reg>>3)&0x08
	}
	return int(m.reg & 7)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *NINA03) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank(), 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *NINA03) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank(), 8192, addr, v) }

// Mirroring on the multicart revision follows register bit 7; the plain
// board keeps its solder-pad mirroring.
func (m *NINA03) Mirroring() cartridge.Mirroring {
	if m.multicart {
		if m.reg&0x80 != 0 {
			return cartridge.Vertical
		}
		return cartridge.Horizontal
	}
	return m.base.Mirroring()
}

// Save writes the board's mapper-specific state into s.
func (m *NINA03) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *NINA03) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// Jaleco87 (mapper 87): CHR latch at $6000-$7FFF with its two bank
// bits wired swapped.
type Jaleco87 struct {
	base
	bank byte
}

// NewJaleco87 constructs the corresponding board.
func NewJaleco87(c *cartridge.Cartridge) *Jaleco87 {
	return &Jaleco87{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Jaleco87) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.prg[int(addr-0x8000)%len(m.prg)]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Jaleco87) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.bank = (v&1)<<1 | (v&2)>>1
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Jaleco87) ReadCHR(addr uint16) byte { return m.chrRead(int(m.bank), 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Jaleco87) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.bank), 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Jaleco87) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.bank }

// Restore loads the board's mapper-specific state from s.
func (m *Jaleco87) Restore(s *State) { m.restoreRAM(s); m.bank = s.Regs[0] }

// UN1ROM (mapper 94): UxROM with the bank number in bits 2-4.
type UN1ROM struct {
	base
	bank byte
}

// NewUN1ROM constructs the corresponding board.
func NewUN1ROM(c *cartridge.Cartridge) *UN1ROM {
	return &UN1ROM{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *UN1ROM) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.bank), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *UN1ROM) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		v &= m.ReadPRG(addr) // bus conflict
		m.bank = v >> 2 & 7
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *UN1ROM) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *UN1ROM) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *UN1ROM) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.bank }

// Restore loads the board's mapper-specific state from s.
func (m *UN1ROM) Restore(s *State) { m.restoreRAM(s); m.bank = s.Regs[0] }

// Jaleco140 (mapper 140): GxROM-style latch at $6000-$7FFF.
type Jaleco140 struct {
	base
	reg byte
}

// NewJaleco140 constructs the corresponding board.
func NewJaleco140(c *cartridge.Cartridge) *Jaleco140 {
	return &Jaleco140{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Jaleco140) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.reg>>4&3), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Jaleco140) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.reg = v
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Jaleco140) ReadCHR(addr uint16) byte { return m.chrRead(int(m.reg&0x0F), 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Jaleco140) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.reg&0x0F), 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Jaleco140) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *Jaleco140) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// UNROM180 (mapper 180, Crazy Climber): UxROM inverted — the first
// bank is fixed at $8000 and $C000 is switchable.
type UNROM180 struct {
	base
	bank byte
}

// NewUNROM180 constructs the corresponding board.
func NewUNROM180(c *cartridge.Cartridge) *UNROM180 {
	return &UNROM180{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *UNROM180) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, int(m.bank), 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, 0, 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *UNROM180) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		v &= m.ReadPRG(addr) // bus conflict
		m.bank = v & 7
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *UNROM180) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *UNROM180) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *UNROM180) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.bank }

// Restore loads the board's mapper-specific state from s.
func (m *UNROM180) Restore(s *State) { m.restoreRAM(s); m.bank = s.Regs[0] }

// Quattro (mapper 232, Camerica BF9096): a 64 KiB outer block register
// at $8000-$BFFF and a 16 KiB inner bank at $C000-$FFFF; the last bank
// of the block is fixed at $C000.
type Quattro struct {
	base
	block byte
	inner byte
}

// NewQuattro constructs the corresponding board.
func NewQuattro(c *cartridge.Cartridge) *Quattro {
	return &Quattro{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Quattro) ReadPRG(addr uint16) byte {
	block := int(m.block) * 4
	switch {
	case addr >= 0xC000:
		return window(m.prg, block+3, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, block+int(m.inner), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Quattro) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0xC000:
		m.inner = v & 3
	case addr >= 0x8000:
		m.block = v >> 3 & 3
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Quattro) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Quattro) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Quattro) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.block
	s.Regs[1] = m.inner
}

// Restore loads the board's mapper-specific state from s.
func (m *Quattro) Restore(s *State) {
	m.restoreRAM(s)
	m.block = s.Regs[0]
	m.inner = s.Regs[1]
}
