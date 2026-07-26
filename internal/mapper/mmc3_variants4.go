package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Further MMC3-derived boards ported from the reference emulator.

// security357 permutes the low 3 bits of a bank-select write, shared by
// several MMC3 pirate clones (114, 123).
var security357 = [8]byte{0, 3, 1, 5, 6, 7, 2, 4}

// MMC3114 (mapper 114): a $5000-$7FFF register can force a 16 KiB PRG
// bank; the $8000-region register addresses are remapped and the
// bank-select field is permuted.
type MMC3114 struct {
	MMC3

	exReg0 byte
	exReg1 byte
}

// NewMMC3114 wires the board.
func NewMMC3114(c *cartridge.Cartridge) *MMC3114 {
	return &MMC3114{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3114) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x8000 {
		m.exReg0 = v
		return
	}
	if addr < 0x8000 {
		m.MMC3.WritePRG(addr, v)
		return
	}
	switch addr & 0xE001 {
	case 0x8001:
		m.MMC3.WritePRG(0xA000, v)
	case 0xA000:
		m.MMC3.WritePRG(0x8000, v&0xC0|security357[v&0x07])
		m.exReg1 = 1
	case 0xA001:
		m.irqLatch = v
	case 0xC000:
		if m.exReg1 != 0 {
			m.exReg1 = 0
			m.MMC3.WritePRG(0x8001, v)
		}
	case 0xC001:
		m.irqCounter = 0
		m.irqReload = true
	case 0xE000:
		m.irqEnabled = false
		m.irqAsserted = false
	case 0xE001:
		m.irqEnabled = true
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3114) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 && m.exReg0&0x80 != 0 {
		// Forced 16 KiB bank mirrored into both halves.
		page := int(m.exReg0&0x0F)<<1 | int(addr>>13&1)
		return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	return m.MMC3.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3114) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.exReg0
	s.Regs[18] = m.exReg1
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3114) Restore(s *State) {
	m.MMC3.Restore(s)
	m.exReg0 = s.Regs[17]
	m.exReg1 = s.Regs[18]
}

// MMC3123 (mapper 123): a $5800-$5FFF register can force a 16/32 KiB PRG
// bank from a scrambled nibble; the $8000/$8001 pair is permuted.
type MMC3123 struct {
	MMC3

	exReg0 byte
	exReg1 byte
}

// NewMMC3123 wires the board.
func NewMMC3123(c *cartridge.Cartridge) *MMC3123 {
	return &MMC3123{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3123) WritePRG(addr uint16, v byte) {
	switch {
	case addr < 0x8000 && addr&0x0800 != 0:
		if addr&1 != 0 {
			m.exReg1 = v
		} else {
			m.exReg0 = v
		}
	case addr < 0x8000:
		m.MMC3.WritePRG(addr, v)
	case addr < 0xA000:
		switch addr & 0x8001 {
		case 0x8000:
			m.MMC3.WritePRG(0x8000, v&0xC0|security357[v&0x07])
		case 0x8001:
			m.MMC3.WritePRG(0x8001, v)
		}
	default:
		m.MMC3.WritePRG(addr, v)
	}
}

// prg123Bank returns the forced bank for an 8 KiB slot when the override
// is active, else -1.
func (m *MMC3123) prg123Bank(addr uint16) int {
	if m.exReg0&0x40 == 0 {
		return -1
	}
	bank := int(m.exReg0&0x05) | int(m.exReg0&0x08)>>2 | int(m.exReg0&0x20)>>2
	if m.exReg0&0x02 != 0 {
		// 32 KiB: four contiguous pages from (bank&0xFE)<<1.
		return (bank&0xFE)<<1 | int(addr>>13&3)
	}
	// 16 KiB mirrored into both halves.
	return bank<<1 | int(addr>>13&1)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3123) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		if b := m.prg123Bank(addr); b >= 0 {
			return m.win(m.prg, b, 0x2000)[addr&0x1FFF]
		}
	}
	return m.MMC3.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3123) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.exReg0
	s.Regs[18] = m.exReg1
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3123) Restore(s *State) {
	m.MMC3.Restore(s)
	m.exReg0 = s.Regs[17]
	m.exReg1 = s.Regs[18]
}

// MMC3134 (mapper 134): a $6001 register adds a PRG outer bank (bit 1 ->
// A19, masking the inner page to 5 bits) and a CHR outer bank (bit 5 ->
// A16, masking to 8 bits).
type MMC3134 struct {
	MMC3

	exReg byte
}

// NewMMC3134 wires the board.
func NewMMC3134(c *cartridge.Cartridge) *MMC3134 {
	return &MMC3134{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3134) WritePRG(addr uint16, v byte) {
	if addr == 0x6001 {
		m.exReg = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3134) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	page := p&0x1F | int(m.exReg&0x02)<<4
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3134) chrPage(addr uint16) int {
	return m.chrPage1K(addr)&0xFF | int(m.exReg&0x20)<<3
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3134) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3134) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *MMC3134) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.exReg }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3134) Restore(s *State) { m.MMC3.Restore(s); m.exReg = s.Regs[17] }

// MMC3249 (mapper 249, Waixing): a $5000 register whose bit 1, when set,
// scrambles the PRG and CHR bank numbers through fixed bit permutations.
type MMC3249 struct {
	MMC3

	exReg byte
}

// NewMMC3249 wires the board.
func NewMMC3249(c *cartridge.Cartridge) *MMC3249 {
	return &MMC3249{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3249) WritePRG(addr uint16, v byte) {
	if addr == 0x5000 {
		m.exReg = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

func scramble249CHR(page int) int {
	return (page & 0x03) | (page>>1)&0x04 | (page>>4)&0x08 | (page>>2)&0x10 | (page<<3)&0x20 | (page<<2)&0xC0
}

func scramble249PRG(page int) int {
	if page < 0x20 {
		return (page & 0x01) | (page>>3)&0x02 | (page>>1)&0x04 | (page<<2)&0x18
	}
	page -= 0x20
	return scramble249CHR(page)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3249) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	if m.exReg&0x02 != 0 {
		p = scramble249PRG(p)
	}
	return m.win(m.prg, p, 0x2000)[addr&0x1FFF]
}

func (m *MMC3249) chrPage(addr uint16) int {
	p := m.chrPage1K(addr)
	if m.exReg&0x02 != 0 {
		p = scramble249CHR(p)
	}
	return p
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3249) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3249) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *MMC3249) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.exReg }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3249) Restore(s *State) { m.MMC3.Restore(s); m.exReg = s.Regs[17] }
