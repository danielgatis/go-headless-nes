package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// FDS-pirate and misc boards with cycle IRQs and PRG-ROM windows at
// $5000/$6000, ported from the reference emulator.

// Mapper39 (39, Study & Game 32-in-1): the whole value is a 32 KiB PRG
// bank; CHR is fixed.
type Mapper39 struct {
	base
	prgBank byte
}

// NewMapper39 wires the board.
func NewMapper39(c *cartridge.Cartridge) *Mapper39 {
	return &Mapper39{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper39) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = v
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper39) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper39) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper39) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper39) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.prgBank }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper39) Restore(s *State) { m.restoreRAM(s); m.prgBank = s.Regs[0] }

// Mapper40 (40, SMB2j pirate): fixed 8 KiB PRG banks with a switchable
// window at $C000, a fixed ROM bank at $6000, and a down-counting cycle
// IRQ armed by a write to $A000.
type Mapper40 struct {
	base
	prgC000    int
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewMapper40 wires the board.
func NewMapper40(c *cartridge.Cartridge) *Mapper40 {
	return &Mapper40{base: makeBase(c), prgC000: 6}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper40) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	switch addr & 0xE000 {
	case 0x8000:
		m.irqEnabled = false
		m.irqLine = false
		m.irqCounter = 0
	case 0xA000:
		m.irqEnabled = true
		m.irqCounter = 4096
	case 0xE000:
		m.prgC000 = int(v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper40) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, 7, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.win(m.prg, m.prgC000, 0x2000)[addr&0x1FFF]
	case addr >= 0xA000:
		return m.win(m.prg, 5, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, 4, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.win(m.prg, 6, 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper40) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper40) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Tick advances the board by one cycle.
func (m *Mapper40) Tick() {
	if !m.irqEnabled || m.irqCounter == 0 {
		return
	}
	m.irqCounter--
	if m.irqCounter == 0 {
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper40) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper40) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgC000)
	s.Regs[1] = byte(m.irqCounter)
	s.Regs[2] = byte(m.irqCounter >> 8)
	s.Regs[3] = boolByte(m.irqEnabled)
	s.Regs[4] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper40) Restore(s *State) {
	m.restoreRAM(s)
	m.prgC000 = int(s.Regs[0])
	m.irqCounter = uint16(s.Regs[1]) | uint16(s.Regs[2])<<8
	m.irqEnabled = s.Regs[3] != 0
	m.irqLine = s.Regs[4] != 0
}

// Mapper42 ("Ai Senshi Nicol" FDS conversion): a switchable 8 KiB PRG-ROM
// window at $6000, the last four 8 KiB banks fixed at $8000, an 8 KiB CHR
// bank, and a free-running cycle IRQ that asserts in the top of its range.
type Mapper42 struct {
	base
	prg6000    byte
	chrBank    byte
	mirror     cartridge.Mirroring
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewMapper42 wires the board.
func NewMapper42(c *cartridge.Cartridge) *Mapper42 {
	return &Mapper42{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper42) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	switch addr & 0xE003 {
	case 0x8000:
		if m.chr != nil {
			m.chrBank = v & 0x0F
		}
	case 0xE000:
		m.prg6000 = v & 0x0F
	case 0xE001:
		m.mirror = hvMirror(v&0x08 == 0)
	case 0xE002:
		m.irqEnabled = v == 0x02
		if !m.irqEnabled {
			m.irqLine = false
			m.irqCounter = 0
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper42) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		slot := (addr - 0x8000) >> 13
		return m.win(m.prg, -4+int(slot), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.win(m.prg, int(m.prg6000), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper42) ReadCHR(addr uint16) byte { return m.chrRead(int(m.chrBank), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper42) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.chrBank), 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper42) Mirroring() cartridge.Mirroring { return m.mirror }

// Tick advances the board by one cycle.
func (m *Mapper42) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter++
	if m.irqCounter >= 0x8000 {
		m.irqCounter -= 0x8000
	}
	m.irqLine = m.irqCounter >= 0x6000
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper42) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper42) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prg6000
	s.Regs[1] = m.chrBank
	s.Regs[2] = byte(m.mirror)
	s.Regs[3] = byte(m.irqCounter)
	s.Regs[4] = byte(m.irqCounter >> 8)
	s.Regs[5] = boolByte(m.irqEnabled)
	s.Regs[6] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper42) Restore(s *State) {
	m.restoreRAM(s)
	m.prg6000 = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.mirror = cartridge.Mirroring(s.Regs[2])
	m.irqCounter = uint16(s.Regs[3]) | uint16(s.Regs[4])<<8
	m.irqEnabled = s.Regs[5] != 0
	m.irqLine = s.Regs[6] != 0
}

// UnlD1038 (mapper 59): the write address selects a PRG (16/32 KiB) and
// CHR bank plus mirroring; address bit 8 makes reads return DIP switches
// (returned here as 0, no DIP support).
type UnlD1038 struct {
	dualPRG16
	returnDip bool
}

// NewUnlD1038 wires the board.
func NewUnlD1038(c *cartridge.Cartridge) *UnlD1038 {
	m := &UnlD1038{dualPRG16: dualPRG16{base: makeBase(c)}}
	m.decode(0x8000)
	return m
}

func (m *UnlD1038) decode(addr uint16) {
	if addr&0x80 != 0 {
		p := int(addr&0x70) >> 4
		m.prg0, m.prg1 = p, p
	} else {
		p := int(addr&0x60) >> 4
		m.prg0, m.prg1 = p, p+1
	}
	m.chrBank = int(addr & 0x07)
	m.mirror = hvMirror(addr&0x08 == 0)
	m.returnDip = addr&0x100 != 0
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *UnlD1038) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *UnlD1038) ReadPRG(addr uint16) byte {
	if m.returnDip && addr >= 0x8000 {
		return 0 // DIP switches: not modelled, return 0
	}
	return m.dualPRG16.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *UnlD1038) Save(s *State) {
	m.saveDual(s)
	s.Regs[7] = boolByte(m.returnDip)
}

// Restore loads the board's mapper-specific state from s.
func (m *UnlD1038) Restore(s *State) {
	m.restoreDual(s)
	m.returnDip = s.Regs[7] != 0
}

// Mapper106 (106, SMB3 pirate): four switchable 8 KiB PRG windows and
// eight 1 KiB CHR windows, plus a 16-bit up-counting cycle IRQ.
type Mapper106 struct {
	base
	prgReg     [4]byte
	chrReg     [8]byte
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewMapper106 wires the board.
func NewMapper106(c *cartridge.Cartridge) *Mapper106 {
	return &Mapper106{base: makeBase(c), prgReg: [4]byte{0xFF, 0xFF, 0xFF, 0xFF}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper106) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0x0F {
	case 0, 2:
		m.chrReg[addr&0x0F] = v & 0xFE
	case 1, 3:
		m.chrReg[addr&0x0F] = v | 0x01
	case 4, 5, 6, 7:
		m.chrReg[addr&0x0F] = v
	case 8, 0x0B:
		m.prgReg[(addr&0x0F)-8] = v&0x0F | 0x10
	case 9, 0x0A:
		m.prgReg[(addr&0x0F)-8] = v & 0x1F
	case 0x0D:
		m.irqEnabled = false
		m.irqCounter = 0
		m.irqLine = false
	case 0x0E:
		m.irqCounter = m.irqCounter&0xFF00 | uint16(v)
	case 0x0F:
		m.irqCounter = m.irqCounter&0x00FF | uint16(v)<<8
		m.irqEnabled = true
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper106) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := (addr - 0x8000) >> 13
		return m.win(m.prg, int(m.prgReg[slot]), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper106) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper106) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Tick advances the board by one cycle.
func (m *Mapper106) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter++
	if m.irqCounter == 0 {
		m.irqLine = true
		m.irqEnabled = false
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper106) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper106) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.prgReg[:])
	copy(s.Regs[4:12], m.chrReg[:])
	s.Regs[12] = byte(m.irqCounter)
	s.Regs[13] = byte(m.irqCounter >> 8)
	s.Regs[14] = boolByte(m.irqEnabled)
	s.Regs[15] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper106) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgReg[:], s.Regs[0:4])
	copy(m.chrReg[:], s.Regs[4:12])
	m.irqCounter = uint16(s.Regs[12]) | uint16(s.Regs[13])<<8
	m.irqEnabled = s.Regs[14] != 0
	m.irqLine = s.Regs[15] != 0
}
