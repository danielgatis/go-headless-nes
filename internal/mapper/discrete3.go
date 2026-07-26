package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// More discrete/multicart boards ported from the reference emulator.

// ColorDreams46 (mapper 46, Rumble Station): a $6000 outer register and a
// $8000 inner register combine into a 32 KiB PRG bank and an 8 KiB CHR
// bank.
type ColorDreams46 struct {
	base
	reg [2]byte
}

// NewColorDreams46 wires the board.
func NewColorDreams46(c *cartridge.Cartridge) *ColorDreams46 {
	return &ColorDreams46{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *ColorDreams46) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.reg[0] = v
	} else if addr >= 0x8000 {
		m.reg[1] = v
	}
}

func (m *ColorDreams46) prgBank() int {
	return int(m.reg[0]&0x0F)<<1 | int(m.reg[1]&0x01)
}

func (m *ColorDreams46) chrBank() int {
	return int(m.reg[0]&0xF0)>>1 | int(m.reg[1]&0x70)>>4
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *ColorDreams46) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank(), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		// The $6000 window is the outer register only, not RAM.
		return m.openBus()
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *ColorDreams46) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank(), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *ColorDreams46) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank(), 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *ColorDreams46) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.reg[0]
	s.Regs[1] = m.reg[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *ColorDreams46) Restore(s *State) {
	m.restoreRAM(s)
	m.reg[0] = s.Regs[0]
	m.reg[1] = s.Regs[1]
}

// Mapper50 (mapper 50, SMB2j pirate): three fixed 8 KiB PRG banks plus a
// switchable window at $C000, a fixed ROM bank at $6000, and a CPU-cycle
// IRQ that fires at count 0x1000.
type Mapper50 struct {
	base

	prgC000 int

	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewMapper50 wires the board.
func NewMapper50(c *cartridge.Cartridge) *Mapper50 {
	return &Mapper50{base: makeBase(c), prgC000: 0}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper50) WritePRG(addr uint16, v byte) {
	if addr < 0x6000 {
		switch addr & 0x4120 {
		case 0x4020:
			// Scrambled bank number for the $C000 window.
			m.prgC000 = int(v&0x08) | int(v&0x01)<<2 | int(v&0x06)>>1
		case 0x4120:
			if v&0x01 != 0 {
				m.irqEnabled = true
			} else {
				m.irqEnabled = false
				m.irqLine = false
				m.irqCounter = 0
			}
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper50) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, 0x0B, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.win(m.prg, m.prgC000, 0x2000)[addr&0x1FFF]
	case addr >= 0xA000:
		return m.win(m.prg, 0x09, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, 0x08, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.win(m.prg, 0x0F, 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper50) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper50) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Tick advances the board by one cycle.
func (m *Mapper50) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter++
	if m.irqCounter == 0x1000 {
		m.irqLine = true
		m.irqEnabled = false
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper50) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper50) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgC000)
	s.Regs[1] = byte(m.irqCounter)
	s.Regs[2] = byte(m.irqCounter >> 8)
	s.Regs[3] = boolByte(m.irqEnabled)
	s.Regs[4] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper50) Restore(s *State) {
	m.restoreRAM(s)
	m.prgC000 = int(s.Regs[0])
	m.irqCounter = uint16(s.Regs[1]) | uint16(s.Regs[2])<<8
	m.irqEnabled = s.Regs[3] != 0
	m.irqLine = s.Regs[4] != 0
}

// Mapper170 (mapper 170, Fujiya): a one-bit protection latch. A write to
// $6502 stores bit 6 of the value into bit 7; a read of $7777 returns it
// ORed with the low address byte. PRG/CHR are fixed.
type Mapper170 struct {
	base
	reg byte
}

// NewMapper170 wires the board.
func NewMapper170(c *cartridge.Cartridge) *Mapper170 {
	return &Mapper170{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper170) WritePRG(addr uint16, v byte) {
	if addr == 0x6502 {
		m.reg = (v << 1) & 0x80
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper170) ReadPRG(addr uint16) byte {
	if addr == 0x7777 {
		return m.reg | byte((addr>>8)&0x7F)
	}
	if addr >= 0x8000 {
		return m.win(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper170) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper170) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper170) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper170) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// Mapper174 (mapper 174, NTDec 5-in-1): address bits select PRG (16/32
// KiB), CHR and mirroring.
type Mapper174 struct{ dualPRG16 }

// NewMapper174 wires the board.
func NewMapper174(c *cartridge.Cartridge) *Mapper174 {
	return &Mapper174{dualPRG16{base: makeBase(c)}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper174) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	prg := int(addr>>4) & 0x07
	if addr&0x80 != 0 {
		m.prg0, m.prg1 = prg&0xFE, prg&0xFE|1
	} else {
		m.prg0, m.prg1 = prg, prg
	}
	m.chrBank = int(addr>>1) & 0x07
	m.mirror = hvMirror(addr&0x01 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper174) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper174) Restore(s *State) { m.restoreDual(s) }

// Mapper204 (mapper 204): address bits pick 16/32 KiB PRG windows and an
// 8 KiB CHR bank, with a self-locking top bank.
type Mapper204 struct{ dualPRG16 }

// NewMapper204 wires the board.
func NewMapper204(c *cartridge.Cartridge) *Mapper204 {
	return &Mapper204{dualPRG16{base: makeBase(c)}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper204) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	mask := int(addr) & 0x06
	var page, page1 int
	if mask == 0x06 {
		page, page1 = mask, mask+1
	} else {
		page = mask + int(addr&0x01)
		page1 = page
	}
	m.prg0, m.prg1 = page, page1
	m.chrBank = page
	m.mirror = hvMirror(addr&0x10 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper204) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper204) Restore(s *State) { m.restoreDual(s) }

// Mapper216 (mapper 216): address bit 0 selects the 32 KiB PRG bank and
// bits 1-3 the 8 KiB CHR bank.
type Mapper216 struct {
	base
	prgBank int
	chrBank int
}

// NewMapper216 wires the board.
func NewMapper216(c *cartridge.Cartridge) *Mapper216 {
	return &Mapper216{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper216) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = int(addr & 0x01)
		m.chrBank = int(addr&0x0E) >> 1
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper216) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper216) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper216) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper216) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper216) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
}
