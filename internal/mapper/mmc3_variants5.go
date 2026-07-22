package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// More MMC3-derived boards ported from the reference emulator. These are outer-bank clones
// that fit the repo's fixed 8 KiB PRG-RAM / single CHR model cleanly.

// MMC3187 (mappers 186, 187): a $5000/$6000 register can force PRG into
// 8/16/32 KiB modes; the CHR high half always carries A17; and $8001 is
// gated behind a preceding $8000 write. A read-back register returns a
// small security table.
type MMC3187 struct {
	MMC3

	exReg0 byte
	exReg1 byte
}

// NewMMC3187 wires the board.
func NewMMC3187(c *cartridge.Cartridge) *MMC3187 {
	return &MMC3187{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3187) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr == 0x5000 || addr == 0x6000 {
			m.exReg0 = v
		}
		return
	}
	switch addr {
	case 0x8000:
		m.exReg1 = 1
		m.MMC3.WritePRG(addr, v)
	case 0x8001:
		if m.exReg1 == 1 {
			m.MMC3.WritePRG(addr, v)
		}
	default:
		m.MMC3.WritePRG(addr, v)
	}
}

var security187 = [4]byte{0x83, 0x83, 0x42, 0x00}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3187) ReadPRG(addr uint16) byte {
	if addr >= 0x5000 && addr < 0x6000 {
		return security187[m.exReg1&0x03]
	}
	if addr >= 0x8000 && m.exReg0&0x80 != 0 {
		return window(m.prg, m.prg187Slot(int(addr>>13&3)), 0x2000)[addr&0x1FFF]
	}
	return m.MMC3.ReadPRG(addr)
}

// prg187Slot mirrors the reference's per-slot PRG select override for the forced modes.
func (m *MMC3187) prg187Slot(slot int) int {
	ex := int(m.exReg0 & 0x1F)
	if m.exReg0&0x20 != 0 {
		if m.exReg0&0x40 != 0 {
			// 32 KiB, page-granular.
			return (ex & 0xFC) + slot
		}
		// 32 KiB, 16 KiB-granular base doubled.
		return (ex&0xFE)<<1 + slot
	}
	// 16 KiB mirrored into both halves.
	base := ex << 1
	switch slot {
	case 0, 2:
		return base + (slot & 1)
	default:
		return base + 1
	}
}

func (m *MMC3187) chrPage(addr uint16) int {
	// The $1000-$1FFF half always carries CHR A17 (slots 4-7 |= 0x100).
	sel := addr >> 12 & 1
	if m.bankSelect&0x80 != 0 {
		sel ^= 1
	}
	p := m.chrPage1K(addr)
	if sel != 0 {
		p |= 0x100
	}
	return p
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3187) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3187) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *MMC3187) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.exReg0
	s.Regs[18] = m.exReg1
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3187) Restore(s *State) {
	m.MMC3.Restore(s)
	m.exReg0 = s.Regs[17]
	m.exReg1 = s.Regs[18]
}

// MMC3189 (mapper 189, TXC): a $4120-$7FFF register forces a fixed
// 32 KiB PRG bank; the two nibbles of the register are OR'd together.
type MMC3189 struct {
	MMC3

	prgReg byte
}

// NewMMC3189 wires the board.
func NewMMC3189(c *cartridge.Cartridge) *MMC3189 {
	return &MMC3189{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3189) WritePRG(addr uint16, v byte) {
	if addr >= 0x4120 && addr < 0x8000 {
		m.prgReg = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3189) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	page := int((m.prgReg|m.prgReg>>4)&0x07)*4 + int(addr>>13&3)
	return window(m.prg, page, 0x2000)[addr&0x1FFF]
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3189) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.prgReg }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3189) Restore(s *State) { m.MMC3.Restore(s); m.prgReg = s.Regs[17] }

// MMC3224 (mapper 224, Jncota KT-008): an MMC3 with a $5000 outer-bank
// register adding PRG A20 (bit 2 -> a 512 KiB block), for up to 1 MiB.
type MMC3224 struct {
	MMC3

	outerBank int
}

// NewMMC3224 wires the board.
func NewMMC3224(c *cartridge.Cartridge) *MMC3224 {
	return &MMC3224{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3224) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr <= 0x5003 {
		m.outerBank = int(v>>2) & 0x01
		return
	}
	m.MMC3.WritePRG(addr, v)
}

func (m *MMC3224) prgSlot(slot int) int {
	or := m.outerBank << 6
	if m.bankSelect&0x40 == 0 {
		switch slot {
		case 0:
			return int(m.banks[6]&0x3F) | or
		case 1:
			return int(m.banks[7]&0x3F) | or
		case 2:
			return 0x3E | or
		default:
			return 0x3F | or
		}
	}
	switch slot {
	case 0:
		return 0x3E | or
	case 1:
		return int(m.banks[6]&0x3F) | or
	case 2:
		return int(m.banks[7]&0x3F) | or
	default:
		return 0x3F | or
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3224) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return window(m.prg, m.prgSlot(int(addr>>13&3)), 0x2000)[addr&0x1FFF]
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3224) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = byte(m.outerBank) }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3224) Restore(s *State) { m.MMC3.Restore(s); m.outerBank = int(s.Regs[17]) }
