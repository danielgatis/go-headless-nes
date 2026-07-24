package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Sachen9602 (mapper 513): Sachen's 3D Block variant board, an MMC3
// whose CHR is 32 KiB of battery RAM and whose PRG space extends past
// the MMC3's 512 KiB with an outer-bank register: bits 6-7 of a bank
// data write to an even register supply PRG A19-A20. The two fixed PRG
// windows always point at banks $3E/$3F.
type Sachen9602 struct {
	MMC3

	reg   byte // last $8000 write (mirrors bankSelect, pre-mask)
	outer byte // PRG A19-A20 latch from $8001 bit 6-7

	chr32 [32768]byte // 32 KiB CHR RAM (larger than base's 8 KiB)
}

// NewSachen9602 wires the board.
func NewSachen9602(c *cartridge.Cartridge) *Sachen9602 {
	return &Sachen9602{MMC3: *NewMMC3(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen9602) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prgBank9602(addr), 0x2000)[addr&0x1FFF]
	}
	return m.MMC3.ReadPRG(addr)
}

// prgBank9602 is the MMC3 PRG map with the outer bank applied to the
// switchable windows and the fixed windows forced to $3E/$3F.
func (m *Sachen9602) prgBank9602(addr uint16) int {
	swap := m.bankSelect&0x40 != 0
	outer := int(m.outer) << 6
	switch addr >> 13 & 3 {
	case 0: // $8000
		if swap {
			return 0x3E
		}
		return int(m.banks[6]&0x3F) | outer
	case 1: // $A000
		return int(m.banks[7]&0x3F) | outer
	case 2: // $C000
		if swap {
			return int(m.banks[6]&0x3F) | outer
		}
		return 0x3E
	default: // $E000
		return 0x3F
	}
}

// WritePRG handles a CPU write into the PRG address space.
func (m *Sachen9602) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		switch addr & 0xE001 {
		case 0x8000:
			m.reg = v
		case 0x8001:
			// Bank data for the PRG/CHR registers carries the outer PRG
			// bank in its top bits (the CHR-RAM banks are only 5 bits).
			if m.reg&0x07 < 6 {
				m.outer = v >> 6
				v &= 0x1F
			}
		}
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen9602) ReadCHR(addr uint16) byte {
	p := m.chrPage1K(addr) & 0x1F
	return m.chr32[p<<10|int(addr&0x3FF)]
}

// WriteCHR handles a write into the CHR address space.
func (m *Sachen9602) WriteCHR(addr uint16, v byte) {
	p := m.chrPage1K(addr) & 0x1F
	m.chr32[p<<10|int(addr&0x3FF)] = v
}

// Save writes the board's mapper-specific state into s. The 32 KiB CHR
// RAM rides in the unused PRG-RAM tail plus the CHR-RAM area, so it
// round-trips fully.
func (m *Sachen9602) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.reg
	s.Regs[18] = m.outer
	copy(s.PRGRAM[8192:], m.chr32[:24576])
	copy(s.CHRRAM[:], m.chr32[24576:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Sachen9602) Restore(s *State) {
	m.MMC3.Restore(s)
	m.reg = s.Regs[17]
	m.outer = s.Regs[18]
	copy(m.chr32[:24576], s.PRGRAM[8192:])
	copy(m.chr32[24576:], s.CHRRAM[:])
}
