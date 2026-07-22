package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// vrcVariant distinguishes the VRC2/VRC4 revisions, which differ only in
// how the two register-select address lines are wired to the cartridge
// edge (and in VRC2's ignored low CHR bit and missing IRQ).
type vrcVariant byte

const (
	vrc2a   vrcVariant = iota // mapper 22
	vrc2b                     // 23
	vrc2c                     // 25
	vrc4a                     // 21
	vrc4b                     // 25
	vrc4c                     // 21
	vrc4d                     // 25
	vrc4e                     // 23
	vrc4f                     // 23
	vrc427                    // 27
	vrc4183                   // 183 (Konami VRC4 clone with a banked $6000 ROM window)
)

// VRC24 covers Konami's VRC2 and VRC4 boards (mappers 21, 22, 23, 25 and
// 27): 8 KiB PRG banking with a swappable fixed window (VRC4), eight
// 1 KiB CHR banks split into low/high nibble registers, register-
// controlled mirroring and the shared VRC IRQ (VRC4 only). Without a
// submapper the register lines of the conflicting revisions are ORed
// together (heuristics), which makes every licensed game work.
type VRC24 struct {
	base

	variant       vrcVariant
	useHeuristics bool
	hasIRQ        bool

	prgReg0 byte
	prgReg1 byte
	prgMode byte
	prg6000 byte // 183 only: 8 KiB PRG-ROM bank mapped read-only at $6000

	loCHR [8]byte
	hiCHR [8]byte

	irq vrcIRQ
}

// NewVRC24 wires a VRC2/VRC4 board for the given mapper/submapper pair.
func NewVRC24(c *cartridge.Cartridge) *VRC24 {
	m := &VRC24{base: makeBase(c)}

	switch c.MapperID {
	case 21:
		switch c.Submapper {
		case 2:
			m.variant = vrc4c
		default:
			m.variant = vrc4a
		}
	case 22:
		m.variant = vrc2a
	case 23:
		switch c.Submapper {
		case 1:
			m.variant = vrc4f
		case 2:
			m.variant = vrc4e
		case 3:
			m.variant = vrc2b
		default:
			m.variant = vrc2b
		}
	case 25:
		switch c.Submapper {
		case 2:
			m.variant = vrc4d
		case 3:
			m.variant = vrc2c
		default:
			m.variant = vrc4b
		}
	case 27:
		m.variant = vrc427
	case 183:
		m.variant = vrc4183
	}

	m.useHeuristics = c.Submapper == 0 && c.MapperID != 22 && c.MapperID != 27 && c.MapperID != 183
	// Only the VRC4 has the IRQ unit; with heuristics on, assume it is
	// present except on mapper 22 (always a VRC2).
	m.hasIRQ = m.variant >= vrc4a || (m.useHeuristics && c.MapperID != 22)
	return m
}

// isVRC2 reports whether the board is certainly a VRC2 revision.
func (m *VRC24) isVRC2() bool { return m.variant <= vrc2c }

// translate rewires the register-select lines A0/A1 for the board
// revision, ORing conflicting wirings together in heuristic mode.
func (m *VRC24) translate(addr uint16) uint16 {
	var a0, a1 uint16
	if m.useHeuristics {
		switch m.variant {
		case vrc2c, vrc4b, vrc4d: // mapper 25
			a0 = (addr >> 1) & 0x01
			a1 = addr & 0x01
			a0 |= (addr >> 3) & 0x01
			a1 |= (addr >> 2) & 0x01
		case vrc4a, vrc4c: // mapper 21
			a0 = (addr >> 1) & 0x01
			a1 = (addr >> 2) & 0x01
			a0 |= (addr >> 6) & 0x01
			a1 |= (addr >> 7) & 0x01
		default: // mapper 23 (vrc2b, vrc4e, vrc4f)
			a0 = addr & 0x01
			a1 = (addr >> 1) & 0x01
			a0 |= (addr >> 2) & 0x01
			a1 |= (addr >> 3) & 0x01
		}
	} else {
		switch m.variant {
		case vrc2a, vrc2c, vrc4b: // mappers 22 and 25
			a0 = (addr >> 1) & 0x01
			a1 = addr & 0x01
		case vrc4d: // mapper 25
			a0 = (addr >> 3) & 0x01
			a1 = (addr >> 2) & 0x01
		case vrc4a: // mapper 21
			a0 = (addr >> 1) & 0x01
			a1 = (addr >> 2) & 0x01
		case vrc4c: // mapper 21
			a0 = (addr >> 6) & 0x01
			a1 = (addr >> 7) & 0x01
		case vrc4e, vrc4183: // mapper 23 wiring (also mapper 183)
			a0 = (addr >> 2) & 0x01
			a1 = (addr >> 3) & 0x01
		default: // vrc2b, vrc4f, vrc427
			a0 = addr & 0x01
			a1 = (addr >> 1) & 0x01
		}
	}
	return (addr & 0xFF00) | (a1 << 1) | a0
}

func (m *VRC24) prgBank(addr uint16) int {
	switch (addr >> 13) & 3 {
	case 0: // $8000
		if m.prgMode != 0 {
			return -2
		}
		return int(m.prgReg0)
	case 1: // $A000
		return int(m.prgReg1)
	case 2: // $C000
		if m.prgMode != 0 {
			return int(m.prgReg0)
		}
		return -2
	default: // $E000
		return -1
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *VRC24) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return window(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.variant == vrc4183 {
			// 183 maps a switchable 8 KiB PRG-ROM bank here (read-only).
			return window(m.prg, int(m.prg6000), 0x2000)[addr&0x1FFF]
		}
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *VRC24) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			if m.variant == vrc4183 {
				m.prg6000 = byte(addr & 0x0F)
				return
			}
			m.writePRGRAM(addr, v)
		}
		return
	}

	addr = m.translate(addr) & 0xF00F

	switch {
	case addr >= 0x8000 && addr <= 0x8006:
		m.prgReg0 = v & 0x1F
	case (m.isVRC2() && addr >= 0x9000 && addr <= 0x9003) ||
		(!m.isVRC2() && addr >= 0x9000 && addr <= 0x9001):
		mask := byte(0x03)
		if !m.useHeuristics && m.isVRC2() {
			// A certain VRC2 only wires the first mirroring bit.
			mask = 0x01
		}
		switch v & mask {
		case 0:
			m.mirroring = cartridge.Vertical
		case 1:
			m.mirroring = cartridge.Horizontal
		case 2:
			m.mirroring = cartridge.SingleLow
		case 3:
			m.mirroring = cartridge.SingleHigh
		}
	case !m.isVRC2() && addr >= 0x9002 && addr <= 0x9003:
		m.prgMode = (v >> 1) & 0x01
	case addr >= 0xA000 && addr <= 0xA006:
		m.prgReg1 = v & 0x1F
	case addr >= 0xB000 && addr <= 0xE006:
		reg := ((((addr >> 12) & 0x07) - 3) << 1) + ((addr >> 1) & 0x01)
		if addr&0x01 == 0 {
			m.loCHR[reg] = v & 0x0F
		} else {
			m.hiCHR[reg] = v & 0x1F
		}
	case addr == 0xF000:
		m.irq.setReloadLow(v)
	case addr == 0xF001:
		m.irq.setReloadHigh(v)
	case addr == 0xF002:
		m.irq.setControl(v)
	case addr == 0xF003:
		m.irq.ack()
	}
}

func (m *VRC24) chrBank(addr uint16) int {
	page := int(m.loCHR[addr>>10]) | int(m.hiCHR[addr>>10])<<4
	if m.variant == vrc2a {
		// VRC2a ignores the low CHR bit: the registers hold page*2.
		page >>= 1
	}
	return page
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *VRC24) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrBank(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *VRC24) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrBank(addr), 0x400, addr, v)
}

// Tick advances the board by one cycle.
func (m *VRC24) Tick() {
	if m.hasIRQ {
		m.irq.tick()
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *VRC24) IRQ() bool { return m.irq.line }

// Save writes the board's mapper-specific state into s.
func (m *VRC24) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg0
	s.Regs[1] = m.prgReg1
	s.Regs[2] = m.prgMode
	copy(s.Regs[3:11], m.loCHR[:])
	copy(s.Regs[11:19], m.hiCHR[:])
	s.Regs[19] = byte(m.mirroring)
	m.irq.save(s.Regs[20:27])
	s.Regs[27] = m.prg6000
}

// Restore loads the board's mapper-specific state from s.
func (m *VRC24) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg0 = s.Regs[0]
	m.prgReg1 = s.Regs[1]
	m.prgMode = s.Regs[2]
	copy(m.loCHR[:], s.Regs[3:11])
	copy(m.hiCHR[:], s.Regs[11:19])
	m.mirroring = cartridge.Mirroring(s.Regs[19])
	m.irq.restore(s.Regs[20:27])
	m.prg6000 = s.Regs[27]
}
