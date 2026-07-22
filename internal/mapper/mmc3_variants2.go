package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// This file continues the MMC3-derived boards begun in mmc3_variants.go
// and mmc3_multi.go. Each board embeds the stock MMC3 (mapper 4) and
// overrides only the wiring that differs — an outer bank register, a CHR
// RAM window, a scrambled address decode — leaving the shared IRQ counter
// and register file intact. Extra state packs into s.Regs from index 17
// up, above the 17 bytes MMC3 itself uses.

// MMC3ChrRAM (mappers 74, 119, 191, 192, 194, 195) is an MMC3 that wires
// a contiguous range of CHR bank numbers to CHR RAM instead of ROM. When
// a 1 KiB CHR page falls in [firstRAM, lastRAM] it addresses RAM at page
// index page-firstRAM; otherwise it addresses ROM. This mirrors the reference's
// MMC3_ChrRam(firstRamBank, lastRamBank, chrRamSize):
//
//	74:  $08-$09 -> RAM (2 KiB)   119: $40-$7F -> RAM (8 KiB, TQROM)
//	191: $80-$FF -> RAM (2 KiB)   192: $08-$0B -> RAM (4 KiB)
//	194: $00-$01 -> RAM (2 KiB)   195: $00-$03 -> RAM (4 KiB)
type MMC3ChrRAM struct {
	MMC3

	firstRAM int // first 1 KiB CHR page routed to RAM
	lastRAM  int // last 1 KiB CHR page routed to RAM (inclusive)
	ramPages int // 1 KiB CHR RAM pages; RAM index wraps modulo this
}

// NewMMC3ChrRAM wires a board that overlays CHR RAM into the MMC3 CHR map.
func NewMMC3ChrRAM(c *cartridge.Cartridge) *MMC3ChrRAM {
	m := &MMC3ChrRAM{MMC3: *NewMMC3(c)}
	switch c.MapperID {
	case 74:
		m.firstRAM, m.lastRAM, m.ramPages = 0x08, 0x09, 2
	case 119:
		m.firstRAM, m.lastRAM, m.ramPages = 0x40, 0x7F, 8
	case 191:
		m.firstRAM, m.lastRAM, m.ramPages = 0x80, 0xFF, 2
	case 192:
		m.firstRAM, m.lastRAM, m.ramPages = 0x08, 0x0B, 4
	case 194:
		m.firstRAM, m.lastRAM, m.ramPages = 0x00, 0x01, 2
	case 195:
		m.firstRAM, m.lastRAM, m.ramPages = 0x00, 0x03, 4
	}
	return m
}

// chrRAMPage returns the 1 KiB CHR RAM page for a raw CHR page, or -1 if
// that page addresses ROM. The RAM index wraps modulo the RAM size, as an
// unwired upper address line would on hardware.
func (m *MMC3ChrRAM) chrRAMPage(page int) int {
	if page >= m.firstRAM && page <= m.lastRAM {
		return (page - m.firstRAM) % m.ramPages
	}
	return -1
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3ChrRAM) ReadCHR(addr uint16) byte {
	page := m.chrPage1K(addr)
	if p := m.chrRAMPage(page); p >= 0 {
		return m.chrRAM[p<<10|int(addr&0x3FF)]
	}
	return window(m.chr, page, 0x400)[addr&0x3FF]
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3ChrRAM) WriteCHR(addr uint16, v byte) {
	page := m.chrPage1K(addr)
	if p := m.chrRAMPage(page); p >= 0 {
		m.chrRAM[p<<10|int(addr&0x3FF)] = v
	}
}

// MMC3115 (mapper 115, Yuu Yuu Hakusho / Kǎshèng SFC boards): registers
// at $6000 and $6001 add a PRG outer bank and a CHR outer bank, plus a
// mode bit that forces a fixed 16 KiB PRG bank at $8000.
type MMC3115 struct {
	MMC3

	prgReg byte
	chrReg byte
}

// NewMMC3115 wires the board.
func NewMMC3115(c *cartridge.Cartridge) *MMC3115 {
	return &MMC3115{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3115) WritePRG(addr uint16, v byte) {
	// The board decodes its outer registers across $4100-$7FFF, even
	// address = PRG reg, odd = CHR reg. ($5080 is a protection latch we
	// don't emulate.) Everything at $8000+ is the stock MMC3.
	if addr >= 0x4100 && addr < 0x8000 {
		if addr == 0x5080 {
			return
		}
		if addr&1 == 0 {
			m.prgReg = v
		} else {
			m.chrReg = v & 0x01
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3115) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	if m.prgReg&0x80 != 0 {
		if m.prgReg&0x20 != 0 {
			// 32 KiB mode: four contiguous 8 KiB pages from ((reg&0x0F)>>1)<<2.
			page := int((m.prgReg&0x0F)>>1)<<2 | int(addr>>13&3)
			return window(m.prg, page, 0x2000)[addr&0x1FFF]
		}
		// 16 KiB mode: the same 16 KiB bank mirrors into $8000 and $C000.
		page := int(m.prgReg&0x0F)<<1 | int(addr>>13&1)
		return window(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	return m.MMC3.ReadPRG(addr)
}

func (m *MMC3115) chrPage(addr uint16) int {
	return m.chrPage1K(addr) | int(m.chrReg&0x01)<<8
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3115) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3115) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3115) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.prgReg
	s.Regs[18] = m.chrReg
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3115) Restore(s *State) {
	m.MMC3.Restore(s)
	m.prgReg = s.Regs[17]
	m.chrReg = s.Regs[18]
}

// MMC3165 (mapper 165, Waixing): an MMC3 banking CHR in 4 KiB windows,
// each carrying an MMC2-style $FD/$FE latch that picks between two of the
// MMC3 bank registers. Register value 0 in a window selects 4 KiB of CHR
// RAM instead of ROM; any other value selects a 4 KiB CHR ROM bank at
// (reg>>2). The low window latches on R0/R1, the high window on R2/R4.
type MMC3165 struct {
	MMC3

	latch [2]bool // false = FD (or reset) state, true = FE state
}

// NewMMC3165 wires the board.
func NewMMC3165(c *cartridge.Cartridge) *MMC3165 {
	return &MMC3165{MMC3: *NewMMC3(c)}
}

// chrReg returns the MMC3 bank register selected for a 4 KiB window given
// its current latch.
func (m *MMC3165) chrReg(win int) byte {
	if win == 0 {
		if m.latch[0] {
			return m.banks[1]
		}
		return m.banks[0]
	}
	if m.latch[1] {
		return m.banks[4]
	}
	return m.banks[2]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3165) ReadCHR(addr uint16) byte {
	win := int(addr >> 12 & 1)
	reg := m.chrReg(win)
	var v byte
	if reg == 0 {
		v = m.chrRAM[addr&0x0FFF] // 4 KiB CHR RAM at bank 0
	} else {
		v = window(m.chr, int(reg)>>2, 0x1000)[addr&0x0FFF]
	}
	m.updateLatch(addr)
	return v
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3165) WriteCHR(addr uint16, v byte) {
	win := int(addr >> 12 & 1)
	if m.chrReg(win) == 0 {
		m.chrRAM[addr&0x0FFF] = v
	}
	m.updateLatch(addr)
}

// updateLatch flips a window's latch on the MMC2-style trigger fetches
// ($xFD0/$xFE8 rows), the window selected by A12.
func (m *MMC3165) updateLatch(addr uint16) {
	switch addr & 0x0FF8 {
	case 0x0FD0:
		m.latch[addr>>12&1] = false
	case 0x0FE8:
		m.latch[addr>>12&1] = true
	}
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3165) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = boolByte(m.latch[0])
	s.Regs[18] = boolByte(m.latch[1])
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3165) Restore(s *State) {
	m.MMC3.Restore(s)
	m.latch[0] = s.Regs[17] != 0
	m.latch[1] = s.Regs[18] != 0
}
