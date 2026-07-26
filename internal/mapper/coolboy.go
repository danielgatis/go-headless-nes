package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// MMC3Coolboy (mapper 268, COOLBOY/MINDKIDS multicarts): an MMC3 clone
// whose four extra registers re-wire the PRG/CHR bank lines around the
// stock banking, addressing up to 256 KiB of CHR RAM. Submapper 1
// (MINDKIDS) takes its registers at $5000-$5FFF instead of $6000-$7FFF.
type MMC3Coolboy struct {
	MMC3

	sub     byte
	exRegs  [4]byte
	bigCHR  [0x40000]byte // 256 KiB CHR RAM (not snapshotted; see Save)
	usesROM bool
}

// NewMMC3Coolboy wires the board.
func NewMMC3Coolboy(c *cartridge.Cartridge) *MMC3Coolboy {
	m := &MMC3Coolboy{MMC3: *NewMMC3(c), sub: c.Submapper}
	m.usesROM = c.CHR != nil
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Coolboy) WritePRG(addr uint16, v byte) {
	regStart := uint16(0x6000)
	if m.sub == 1 {
		regStart = 0x5000
	}
	if addr >= regStart && addr < 0x8000 {
		if m.ramEnabled && addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		if (m.sub != 1 || addr < 0x6000) && m.exRegs[3]&0x90 != 0x80 {
			m.exRegs[addr&0x03] = v
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// prgPage applies the COOLBOY outer-bank rewiring to the MMC3-resolved
// 8 KiB page.
func (m *MMC3Coolboy) prgPage(a uint16) int {
	slot := uint32((a - 0x8000) >> 13)
	addr := 0x8000 + slot*0x2000
	page := uint32(m.prgBank(a)) & 0xFFFF // fixed banks arrive as $FFFE/$FFFF
	prgMode := m.bankSelect&0x40 != 0

	mask := (uint32(0x3F) | uint32(m.exRegs[1]&0x40) | uint32(m.exRegs[1]&0x20)<<2) ^
		(uint32(m.exRegs[0]&0x40) >> 2) ^ (uint32(m.exRegs[1]&0x80) >> 2)
	base := uint32(m.exRegs[0]&0x07) | uint32(m.exRegs[1]&0x10)>>1 |
		uint32(m.exRegs[1]&0x0C)<<2 | uint32(m.exRegs[0]&0x30)<<2

	if m.exRegs[3]&0x40 != 0 && page >= 0xFE && prgMode {
		switch slot {
		case 1:
			if prgMode {
				page = 0
			}
		case 2:
			if !prgMode {
				page = 0
			}
		case 3:
			page = 0
		}
	}

	if m.exRegs[3]&0x10 == 0 {
		return int((base << 4 &^ mask) | (page & mask))
	}
	mask &= 0xF0
	var emask uint32
	if m.exRegs[1]&0x02 != 0 {
		emask = uint32(m.exRegs[3]&0x0C) | (addr&0x4000)>>13
	} else {
		emask = uint32(m.exRegs[3] & 0x0E)
	}
	return int((base << 4 &^ mask) | (page & mask) | emask | (slot & 0x01))
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Coolboy) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return m.win(m.prg, m.prgPage(addr), 0x2000)[addr&0x1FFF]
}

// chrPage applies the COOLBOY CHR rewiring to the MMC3-resolved 1 KiB
// page.
func (m *MMC3Coolboy) chrPage(a uint16) int {
	page := uint32(m.chrPage1K(a))
	addr := uint32(a) &^ 0x3FF
	slot := addr >> 10
	mask := 0xFF ^ uint32(m.exRegs[0]&0x80)
	var cbase uint32
	if m.bankSelect&0x80 != 0 {
		cbase = 0x1000
	}

	if m.exRegs[3]&0x10 != 0 {
		if m.exRegs[3]&0x40 != 0 {
			switch cbase ^ addr {
			case 0x0400, 0x0C00:
				page &= 0x7F
			}
		}
		return int((page & 0x80 & mask) | (uint32(m.exRegs[0]&0x08) << 4 &^ mask) |
			uint32(m.exRegs[2]&0x0F)<<3 | slot)
	}
	if m.exRegs[3]&0x40 != 0 {
		switch cbase ^ addr {
		case 0x0000:
			page = uint32(m.banks[0])
		case 0x0800:
			page = uint32(m.banks[1])
		case 0x0400, 0x0C00:
			page = 0
		}
	}
	return int((page & mask) | (uint32(m.exRegs[0]&0x08) << 4 &^ mask))
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Coolboy) ReadCHR(addr uint16) byte {
	page := m.chrPage(addr)
	if m.usesROM {
		return m.win(m.chr, page, 0x400)[addr&0x3FF]
	}
	return m.bigCHR[(page<<10|int(addr&0x3FF))%len(m.bigCHR)]
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Coolboy) WriteCHR(addr uint16, v byte) {
	if !m.usesROM {
		page := m.chrPage(addr)
		m.bigCHR[(page<<10|int(addr&0x3FF))%len(m.bigCHR)] = v
	}
}

// Save snapshots the registers; the 256 KiB CHR RAM exceeds the
// fixed-size snapshot and only its first 8 KiB round-trips (rewind on
// these multicart menus is best-effort).
func (m *MMC3Coolboy) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:21], m.exRegs[:])
	copy(s.CHRRAM[:], m.bigCHR[:len(s.CHRRAM)])
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Coolboy) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.exRegs[:], s.Regs[17:21])
	copy(m.bigCHR[:len(s.CHRRAM)], s.CHRRAM[:])
}
