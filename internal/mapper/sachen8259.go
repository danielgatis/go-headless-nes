package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Sachen8259 (mappers 137/138/139/141 = variants D/B/C/A): a Sachen glue
// board with an index/data register pair at $4100/$4101. It banks a
// single 32 KiB PRG window and several CHR windows, the CHR granularity
// and bank-OR pattern differing per variant. Ported from the reference emulator.
//
// Variants A/B/C use 2 KiB CHR windows with a per-variant left shift and
// a three-entry OR table; variant D uses 1 KiB windows with its own
// fixed high-bit wiring and a fixed last-4 KiB window.
type Sachen8259 struct {
	base

	variantD bool
	shift    byte
	chrOr    [3]byte

	currentReg byte
	regs       [8]byte
	mirror     cartridge.Mirroring
}

// NewSachen8259 wires the board for the variant selected by the mapper ID.
func NewSachen8259(c *cartridge.Cartridge) *Sachen8259 {
	m := &Sachen8259{base: makeBase(c), mirror: c.Mirroring}
	switch c.MapperID {
	case 141: // variant A
		m.shift, m.chrOr = 1, [3]byte{1, 0, 1}
	case 138: // variant B
		m.shift, m.chrOr = 0, [3]byte{0, 0, 0}
	case 139: // variant C
		m.shift, m.chrOr = 2, [3]byte{1, 2, 3}
	case 137: // variant D
		m.variantD = true
	}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen8259) WritePRG(addr uint16, v byte) {
	if addr >= 0x4100 && addr < 0x8000 {
		switch addr & 0xC101 {
		case 0x4100:
			m.currentReg = v & 0x07
		case 0x4101:
			m.regs[m.currentReg] = v & 0x07
			m.update()
		}
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// update recomputes mirroring from register 7. Banking is resolved per
// access below.
func (m *Sachen8259) update() {
	switch (m.regs[7] >> 1) & 0x03 {
	case 0:
		if m.variantD {
			m.mirror = cartridge.Horizontal
		} else {
			m.mirror = cartridge.Vertical
		}
	case 1:
		if m.variantD {
			m.mirror = cartridge.Vertical
		} else {
			m.mirror = cartridge.Horizontal
		}
	case 2:
		// The reference uses SetNametables(0,1,1,1); the repo has no such
		// arrangement, so approximate with single-screen high.
		m.mirror = cartridge.SingleHigh
	case 3:
		m.mirror = cartridge.SingleLow
	}
	if m.regs[7]&0x01 != 0 { // simple mode forces fixed mirroring
		if m.variantD {
			m.mirror = cartridge.Horizontal
		} else {
			m.mirror = cartridge.Vertical
		}
	}
}

func (m *Sachen8259) simpleMode() bool { return m.regs[7]&0x01 != 0 }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen8259) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return window(m.prg, int(m.regs[5]), 0x8000)[addr&0x7FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// chrBank1K returns the 1 KiB CHR page for a PPU address per the variant's
// wiring.
func (m *Sachen8259) chrBank1K(addr uint16) int {
	simple := m.simpleMode()
	if m.variantD {
		// Four 1 KiB windows; the fourth pair is fixed to the last 4 KiB.
		slot := addr >> 10 & 3
		switch slot {
		case 0:
			return int(m.regs[0])
		case 1:
			return int(m.regs[4]&0x01)<<4 | int(m.regs[bIf(simple, 0, 1)])
		case 2:
			return int(m.regs[4]&0x02)<<3 | int(m.regs[bIf(simple, 0, 2)])
		default:
			return int(m.regs[4]&0x04)<<2 | int(m.regs[6]&0x01)<<3 | int(m.regs[bIf(simple, 0, 3)])
		}
	}
	// A/B/C: two-KiB windows built from the register, shifted, with a
	// per-window OR. Resolve to 1 KiB granularity for the read path.
	win := addr >> 11 & 3
	chrHigh := m.regs[4] << 3
	var page int
	switch win {
	case 0:
		page = int(chrHigh|m.regs[0]) << m.shift
	case 1:
		page = int(chrHigh|m.regs[bIf(simple, 0, 1)])<<m.shift | int(m.chrOr[0])
	case 2:
		page = int(chrHigh|m.regs[bIf(simple, 0, 2)])<<m.shift | int(m.chrOr[1])
	default:
		page = int(chrHigh|m.regs[bIf(simple, 0, 3)])<<m.shift | int(m.chrOr[2])
	}
	// page is in 2 KiB units; convert to the 1 KiB page for this address.
	return page<<1 | int(addr>>10&1)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen8259) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrBank1K(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sachen8259) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrBank1K(addr), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Sachen8259) Mirroring() cartridge.Mirroring { return m.mirror }

// NametablePage reports which page a nametable slot maps to.
func (m *Sachen8259) NametablePage(table byte) byte {
	switch m.mirror {
	case cartridge.Horizontal:
		return table >> 1
	case cartridge.SingleHigh:
		return 1
	case cartridge.SingleLow:
		return 0
	default: // Vertical
		return table & 1
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Sachen8259) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.currentReg
	copy(s.Regs[1:9], m.regs[:])
	s.Regs[9] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sachen8259) Restore(s *State) {
	m.restoreRAM(s)
	m.currentReg = s.Regs[0]
	copy(m.regs[:], s.Regs[1:9])
	m.mirror = cartridge.Mirroring(s.Regs[9])
}

// bIf selects one of two register indices without a branch at the call
// site, keeping the CHR wiring readable.
func bIf(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}
