package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Action53 (mapper 28): the Action 53 homebrew multicart mapper. A
// register-select write to $5000-$5FFF picks one of four registers, and a
// data write to $8000-$FFFF sets it. The registers configure mirroring, a
// per-game PRG window size (16/32 KiB) with inner/outer bank split, and an
// 8 KiB CHR RAM bank. Ported from the reference emulator's Action 53 board.
type Action53 struct {
	base

	selected  byte
	regs      [4]byte
	mirrorBit byte
	mirror    cartridge.Mirroring
}

// NewAction53 wires the board.
func NewAction53(c *cartridge.Cartridge) *Action53 {
	m := &Action53{base: makeBase(c)}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Action53) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x5000 && addr < 0x6000:
		m.selected = (v&0x80)>>6 | (v & 0x01)
	case addr >= 0x8000:
		if m.selected <= 1 {
			m.mirrorBit = (v >> 4) & 0x01
		} else if m.selected == 2 {
			m.mirrorBit = v & 0x01
		}
		m.regs[m.selected] = v
		m.update()
	}
}

// update recomputes mirroring from the current registers. PRG/CHR banks
// are resolved per access in ReadPRG/ReadCHR.
func (m *Action53) update() {
	mir := m.regs[2] & 0x03
	if mir&0x02 == 0 {
		mir = m.mirrorBit
	}
	switch mir {
	case 0:
		m.mirror = cartridge.SingleLow
	case 1:
		m.mirror = cartridge.SingleHigh
	case 2:
		m.mirror = cartridge.Vertical
	case 3:
		m.mirror = cartridge.Horizontal
	}
}

// prgBanks returns the two 16 KiB PRG banks mapped at $8000 and $C000.
func (m *Action53) prgBanks() (bank0, bank1 int) {
	gameSize := (m.regs[2] & 0x30) >> 4
	prgSize := (m.regs[2] & 0x08) >> 3
	slotSelect := (m.regs[2] & 0x04) >> 2
	prgSelect := int(m.regs[1] & 0x0F)
	outer := int(m.regs[3]) << 1

	outerAnd := [4]int{0x1FE, 0x1FC, 0x1F8, 0x1F0}
	innerAnd := [4]int{0x01, 0x03, 0x07, 0x0F}

	if prgSize != 0 {
		// 32 KiB game window: one switchable bank + one fixed to slotSelect.
		var sw int
		switch gameSize {
		case 0:
			sw = (outer & 0x1FE) | (prgSelect & 0x01)
		case 1:
			sw = (outer & 0x1FC) | (prgSelect & 0x03)
		case 2:
			sw = (outer & 0x1F8) | (prgSelect & 0x07)
		default:
			sw = (outer & 0x1F0) | (prgSelect & 0x0F)
		}
		fixed := (outer & 0x1FE) | int(slotSelect)
		if slotSelect != 0 {
			return fixed, sw
		}
		return sw, fixed
	}
	// 16 KiB window doubled across both slots.
	ps := prgSelect << 1
	bank0 = (outer & outerAnd[gameSize]) | (ps & innerAnd[gameSize])
	bank1 = (outer & outerAnd[gameSize]) | ((ps | 0x01) & innerAnd[gameSize])
	return bank0, bank1
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Action53) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			return m.readPRGRAM(addr)
		}
		return m.openBus()
	}
	b0, b1 := m.prgBanks()
	if addr < 0xC000 {
		return m.win(m.prg, b0, 0x4000)[addr&0x3FFF]
	}
	return m.win(m.prg, b1, 0x4000)[addr&0x3FFF]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Action53) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.regs[0]&0x03), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Action53) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.regs[0]&0x03), 0x2000, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Action53) Mirroring() cartridge.Mirroring { return m.mirror }

// NametablePage reports which page a nametable slot maps to.
func (m *Action53) NametablePage(table byte) byte {
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
func (m *Action53) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.selected
	copy(s.Regs[1:5], m.regs[:])
	s.Regs[5] = m.mirrorBit
	s.Regs[6] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Action53) Restore(s *State) {
	m.restoreRAM(s)
	m.selected = s.Regs[0]
	copy(m.regs[:], s.Regs[1:5])
	m.mirrorBit = s.Regs[5]
	m.mirror = cartridge.Mirroring(s.Regs[6])
}
