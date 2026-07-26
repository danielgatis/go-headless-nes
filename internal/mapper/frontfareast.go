package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// FrontFareast (mappers 6, 8, 17): the FFE "Front Far East" FDS/copier
// conversion boards. They decode a register block at $42FE-$4517 (IRQ,
// mirroring) and, for 6/8, a $8000+ bank register; mapper 17 banks four
// 8 KiB PRG windows and eight 1 KiB CHR windows through the $45xx block.
// A 16-bit CPU-cycle IRQ counts up and fires on overflow.
//
// Ported from the reference emulator. PRG is 8 KiB-windowed (four slots);
// CHR is 8 KiB RAM in 1 KiB windows.
type FrontFareast struct {
	base

	id     uint16
	prgReg [4]byte // 8 KiB PRG banks
	chrReg [8]byte // 1 KiB CHR banks
	mirror cartridge.Mirroring

	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewFrontFareast wires the board for its mapper ID.
func NewFrontFareast(c *cartridge.Cartridge) *FrontFareast {
	m := &FrontFareast{base: makeBase(c), id: c.MapperID, mirror: c.Mirroring}
	// Power-on PRG layout per the reference emulator.
	switch c.MapperID {
	case 6:
		m.prgReg = [4]byte{0, 1, 14, 15}
	case 8:
		m.prgReg = [4]byte{0, 1, 2, 3}
	case 17:
		n := byte(len(m.prg) / 0x2000)
		m.prgReg = [4]byte{n - 4, n - 3, n - 2, n - 1}
	}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *FrontFareast) WritePRG(addr uint16, v byte) {
	switch addr {
	case 0x42FE:
		if v&0x10 != 0 {
			m.mirror = cartridge.SingleHigh
		} else {
			m.mirror = cartridge.SingleLow
		}
		return
	case 0x42FF:
		m.mirror = hvMirror(v&0x10 == 0)
		return
	case 0x4501:
		m.irqEnabled = false
		m.irqLine = false
		return
	case 0x4502:
		m.irqCounter = m.irqCounter&0xFF00 | uint16(v)
		m.irqLine = false
		return
	case 0x4503:
		m.irqCounter = m.irqCounter&0x00FF | uint16(v)<<8
		m.irqEnabled = true
		m.irqLine = false
		return
	}

	switch m.id {
	case 6:
		if addr >= 0x8000 {
			// bits 2-7 -> two 8 KiB PRG banks; bits 0-1 -> 8 KiB CHR page.
			p := (v & 0xFC) >> 1
			m.prgReg[0], m.prgReg[1] = p, p+1
			m.setCHR8(int(v&0x03) << 3)
		}
	case 8:
		if addr >= 0x8000 {
			p := (v & 0xF8) >> 2
			m.prgReg[0], m.prgReg[1] = p, p+1
			m.setCHR8(int(v&0x07) << 3)
		}
	default: // 17
		switch addr {
		case 0x4504, 0x4505, 0x4506, 0x4507:
			m.prgReg[addr-0x4504] = v
		case 0x4510, 0x4511, 0x4512, 0x4513, 0x4514, 0x4515, 0x4516, 0x4517:
			m.chrReg[addr-0x4510] = v
		}
	}

	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// setCHR8 maps eight consecutive 1 KiB CHR pages starting at base.
func (m *FrontFareast) setCHR8(base int) {
	for i := 0; i < 8; i++ {
		m.chrReg[i] = byte(base + i)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *FrontFareast) ReadPRG(addr uint16) byte {
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
func (m *FrontFareast) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *FrontFareast) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *FrontFareast) Mirroring() cartridge.Mirroring { return m.mirror }

// Tick counts the IRQ up each CPU cycle, asserting on 16-bit overflow.
func (m *FrontFareast) Tick() {
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
func (m *FrontFareast) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *FrontFareast) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.prgReg[:])
	copy(s.Regs[4:12], m.chrReg[:])
	s.Regs[12] = byte(m.mirror)
	s.Regs[13] = byte(m.irqCounter)
	s.Regs[14] = byte(m.irqCounter >> 8)
	s.Regs[15] = boolByte(m.irqEnabled)
	s.Regs[16] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *FrontFareast) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgReg[:], s.Regs[0:4])
	copy(m.chrReg[:], s.Regs[4:12])
	m.mirror = cartridge.Mirroring(s.Regs[12])
	m.irqCounter = uint16(s.Regs[13]) | uint16(s.Regs[14])<<8
	m.irqEnabled = s.Regs[15] != 0
	m.irqLine = s.Regs[16] != 0
}
