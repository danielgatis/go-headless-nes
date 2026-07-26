package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Sunsoft184 (mapper 184, Sunsoft-1): two 4 KiB CHR banks selected from
// one register at $6000-$7FFF; the upper window's high bit is wired set.
type Sunsoft184 struct {
	base

	chrBanks [2]byte
}

// NewSunsoft184 wires a Sunsoft-1 board.
func NewSunsoft184(c *cartridge.Cartridge) *Sunsoft184 {
	m := &Sunsoft184{base: makeBase(c)}
	m.chrBanks[1] = 0x04
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sunsoft184) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sunsoft184) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.chrBanks[0] = v & 0x07
		m.chrBanks[1] = 0x04 | ((v >> 4) & 0x03)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sunsoft184) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>12]), 0x1000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sunsoft184) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>12]), 0x1000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Sunsoft184) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:2], m.chrBanks[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Sunsoft184) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.chrBanks[:], s.Regs[0:2])
}

// Sunsoft89 (mapper 89, Sunsoft-2 on Sunsoft-3 PCB): one 16 KiB PRG
// bank, one 8 KiB CHR bank with a 4th bit in the write's top bit, and
// one-screen mirroring.
type Sunsoft89 struct {
	base

	prgBank byte
	chrBank byte
}

// NewSunsoft89 wires the board.
func NewSunsoft89(c *cartridge.Cartridge) *Sunsoft89 {
	return &Sunsoft89{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sunsoft89) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sunsoft89) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = (v >> 4) & 0x07
		m.chrBank = (v & 0x07) | ((v & 0x80) >> 4)
		if v&0x08 != 0 {
			m.mirroring = cartridge.SingleHigh
		} else {
			m.mirroring = cartridge.SingleLow
		}
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sunsoft89) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sunsoft89) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Sunsoft89) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
	s.Regs[2] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sunsoft89) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.mirroring = cartridge.Mirroring(s.Regs[2])
}

// Sunsoft93 (mapper 93, Sunsoft-2 on Sunsoft-3R PCB): one 16 KiB PRG
// bank and a CHR enable bit, CHR reads float when disabled.
type Sunsoft93 struct {
	base

	prgBank    byte
	chrEnabled bool
}

// NewSunsoft93 wires the board.
func NewSunsoft93(c *cartridge.Cartridge) *Sunsoft93 {
	return &Sunsoft93{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sunsoft93) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sunsoft93) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = (v >> 4) & 0x07
		m.chrEnabled = v&0x01 != 0
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sunsoft93) ReadCHR(addr uint16) byte {
	if !m.chrEnabled {
		return 0
	}
	return m.chrRead(0, 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sunsoft93) WriteCHR(addr uint16, v byte) {
	if m.chrEnabled {
		m.chrWrite(0, 0x2000, addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Sunsoft93) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = boolByte(m.chrEnabled)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sunsoft93) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chrEnabled = s.Regs[1] != 0
}

// Sunsoft3 (mapper 67): four 2 KiB CHR banks, one 16 KiB PRG bank, a
// 16-bit IRQ down-counter loaded high/low through a write toggle that
// fires on underflow and disables itself.
type Sunsoft3 struct {
	base

	prgBank  byte
	chrBanks [4]byte

	irqLatch   bool
	irqEnabled bool
	irqCounter uint16
	irqLine    bool
}

// NewSunsoft3 wires a Sunsoft-3 board.
func NewSunsoft3(c *cartridge.Cartridge) *Sunsoft3 {
	return &Sunsoft3{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sunsoft3) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sunsoft3) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0xF800 {
	case 0x8800:
		m.chrBanks[0] = v
	case 0x9800:
		m.chrBanks[1] = v
	case 0xA800:
		m.chrBanks[2] = v
	case 0xB800:
		m.chrBanks[3] = v
	case 0xC800:
		if m.irqLatch {
			m.irqCounter = (m.irqCounter & 0xFF00) | uint16(v)
		} else {
			m.irqCounter = (m.irqCounter & 0x00FF) | uint16(v)<<8
		}
		m.irqLatch = !m.irqLatch
	case 0xD800:
		m.irqEnabled = v&0x10 != 0
		m.irqLatch = false
		m.irqLine = false
	case 0xE800:
		switch v & 0x03 {
		case 0:
			m.mirroring = cartridge.Vertical
		case 1:
			m.mirroring = cartridge.Horizontal
		case 2:
			m.mirroring = cartridge.SingleLow
		case 3:
			m.mirroring = cartridge.SingleHigh
		}
	case 0xF800:
		m.prgBank = v
	}
}

// Tick advances the board by one cycle.
func (m *Sunsoft3) Tick() {
	if m.irqEnabled {
		m.irqCounter--
		if m.irqCounter == 0xFFFF {
			m.irqEnabled = false
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Sunsoft3) IRQ() bool { return m.irqLine }

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sunsoft3) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>11]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sunsoft3) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>11]), 0x800, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Sunsoft3) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	copy(s.Regs[1:5], m.chrBanks[:])
	s.Regs[5] = boolByte(m.irqLatch) | boolByte(m.irqEnabled)<<1 | boolByte(m.irqLine)<<2
	s.Regs[6] = byte(m.irqCounter)
	s.Regs[7] = byte(m.irqCounter >> 8)
	s.Regs[8] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sunsoft3) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	copy(m.chrBanks[:], s.Regs[1:5])
	m.irqLatch = s.Regs[5]&1 != 0
	m.irqEnabled = s.Regs[5]&2 != 0
	m.irqLine = s.Regs[5]&4 != 0
	m.irqCounter = uint16(s.Regs[6]) | uint16(s.Regs[7])<<8
	m.mirroring = cartridge.Mirroring(s.Regs[8])
}

// SunsoftFME7 (mapper 69, FME-7/Sunsoft 5A/5B): command/parameter
// interface with eight 1 KiB CHR banks, three 8 KiB PRG banks, a
// $6000 window that maps either RAM or a PRG ROM bank, register
// mirroring and a free-running 16-bit IRQ down-counter. The 5B's three
// tone generators are mixed into the APU output.
type SunsoftFME7 struct {
	base

	command  byte
	workReg  byte
	prgBanks [3]byte
	chrBanks [8]byte

	irqEnabled        bool
	irqCounterEnabled bool
	irqCounter        uint16
	irqLine           bool

	audio      sunsoft5b
	audioLevel int
}

// NewSunsoftFME7 wires an FME-7 board.
func NewSunsoftFME7(c *cartridge.Cartridge) *SunsoftFME7 {
	return &SunsoftFME7{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *SunsoftFME7) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prgBanks[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.workReg&0x40 != 0 {
			if m.workReg&0x80 == 0 {
				return m.openBus() // RAM selected but disabled
			}
			return m.readPRGRAM(addr)
		}
		return m.win(m.prg, int(m.workReg&0x3F), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *SunsoftFME7) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		switch addr & 0xE000 {
		case 0x8000:
			m.command = v & 0x0F
		case 0xA000:
			m.writeParameter(v)
		case 0xC000, 0xE000:
			m.audio.writeRegister(addr&0xE000, v)
		}
	case addr >= 0x6000:
		if m.workReg&0x40 != 0 && m.workReg&0x80 != 0 {
			m.writePRGRAM(addr, v)
		}
	}
}

func (m *SunsoftFME7) writeParameter(v byte) {
	switch m.command {
	case 0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7:
		m.chrBanks[m.command] = v
	case 0x8:
		m.workReg = v
	case 0x9, 0xA, 0xB:
		m.prgBanks[m.command-9] = v & 0x3F
	case 0xC:
		switch v & 0x03 {
		case 0:
			m.mirroring = cartridge.Vertical
		case 1:
			m.mirroring = cartridge.Horizontal
		case 2:
			m.mirroring = cartridge.SingleLow
		case 3:
			m.mirroring = cartridge.SingleHigh
		}
	case 0xD:
		m.irqEnabled = v&0x01 != 0
		m.irqCounterEnabled = v&0x80 != 0
		m.irqLine = false
	case 0xE:
		m.irqCounter = (m.irqCounter & 0xFF00) | uint16(v)
	case 0xF:
		m.irqCounter = (m.irqCounter & 0x00FF) | uint16(v)<<8
	}
}

// Tick advances the board by one cycle.
func (m *SunsoftFME7) Tick() {
	m.audioLevel = m.audio.clock()
	if m.irqCounterEnabled {
		m.irqCounter--
		if m.irqCounter == 0xFFFF && m.irqEnabled {
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *SunsoftFME7) IRQ() bool { return m.irqLine }

// AudioLevel mixes the 5B tone generators.
func (m *SunsoftFME7) AudioLevel() float32 {
	return exp5BStep * float32(m.audioLevel)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *SunsoftFME7) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>10]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *SunsoftFME7) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>10]), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *SunsoftFME7) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.command
	s.Regs[1] = m.workReg
	copy(s.Regs[2:5], m.prgBanks[:])
	copy(s.Regs[5:13], m.chrBanks[:])
	s.Regs[13] = boolByte(m.irqEnabled) | boolByte(m.irqCounterEnabled)<<1 | boolByte(m.irqLine)<<2
	s.Regs[14] = byte(m.irqCounter)
	s.Regs[15] = byte(m.irqCounter >> 8)
	s.Regs[16] = byte(m.mirroring)
	copy(s.Regs[17:33], m.audio.Regs[:])
	s.Regs[33] = m.audio.Current
	for i := 0; i < 3; i++ {
		s.Regs[34+i*2] = byte(m.audio.Timer[i])
		s.Regs[35+i*2] = byte(m.audio.Timer[i] >> 8)
	}
	s.Regs[40] = m.audio.ToneStep[0] | m.audio.ToneStep[1]<<4
	s.Regs[41] = m.audio.ToneStep[2] | boolByte(m.audio.ProcessTick)<<4
}

// Restore loads the board's mapper-specific state from s.
func (m *SunsoftFME7) Restore(s *State) {
	m.restoreRAM(s)
	m.command = s.Regs[0]
	m.workReg = s.Regs[1]
	copy(m.prgBanks[:], s.Regs[2:5])
	copy(m.chrBanks[:], s.Regs[5:13])
	m.irqEnabled = s.Regs[13]&1 != 0
	m.irqCounterEnabled = s.Regs[13]&2 != 0
	m.irqLine = s.Regs[13]&4 != 0
	m.irqCounter = uint16(s.Regs[14]) | uint16(s.Regs[15])<<8
	m.mirroring = cartridge.Mirroring(s.Regs[16])
	copy(m.audio.Regs[:], s.Regs[17:33])
	m.audio.Current = s.Regs[33]
	for i := 0; i < 3; i++ {
		m.audio.Timer[i] = int16(uint16(s.Regs[34+i*2]) | uint16(s.Regs[35+i*2])<<8)
	}
	m.audio.ToneStep[0] = s.Regs[40] & 0x0F
	m.audio.ToneStep[1] = s.Regs[40] >> 4
	m.audio.ToneStep[2] = s.Regs[41] & 0x0F
	m.audio.ProcessTick = s.Regs[41]&0x10 != 0
}
