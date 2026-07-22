package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// VRC7 (mapper 85, Lagrange Point): three switchable 8 KiB PRG banks,
// eight 1 KiB CHR banks, register-controlled mirroring, PRG RAM gate and
// the shared VRC IRQ. The board's OPLL-derived FM synthesizer is not
// mixed yet; its registers are accepted and ignored.
type VRC7 struct {
	base

	prgBanks [3]byte
	chrBanks [8]byte
	control  byte

	irq vrcIRQ
}

// NewVRC7 wires a Konami VRC7 board.
func NewVRC7(c *cartridge.Cartridge) *VRC7 {
	m := &VRC7{base: makeBase(c)}
	m.applyControl()
	return m
}

func (m *VRC7) ramEnabled() bool { return m.control&0x80 != 0 }

func (m *VRC7) applyControl() {
	switch m.control & 0x03 {
	case 0:
		m.mirroring = cartridge.Vertical
	case 1:
		m.mirroring = cartridge.Horizontal
	case 2:
		m.mirroring = cartridge.SingleLow
	case 3:
		m.mirroring = cartridge.SingleHigh
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *VRC7) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		bank := m.prgBanks[(addr-0x8000)>>13]
		return window(m.prg, int(bank), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if !m.ramEnabled() {
			return m.openBus()
		}
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *VRC7) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 && m.ramEnabled() {
			m.writePRGRAM(addr, v)
		}
		return
	}

	// The board wires A4 to A3 (except for the audio register at $9010).
	if addr&0x10 != 0 && addr&0xF010 != 0x9010 {
		addr = (addr | 0x08) &^ 0x10
	}

	switch addr & 0xF038 {
	case 0x8000:
		m.prgBanks[0] = v & 0x3F
	case 0x8008:
		m.prgBanks[1] = v & 0x3F
	case 0x9000:
		m.prgBanks[2] = v & 0x3F
	case 0x9010, 0x9030:
		// FM synthesizer registers (not mixed yet).
	case 0xA000:
		m.chrBanks[0] = v
	case 0xA008:
		m.chrBanks[1] = v
	case 0xB000:
		m.chrBanks[2] = v
	case 0xB008:
		m.chrBanks[3] = v
	case 0xC000:
		m.chrBanks[4] = v
	case 0xC008:
		m.chrBanks[5] = v
	case 0xD000:
		m.chrBanks[6] = v
	case 0xD008:
		m.chrBanks[7] = v
	case 0xE000:
		m.control = v
		m.applyControl()
	case 0xE008:
		m.irq.reload = v
	case 0xF000:
		m.irq.setControl(v)
	case 0xF008:
		m.irq.ack()
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *VRC7) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>10]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *VRC7) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>10]), 0x400, addr, v)
}

// Tick advances the board by one cycle.
func (m *VRC7) Tick() { m.irq.tick() }

// IRQ reports whether the board is asserting the IRQ line.
func (m *VRC7) IRQ() bool { return m.irq.line }

// Save writes the board's mapper-specific state into s.
func (m *VRC7) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:3], m.prgBanks[:])
	copy(s.Regs[3:11], m.chrBanks[:])
	s.Regs[11] = m.control
	m.irq.save(s.Regs[12:19])
}

// Restore loads the board's mapper-specific state from s.
func (m *VRC7) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:3])
	copy(m.chrBanks[:], s.Regs[3:11])
	m.control = s.Regs[11]
	m.applyControl()
	m.irq.restore(s.Regs[12:19])
}
