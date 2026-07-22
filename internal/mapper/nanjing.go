package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Nanjing (mapper 163): the Nanjing FC-001 pirate board used by a large
// family of Chinese RPGs (Final Fantasy VII, Zelda ports and friends).
// One 32 KiB PRG window is banked from write-only registers at
// $5000-$5FFF, which double as a copy-protection challenge the game reads
// back. CHR is 8 KiB of RAM in two 4 KiB windows the board can swap by
// itself mid-frame: with the automatic switch enabled the hardware
// watches the raster position and flips both windows at lines 127 and
// 239, which is how these games show a split status bar without a
// scanline IRQ. The board carries battery PRG RAM at $6000.
type Nanjing struct {
	base

	regs       [5]byte
	prgPage    byte // current 32 KiB PRG bank
	chrBanks   [2]byte
	toggle     bool // protection flip-flop read back through $5500
	autoSwitch bool // hardware CHR switch at lines 127/239

	ppuPos func() (scanline, dot int)
}

// NewNanjing wires the board.
func NewNanjing(c *cartridge.Cartridge) *Nanjing {
	m := &Nanjing{
		base: makeBase(c),
		// The protection register powers up at 1 and the trigger at 0.
		toggle: true,
		ppuPos: func() (int, int) { return 0, 0 },
	}
	return m
}

// SetPPUPos installs the raster-position accessor (console wiring).
func (m *Nanjing) SetPPUPos(pos func() (scanline, dot int)) { m.ppuPos = pos }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Nanjing) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return window(m.prg, int(m.prgPage), 0x8000)[addr&0x7FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	case addr >= 0x5000:
		// Copy-protection reads (per FCEUX's reverse engineering).
		switch addr & 0x7700 {
		case 0x5100:
			return m.regs[3] | m.regs[1] | m.regs[0] | (m.regs[2] ^ 0xFF)
		case 0x5500:
			if m.toggle {
				return m.regs[3] | m.regs[0]
			}
			return 0
		}
		return 4
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space.
func (m *Nanjing) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	case addr >= 0x5000:
		m.writeRegister(addr, v)
	}
}

// writeRegister decodes the $5000-$5FFF register file. Addresses are
// masked with $7300, except $5101 (the trigger) which is fully decoded.
func (m *Nanjing) writeRegister(addr uint16, v byte) {
	if addr == 0x5101 {
		// The trigger toggles when this register falls from nonzero to zero.
		if m.regs[4] != 0 && v == 0 {
			m.toggle = !m.toggle
		}
		m.regs[4] = v
		return
	}
	if addr == 0x5100 && v == 6 {
		m.prgPage = 3
		return
	}
	switch addr & 0x7300 {
	case 0x5000:
		m.regs[0] = v
		if v&0x80 == 0 {
			// Disabling the auto switch in the top half of the frame
			// resets the windows to the split the game expects.
			if scanline, _ := m.ppuPos(); scanline < 128 {
				m.chrBanks[0], m.chrBanks[1] = 0, 1
			}
		}
		m.updateState()
	case 0x5100:
		m.regs[1] = v
		if v == 6 {
			m.prgPage = 3
		}
	case 0x5200:
		m.regs[2] = v
		m.updateState()
	case 0x5300:
		m.regs[3] = v
	}
}

// updateState derives the PRG bank and auto-switch flag from the
// registers.
func (m *Nanjing) updateState() {
	m.prgPage = (m.regs[0] & 0x0F) | (m.regs[2]&0x0F)<<4
	m.autoSwitch = m.regs[0]&0x80 != 0
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Nanjing) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>12&1]), 0x1000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Nanjing) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>12&1]), 0x1000, addr, v)
}

// NotifyVramAddr implements the automatic CHR switch: with it enabled the
// board flips both 4 KiB windows to bank 1 at the end of line 127 and
// back to bank 0 at the end of line 239.
func (m *Nanjing) NotifyVramAddr(_ uint16) {
	if !m.autoSwitch {
		return
	}
	scanline, dot := m.ppuPos()
	if dot <= 256 {
		return
	}
	switch scanline {
	case 239:
		m.chrBanks[0], m.chrBanks[1] = 0, 0
	case 127:
		m.chrBanks[0], m.chrBanks[1] = 1, 1
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Nanjing) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:5], m.regs[:])
	s.Regs[5] = m.prgPage
	s.Regs[6] = m.chrBanks[0]
	s.Regs[7] = m.chrBanks[1]
	s.Regs[8] = boolByte(m.toggle)
	s.Regs[9] = boolByte(m.autoSwitch)
}

// Restore loads the board's mapper-specific state from s.
func (m *Nanjing) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.regs[:], s.Regs[0:5])
	m.prgPage = s.Regs[5]
	m.chrBanks[0] = s.Regs[6]
	m.chrBanks[1] = s.Regs[7]
	m.toggle = s.Regs[8] != 0
	m.autoSwitch = s.Regs[9] != 0
}
