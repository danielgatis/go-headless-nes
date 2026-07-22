package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Boards whose IRQ counters clock on the filtered A12 rising edge. The
// PPU delivers that edge through Scanline() (the same hook the MMC3 uses),
// so these boards implement Scanline() to advance their own counter.

// Mapper35 (mapper 35, SMB2j-style pirate): four 8 KiB PRG windows, eight
// 1 KiB CHR windows, and an MMC3-style A12 down-counter IRQ.
type Mapper35 struct {
	base
	prgReg     [4]byte
	chrReg     [8]byte
	mirror     cartridge.Mirroring
	irqCounter byte
	irqEnabled bool
	irqLine    bool
}

// NewMapper35 wires the board.
func NewMapper35(c *cartridge.Cartridge) *Mapper35 {
	m := &Mapper35{base: makeBase(c), mirror: c.Mirroring}
	m.prgReg[3] = 0xFF // last bank fixed at $E000
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper35) WritePRG(addr uint16, v byte) {
	switch addr & 0xF007 {
	case 0x8000, 0x8001, 0x8002, 0x8003:
		m.prgReg[addr&0x03] = v
	case 0x9000, 0x9001, 0x9002, 0x9003, 0x9004, 0x9005, 0x9006, 0x9007:
		m.chrReg[addr&0x07] = v
	case 0xC002:
		m.irqEnabled = false
		m.irqLine = false
	case 0xC003:
		m.irqEnabled = true
	case 0xC005:
		m.irqCounter = v
	case 0xD001:
		m.mirror = hvMirror(v&0x01 == 0)
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper35) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := (addr - 0x8000) >> 13
		if slot == 3 {
			return window(m.prg, -1, 0x2000)[addr&0x1FFF]
		}
		return window(m.prg, int(m.prgReg[slot]), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper35) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper35) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper35) Mirroring() cartridge.Mirroring { return m.mirror }

// Scanline clocks the A12 down-counter; a zero decrement asserts the line.
func (m *Mapper35) Scanline() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter--
	if m.irqCounter == 0 {
		m.irqEnabled = false
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper35) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper35) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.prgReg[:])
	copy(s.Regs[4:12], m.chrReg[:])
	s.Regs[12] = byte(m.mirror)
	s.Regs[13] = m.irqCounter
	s.Regs[14] = boolByte(m.irqEnabled)
	s.Regs[15] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper35) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgReg[:], s.Regs[0:4])
	copy(m.chrReg[:], s.Regs[4:12])
	m.mirror = cartridge.Mirroring(s.Regs[12])
	m.irqCounter = s.Regs[13]
	m.irqEnabled = s.Regs[14] != 0
	m.irqLine = s.Regs[15] != 0
}

// Mapper117 (mapper 117): three switchable 8 KiB PRG windows (last fixed),
// eight 1 KiB CHR windows, and an A12 down-counter IRQ gated by two enable
// flags.
type Mapper117 struct {
	base
	prgReg      [4]byte
	chrReg      [8]byte
	mirror      cartridge.Mirroring
	irqCounter  byte
	irqReload   byte
	irqEnabled  bool
	irqEnabled2 bool
	irqLine     bool
}

// NewMapper117 wires the board.
func NewMapper117(c *cartridge.Cartridge) *Mapper117 {
	m := &Mapper117{base: makeBase(c), mirror: c.Mirroring}
	m.prgReg[3] = 0xFF
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper117) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000 && addr <= 0x8003:
		m.prgReg[addr&0x03] = v
	case addr >= 0xA000 && addr <= 0xA007:
		m.chrReg[addr&0x07] = v
	case addr == 0xC001:
		m.irqReload = v
	case addr == 0xC002:
		m.irqLine = false
	case addr == 0xC003:
		m.irqCounter = m.irqReload
		m.irqEnabled2 = true
	case addr == 0xD000:
		m.mirror = hvMirror(v&0x01 == 0)
	case addr == 0xE000:
		m.irqEnabled = v&0x01 != 0
		m.irqLine = false
	case addr >= 0x6000 && addr < 0x8000:
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper117) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := (addr - 0x8000) >> 13
		if slot == 3 {
			return window(m.prg, -1, 0x2000)[addr&0x1FFF]
		}
		return window(m.prg, int(m.prgReg[slot]), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper117) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper117) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper117) Mirroring() cartridge.Mirroring { return m.mirror }

// Scanline clocks the board's per-scanline logic.
func (m *Mapper117) Scanline() {
	if m.irqEnabled && m.irqEnabled2 && m.irqCounter != 0 {
		m.irqCounter--
		if m.irqCounter == 0 {
			m.irqEnabled2 = false
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper117) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper117) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.prgReg[:])
	copy(s.Regs[4:12], m.chrReg[:])
	s.Regs[12] = byte(m.mirror)
	s.Regs[13] = m.irqCounter
	s.Regs[14] = m.irqReload
	s.Regs[15] = boolByte(m.irqEnabled)
	s.Regs[16] = boolByte(m.irqEnabled2)
	s.Regs[17] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper117) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgReg[:], s.Regs[0:4])
	copy(m.chrReg[:], s.Regs[4:12])
	m.mirror = cartridge.Mirroring(s.Regs[12])
	m.irqCounter = s.Regs[13]
	m.irqReload = s.Regs[14]
	m.irqEnabled = s.Regs[15] != 0
	m.irqEnabled2 = s.Regs[16] != 0
	m.irqLine = s.Regs[17] != 0
}

// Mapper222 (mapper 222): two switchable 8 KiB PRG windows (last two
// fixed), eight 1 KiB CHR windows, and an A12 up-counter IRQ that fires at
// 240.
type Mapper222 struct {
	base
	prgReg     [2]byte
	chrReg     [8]byte
	mirror     cartridge.Mirroring
	irqCounter byte
	irqLine    bool
}

// NewMapper222 wires the board.
func NewMapper222(c *cartridge.Cartridge) *Mapper222 {
	return &Mapper222{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper222) WritePRG(addr uint16, v byte) {
	switch addr & 0xF003 {
	case 0x8000:
		m.prgReg[0] = v
	case 0x9000:
		m.mirror = hvMirror(v&0x01 == 0)
	case 0xA000:
		m.prgReg[1] = v
	case 0xB000:
		m.chrReg[0] = v
	case 0xB002:
		m.chrReg[1] = v
	case 0xC000:
		m.chrReg[2] = v
	case 0xC002:
		m.chrReg[3] = v
	case 0xD000:
		m.chrReg[4] = v
	case 0xD002:
		m.chrReg[5] = v
	case 0xE000:
		m.chrReg[6] = v
	case 0xE002:
		m.chrReg[7] = v
	case 0xF000:
		m.irqCounter = v
		m.irqLine = false
	default:
		if addr >= 0x6000 && addr < 0x8000 {
			m.writePRGRAM(addr, v)
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper222) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := (addr - 0x8000) >> 13 // 0..3
		switch slot {
		case 0:
			return window(m.prg, int(m.prgReg[0]), 0x2000)[addr&0x1FFF]
		case 1:
			return window(m.prg, int(m.prgReg[1]), 0x2000)[addr&0x1FFF]
		case 2:
			return window(m.prg, -2, 0x2000)[addr&0x1FFF]
		default:
			return window(m.prg, -1, 0x2000)[addr&0x1FFF]
		}
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper222) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper222) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper222) Mirroring() cartridge.Mirroring { return m.mirror }

// Scanline advances the up-counter on each A12 rise; it fires at 240.
func (m *Mapper222) Scanline() {
	if m.irqCounter != 0 {
		m.irqCounter++
		if m.irqCounter >= 240 {
			m.irqLine = true
			m.irqCounter = 0
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper222) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper222) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg[0]
	s.Regs[1] = m.prgReg[1]
	copy(s.Regs[2:10], m.chrReg[:])
	s.Regs[10] = byte(m.mirror)
	s.Regs[11] = m.irqCounter
	s.Regs[12] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper222) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg[0] = s.Regs[0]
	m.prgReg[1] = s.Regs[1]
	copy(m.chrReg[:], s.Regs[2:10])
	m.mirror = cartridge.Mirroring(s.Regs[10])
	m.irqCounter = s.Regs[11]
	m.irqLine = s.Regs[12] != 0
}
