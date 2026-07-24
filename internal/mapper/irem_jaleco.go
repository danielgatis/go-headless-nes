package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// IremG101 (mapper 32): two switchable 8 KiB PRG banks with a mode bit
// that swaps the fixed/switchable windows, eight 1 KiB CHR banks and
// register mirroring. Submapper 1 (Major League) hard-wires one-screen
// mirroring and the plain PRG mode.
type IremG101 struct {
	base

	prgRegs  [2]byte
	prgMode  byte
	chrBanks [8]byte
	fixedNT  bool // submapper 1
}

// NewIremG101 wires an Irem G-101 board.
func NewIremG101(c *cartridge.Cartridge) *IremG101 {
	m := &IremG101{base: makeBase(c), fixedNT: c.Submapper == 1}
	if m.fixedNT {
		m.mirroring = cartridge.SingleLow
	}
	return m
}

func (m *IremG101) prgBank(addr uint16) int {
	slot := (addr - 0x8000) >> 13
	if m.prgMode == 0 {
		switch slot {
		case 0:
			return int(m.prgRegs[0])
		case 1:
			return int(m.prgRegs[1])
		case 2:
			return -2
		default:
			return -1
		}
	}
	switch slot {
	case 0:
		return -2
	case 1:
		return int(m.prgRegs[1])
	case 2:
		return int(m.prgRegs[0])
	default:
		return -1
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *IremG101) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return window(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *IremG101) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		switch addr & 0xF000 {
		case 0x8000:
			m.prgRegs[0] = v & 0x1F
		case 0x9000:
			m.prgMode = (v & 0x02) >> 1
			switch {
			case m.fixedNT:
				m.prgMode = 0
			case v&0x01 != 0:
				m.mirroring = cartridge.Horizontal
			default:
				m.mirroring = cartridge.Vertical
			}
		case 0xA000:
			m.prgRegs[1] = v & 0x1F
		case 0xB000:
			m.chrBanks[addr&0x07] = v
		}
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *IremG101) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>10]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *IremG101) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>10]), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *IremG101) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:2], m.prgRegs[:])
	s.Regs[2] = m.prgMode
	copy(s.Regs[3:11], m.chrBanks[:])
	s.Regs[11] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *IremG101) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgRegs[:], s.Regs[0:2])
	m.prgMode = s.Regs[2]
	copy(m.chrBanks[:], s.Regs[3:11])
	m.mirroring = cartridge.Mirroring(s.Regs[11])
}

// IremH3001 (mapper 65): three switchable 8 KiB PRG banks, eight 1 KiB
// CHR banks and a 16-bit CPU-cycle IRQ down-counter that fires once and
// disables itself.
type IremH3001 struct {
	base

	prgBanks [3]byte
	chrBanks [8]byte

	irqEnabled bool
	irqCounter uint16
	irqReload  uint16
	irqLine    bool
}

// NewIremH3001 wires an Irem H3001 board.
func NewIremH3001(c *cartridge.Cartridge) *IremH3001 {
	m := &IremH3001{base: makeBase(c)}
	m.prgBanks[0] = 0
	m.prgBanks[1] = 1
	m.prgBanks[2] = 0xFE
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *IremH3001) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBanks[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *IremH3001) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr {
	case 0x8000:
		m.prgBanks[0] = v
	case 0x9001:
		if v&0x80 != 0 {
			m.mirroring = cartridge.Horizontal
		} else {
			m.mirroring = cartridge.Vertical
		}
	case 0x9003:
		m.irqEnabled = v&0x80 != 0
		m.irqLine = false
	case 0x9004:
		m.irqCounter = m.irqReload
		m.irqLine = false
	case 0x9005:
		m.irqReload = (m.irqReload & 0x00FF) | uint16(v)<<8
	case 0x9006:
		m.irqReload = (m.irqReload & 0xFF00) | uint16(v)
	case 0xA000:
		m.prgBanks[1] = v
	case 0xB000, 0xB001, 0xB002, 0xB003, 0xB004, 0xB005, 0xB006, 0xB007:
		m.chrBanks[addr&0x07] = v
	case 0xC000:
		m.prgBanks[2] = v
	}
}

// Tick advances the board by one cycle.
func (m *IremH3001) Tick() {
	if m.irqEnabled {
		m.irqCounter--
		if m.irqCounter == 0 {
			m.irqEnabled = false
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *IremH3001) IRQ() bool { return m.irqLine }

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *IremH3001) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>10]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *IremH3001) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>10]), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *IremH3001) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:3], m.prgBanks[:])
	copy(s.Regs[3:11], m.chrBanks[:])
	s.Regs[11] = boolByte(m.irqEnabled) | boolByte(m.irqLine)<<1
	s.Regs[12] = byte(m.irqCounter)
	s.Regs[13] = byte(m.irqCounter >> 8)
	s.Regs[14] = byte(m.irqReload)
	s.Regs[15] = byte(m.irqReload >> 8)
	s.Regs[16] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *IremH3001) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:3])
	copy(m.chrBanks[:], s.Regs[3:11])
	m.irqEnabled = s.Regs[11]&1 != 0
	m.irqLine = s.Regs[11]&2 != 0
	m.irqCounter = uint16(s.Regs[12]) | uint16(s.Regs[13])<<8
	m.irqReload = uint16(s.Regs[14]) | uint16(s.Regs[15])<<8
	m.mirroring = cartridge.Mirroring(s.Regs[16])
}

// IremTamS1 (mapper 97, Kaiketsu Yanchamaru): the first 16 KiB window is
// fixed to the LAST bank and the second is switchable, the reverse of
// UNROM, with two-bit register mirroring.
type IremTamS1 struct {
	base

	prgBank byte
}

// NewIremTamS1 wires an Irem TAM-S1 board.
func NewIremTamS1(c *cartridge.Cartridge) *IremTamS1 {
	return &IremTamS1{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *IremTamS1) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *IremTamS1) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.prgBank = v & 0x0F
		switch v >> 6 {
		case 0:
			m.mirroring = cartridge.SingleLow
		case 1:
			m.mirroring = cartridge.Horizontal
		case 2:
			m.mirroring = cartridge.Vertical
		case 3:
			m.mirroring = cartridge.SingleHigh
		}
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *IremTamS1) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *IremTamS1) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *IremTamS1) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *IremTamS1) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.mirroring = cartridge.Mirroring(s.Regs[1])
}

// JalecoJF16 (mapper 78): one switchable 16 KiB PRG bank, one 8 KiB CHR
// bank and one-screen mirroring, except submapper 3 (Holy Diver),
// whose bit selects H/V instead. Has bus conflicts.
type JalecoJF16 struct {
	base

	prgBank   byte
	chrBank   byte
	holyDiver bool
}

// NewJalecoJF16 wires a Jaleco JF-16 board.
func NewJalecoJF16(c *cartridge.Cartridge) *JalecoJF16 {
	return &JalecoJF16{base: makeBase(c), holyDiver: c.Submapper == 3}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *JalecoJF16) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *JalecoJF16) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		v &= m.ReadPRG(addr) // bus conflict
		m.prgBank = v & 0x07
		m.chrBank = (v >> 4) & 0x0F
		switch {
		case m.holyDiver && v&0x08 != 0:
			m.mirroring = cartridge.Vertical
		case m.holyDiver:
			m.mirroring = cartridge.Horizontal
		case v&0x08 != 0:
			m.mirroring = cartridge.SingleHigh
		default:
			m.mirroring = cartridge.SingleLow
		}
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *JalecoJF16) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *JalecoJF16) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *JalecoJF16) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
	s.Regs[2] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *JalecoJF16) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.mirroring = cartridge.Mirroring(s.Regs[2])
}

// JalecoJF17 covers mappers 72 (JF-17) and 92 (JF-19): the bank latches
// only load on the rising edge of the write's top bits. Has bus
// conflicts. The sample-playback bits are not emulated.
type JalecoJF17 struct {
	base

	jf19    bool
	prgFlag bool
	chrFlag bool
	prgBank byte
	chrBank byte
}

// NewJalecoJF17 wires a Jaleco JF-17 (mapper 72) or JF-19 (mapper 92).
func NewJalecoJF17(c *cartridge.Cartridge) *JalecoJF17 {
	return &JalecoJF17{base: makeBase(c), jf19: c.MapperID == 92}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *JalecoJF17) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		if m.jf19 {
			return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
		}
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		if m.jf19 {
			return window(m.prg, 0, 0x4000)[addr&0x3FFF]
		}
		return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *JalecoJF17) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	v &= m.ReadPRG(addr) // bus conflict

	if !m.prgFlag && v&0x80 != 0 {
		if m.jf19 {
			m.prgBank = v & 0x0F
		} else {
			m.prgBank = v & 0x07
		}
	}
	if !m.chrFlag && v&0x40 != 0 {
		m.chrBank = v & 0x0F
	}
	m.prgFlag = v&0x80 != 0
	m.chrFlag = v&0x40 != 0
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *JalecoJF17) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *JalecoJF17) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *JalecoJF17) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
	s.Regs[2] = boolByte(m.prgFlag) | boolByte(m.chrFlag)<<1
}

// Restore loads the board's mapper-specific state from s.
func (m *JalecoJF17) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.prgFlag = s.Regs[2]&1 != 0
	m.chrFlag = s.Regs[2]&2 != 0
}

// jalecoIRQMasks selects how much of the SS88006 IRQ counter is active.
var jalecoIRQMasks = [4]uint16{0xFFFF, 0x0FFF, 0x00FF, 0x000F}

// JalecoSS88006 (mapper 18): three 8 KiB PRG banks and eight 1 KiB CHR
// banks loaded a nibble at a time, register mirroring, and a CPU-cycle
// IRQ down-counter maskable to 16/12/8/4 bits.
type JalecoSS88006 struct {
	base

	prgBanks  [3]byte
	chrBanks  [8]byte
	irqReload [4]byte

	irqCounter     uint16
	irqCounterSize byte
	irqEnabled     bool
	irqLine        bool
}

// NewJalecoSS88006 wires a Jaleco SS88006 board.
func NewJalecoSS88006(c *cartridge.Cartridge) *JalecoSS88006 {
	return &JalecoSS88006{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *JalecoSS88006) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBanks[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *JalecoSS88006) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}

	high := addr&0x01 != 0
	v &= 0x0F
	setNibble := func(reg *byte) {
		if high {
			*reg = (*reg & 0x0F) | v<<4
		} else {
			*reg = (*reg & 0xF0) | v
		}
	}

	switch addr & 0xF003 {
	case 0x8000, 0x8001:
		setNibble(&m.prgBanks[0])
	case 0x8002, 0x8003:
		setNibble(&m.prgBanks[1])
	case 0x9000, 0x9001:
		setNibble(&m.prgBanks[2])
	case 0xA000, 0xA001:
		setNibble(&m.chrBanks[0])
	case 0xA002, 0xA003:
		setNibble(&m.chrBanks[1])
	case 0xB000, 0xB001:
		setNibble(&m.chrBanks[2])
	case 0xB002, 0xB003:
		setNibble(&m.chrBanks[3])
	case 0xC000, 0xC001:
		setNibble(&m.chrBanks[4])
	case 0xC002, 0xC003:
		setNibble(&m.chrBanks[5])
	case 0xD000, 0xD001:
		setNibble(&m.chrBanks[6])
	case 0xD002, 0xD003:
		setNibble(&m.chrBanks[7])
	case 0xE000, 0xE001, 0xE002, 0xE003:
		m.irqReload[addr&0x03] = v
	case 0xF000:
		m.irqLine = false
		m.irqCounter = uint16(m.irqReload[0]) | uint16(m.irqReload[1])<<4 |
			uint16(m.irqReload[2])<<8 | uint16(m.irqReload[3])<<12
	case 0xF001:
		m.irqLine = false
		m.irqEnabled = v&0x01 != 0
		switch {
		case v&0x08 != 0:
			m.irqCounterSize = 3 // 4-bit counter
		case v&0x04 != 0:
			m.irqCounterSize = 2 // 8-bit counter
		case v&0x02 != 0:
			m.irqCounterSize = 1 // 12-bit counter
		default:
			m.irqCounterSize = 0 // 16-bit counter
		}
	case 0xF002:
		switch v & 0x03 {
		case 0:
			m.mirroring = cartridge.Horizontal
		case 1:
			m.mirroring = cartridge.Vertical
		case 2:
			m.mirroring = cartridge.SingleLow
		case 3:
			m.mirroring = cartridge.SingleHigh
		}
	}
}

// Tick advances the board by one cycle.
func (m *JalecoSS88006) Tick() {
	if !m.irqEnabled {
		return
	}
	mask := jalecoIRQMasks[m.irqCounterSize]
	counter := m.irqCounter & mask
	counter--
	if counter == 0 {
		m.irqLine = true
	}
	m.irqCounter = (m.irqCounter &^ mask) | (counter & mask)
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *JalecoSS88006) IRQ() bool { return m.irqLine }

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *JalecoSS88006) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>10]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *JalecoSS88006) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>10]), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *JalecoSS88006) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:3], m.prgBanks[:])
	copy(s.Regs[3:11], m.chrBanks[:])
	copy(s.Regs[11:15], m.irqReload[:])
	s.Regs[15] = byte(m.irqCounter)
	s.Regs[16] = byte(m.irqCounter >> 8)
	s.Regs[17] = m.irqCounterSize
	s.Regs[18] = boolByte(m.irqEnabled) | boolByte(m.irqLine)<<1
	s.Regs[19] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *JalecoSS88006) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:3])
	copy(m.chrBanks[:], s.Regs[3:11])
	copy(m.irqReload[:], s.Regs[11:15])
	m.irqCounter = uint16(s.Regs[15]) | uint16(s.Regs[16])<<8
	m.irqCounterSize = s.Regs[17]
	m.irqEnabled = s.Regs[18]&1 != 0
	m.irqLine = s.Regs[18]&2 != 0
	m.mirroring = cartridge.Mirroring(s.Regs[19])
}
