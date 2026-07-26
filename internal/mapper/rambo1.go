package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Rambo1 (mapper 64, Tengen RAMBO-1): an MMC3 look-alike with three
// switchable 8 KiB PRG banks, 1 KiB CHR banking everywhere (an extra
// pair of registers splits the 2 KiB windows), and an IRQ counter
// clockable from A12 rises or from the CPU clock divided by four, with
// a small delay before the line asserts.
type Rambo1 struct {
	base

	reg8000 byte
	regs    [16]byte

	irqEnabled   bool
	irqCycleMode bool
	needReload   bool
	irqCounter   byte
	irqReload    byte
	cpuClock     byte
	needIrqDelay byte
	forceClock   bool
	irqLine      bool
}

// NewRambo1 wires a RAMBO-1 board.
func NewRambo1(c *cartridge.Cartridge) *Rambo1 {
	return &Rambo1{base: makeBase(c)}
}

func (m *Rambo1) currentRegister() byte { return m.reg8000 & 0x0F }
func (m *Rambo1) chrMode() byte         { return (m.reg8000 & 0x80) >> 7 }
func (m *Rambo1) prgMode() byte         { return (m.reg8000 & 0x40) >> 6 }

func (m *Rambo1) prgBank(addr uint16) int {
	slot := (addr - 0x8000) >> 13
	if m.prgMode() == 0 {
		switch slot {
		case 0:
			return int(m.regs[6])
		case 1:
			return int(m.regs[7])
		case 2:
			return int(m.regs[15])
		default:
			return -1
		}
	}
	switch slot {
	case 0:
		return int(m.regs[15])
	case 1:
		return int(m.regs[7])
	case 2:
		return int(m.regs[6])
	default:
		return -1
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Rambo1) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return m.win(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Rambo1) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	m.writeRegister(addr, v)
}

func (m *Rambo1) writeRegister(addr uint16, v byte) {
	switch addr & 0xE001 {
	case 0x8000:
		m.reg8000 = v
	case 0x8001:
		m.regs[m.currentRegister()] = v
	case 0xA000:
		if v&0x01 != 0 {
			m.mirroring = cartridge.Horizontal
		} else {
			m.mirroring = cartridge.Vertical
		}
	case 0xC000:
		m.irqReload = v
	case 0xC001:
		if m.irqCycleMode && v&0x01 == 0 {
			// Switching out of cycle mode takes a few CPU clocks, letting
			// the counter clock once more with the reload pending.
			m.forceClock = true
		}
		m.irqCycleMode = v&0x01 != 0
		if m.irqCycleMode {
			m.cpuClock = 0
		}
		m.needReload = true
	case 0xE000:
		m.irqEnabled = false
		m.irqLine = false
	case 0xE001:
		m.irqEnabled = true
	}
}

// clockIrqCounter reloads (with the +1/+2 quirk) or decrements the
// counter, scheduling the delayed IRQ line on zero.
func (m *Rambo1) clockIrqCounter(delay byte) {
	if m.needReload {
		if m.irqReload <= 1 {
			m.irqCounter = m.irqReload + 1
		} else {
			m.irqCounter = m.irqReload + 2
		}
		m.needReload = false
	} else if m.irqCounter == 0 {
		m.irqCounter = m.irqReload + 1
	}
	m.irqCounter--
	if m.irqCounter == 0 && m.irqEnabled {
		m.needIrqDelay = delay
	}
}

// Tick advances the board by one cycle.
func (m *Rambo1) Tick() {
	if m.needIrqDelay > 0 {
		m.needIrqDelay--
		if m.needIrqDelay == 0 {
			m.irqLine = true
		}
	}
	if m.irqCycleMode || m.forceClock {
		m.cpuClock = (m.cpuClock + 1) & 0x03
		if m.cpuClock == 0 {
			m.clockIrqCounter(1)
			m.forceClock = false
		}
	}
}

// Scanline is the filtered A12 rise; it clocks the counter only in
// scanline mode.
func (m *Rambo1) Scanline() {
	if !m.irqCycleMode {
		m.clockIrqCounter(2)
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Rambo1) IRQ() bool { return m.irqLine }

// page1K resolves the CHR 1 KiB page for a PPU address: registers 0/1
// normally cover the 2 KiB windows, unless the full-1 KiB bit assigns
// registers 8/9 to the odd slots. The CHR mode bit inverts A12.
func (m *Rambo1) page1K(addr uint16) int {
	if m.chrMode() != 0 {
		addr ^= 0x1000
	}
	slot := addr >> 10 & 7
	switch slot {
	case 0, 2:
		return int(m.regs[slot>>1])
	case 1, 3:
		if m.reg8000&0x20 != 0 {
			return int(m.regs[8+(slot>>1)])
		}
		return int(m.regs[slot>>1] | 0x01)
	default:
		return int(m.regs[slot-2])
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Rambo1) ReadCHR(addr uint16) byte {
	return m.chrRead(m.page1K(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Rambo1) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.page1K(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Rambo1) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.reg8000
	copy(s.Regs[1:17], m.regs[:])
	s.Regs[17] = boolByte(m.irqEnabled) | boolByte(m.irqCycleMode)<<1 |
		boolByte(m.needReload)<<2 | boolByte(m.forceClock)<<3 | boolByte(m.irqLine)<<4
	s.Regs[18] = m.irqCounter
	s.Regs[19] = m.irqReload
	s.Regs[20] = m.cpuClock
	s.Regs[21] = m.needIrqDelay
	s.Regs[22] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *Rambo1) Restore(s *State) {
	m.restoreRAM(s)
	m.reg8000 = s.Regs[0]
	copy(m.regs[:], s.Regs[1:17])
	m.irqEnabled = s.Regs[17]&1 != 0
	m.irqCycleMode = s.Regs[17]&2 != 0
	m.needReload = s.Regs[17]&4 != 0
	m.forceClock = s.Regs[17]&8 != 0
	m.irqLine = s.Regs[17]&16 != 0
	m.irqCounter = s.Regs[18]
	m.irqReload = s.Regs[19]
	m.cpuClock = s.Regs[20]
	m.needIrqDelay = s.Regs[21]
	m.mirroring = cartridge.Mirroring(s.Regs[22])
}

// Rambo158 (mapper 158): a RAMBO-1 whose CHR registers' top bit drives
// the nametable selection instead of the $A000 mirroring register.
type Rambo158 struct {
	Rambo1

	ntPages [4]byte
}

// NewRambo158 wires the Tengen 800037 board.
func NewRambo158(c *cartridge.Cartridge) *Rambo158 {
	return &Rambo158{Rambo1: Rambo1{base: makeBase(c)}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Rambo158) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 && addr&0xE001 == 0x8001 {
		nt := v >> 7
		if m.chrMode() == 1 {
			switch m.currentRegister() & 0x07 {
			case 2:
				m.ntPages[0] = nt
			case 3:
				m.ntPages[1] = nt
			case 4:
				m.ntPages[2] = nt
			case 5:
				m.ntPages[3] = nt
			}
		} else {
			switch m.currentRegister() & 0x07 {
			case 0:
				m.ntPages[0], m.ntPages[1] = nt, nt
			case 1:
				m.ntPages[2], m.ntPages[3] = nt, nt
			}
		}
	}
	if addr < 0x8000 || addr&0xE001 != 0xA000 {
		m.Rambo1.WritePRG(addr, v)
	}
}

// NametablePage overrides mirroring with the register-driven tables.
func (m *Rambo158) NametablePage(table byte) byte { return m.ntPages[table&3] }

// Save writes the board's mapper-specific state into s.
func (m *Rambo158) Save(s *State) {
	m.Rambo1.Save(s)
	copy(s.Regs[23:27], m.ntPages[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Rambo158) Restore(s *State) {
	m.Rambo1.Restore(s)
	copy(m.ntPages[:], s.Regs[23:27])
}
