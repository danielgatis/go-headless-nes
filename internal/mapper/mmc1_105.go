package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// MMC1Event (mapper 105, NES-EVENT, Nintendo World Championships): an
// MMC1 whose CHR register 0 drives an init-state machine, a 30-bit
// CPU-cycle IRQ timer (the competition countdown, extended by the DIP
// switches) and a two-level PRG scheme: a 32 KiB outer mode or the
// regular MMC1 modes over the upper 128 KiB.
type MMC1Event struct {
	MMC1

	dips byte // competition-time DIP switches (0-15)

	initState  byte
	irqCounter uint32
	irqEnabled bool
	irqLine    bool
}

// NewMMC1Event wires the NES-EVENT board.
func NewMMC1Event(c *cartridge.Cartridge) *MMC1Event {
	m := &MMC1Event{MMC1: *NewMMC1(c)}
	m.chr0 |= 0x10 // the I bit starts set
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC1Event) WritePRG(addr uint16, v byte) {
	m.MMC1.WritePRG(addr, v)
	m.updateEventState()
}

// updateEventState tracks the CHR-register I bit: the ROM clears then
// sets it once at boot (arming the timer), and the PRG map stays on the
// first bank until that handshake completes.
func (m *MMC1Event) updateEventState() {
	if m.initState == 0 && m.chr0&0x10 == 0 {
		m.initState = 1
	} else if m.initState == 1 && m.chr0&0x10 != 0 {
		m.initState = 2
	}
	if m.chr0&0x10 != 0 {
		m.irqEnabled = false
		m.irqCounter = 0
		m.irqLine = false
	} else {
		m.irqEnabled = true
	}
}

// Tick advances the board by one cycle.
func (m *MMC1Event) Tick() {
	if m.irqEnabled {
		m.irqCounter++
		limit := uint32(0x20000000) | uint32(m.dips)<<25
		if m.irqCounter >= limit {
			m.irqLine = true
			m.irqEnabled = false
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *MMC1Event) IRQ() bool { return m.irqLine }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC1Event) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		if m.initState < 2 {
			return m.win(m.prg, int(addr>>14)&1, 0x4000)[addr&0x3FFF]
		}
		if m.chr0&0x08 != 0 {
			// MMC1 modes over the upper 128 KiB (banks 8-15).
			bank := int(m.prgBank&0x07) | 0x08
			low := addr < 0xC000
			switch m.control >> 2 & 3 {
			case 0, 1: // 32 KiB
				if low {
					return m.win(m.prg, bank&^1, 0x4000)[addr&0x3FFF]
				}
				return m.win(m.prg, bank|1, 0x4000)[addr&0x3FFF]
			case 2:
				if low {
					return m.win(m.prg, 0x08, 0x4000)[addr&0x3FFF]
				}
				return m.win(m.prg, bank, 0x4000)[addr&0x3FFF]
			default:
				if low {
					return m.win(m.prg, bank, 0x4000)[addr&0x3FFF]
				}
				return m.win(m.prg, 0x0F, 0x4000)[addr&0x3FFF]
			}
		}
		// 32 KiB outer mode from the CHR register.
		bank := int(m.chr0&0x06) | int(addr>>14)&1
		return m.win(m.prg, bank, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		if m.ramDisabled() {
			return m.openBus()
		}
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// Save writes the board's mapper-specific state into s.
func (m *MMC1Event) Save(s *State) {
	m.MMC1.Save(s)
	s.Regs[10] = m.initState
	s.Regs[11] = byte(m.irqCounter)
	s.Regs[12] = byte(m.irqCounter >> 8)
	s.Regs[13] = byte(m.irqCounter >> 16)
	s.Regs[14] = byte(m.irqCounter >> 24)
	s.Regs[15] = boolByte(m.irqEnabled) | boolByte(m.irqLine)<<1
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC1Event) Restore(s *State) {
	m.MMC1.Restore(s)
	m.initState = s.Regs[10]
	m.irqCounter = uint32(s.Regs[11]) | uint32(s.Regs[12])<<8 |
		uint32(s.Regs[13])<<16 | uint32(s.Regs[14])<<24
	m.irqEnabled = s.Regs[15]&1 != 0
	m.irqLine = s.Regs[15]&2 != 0
}
