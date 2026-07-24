package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// MMC2 (mapper 9, Punch-Out!!) and MMC4 (mapper 10) switch CHR banks
// automatically mid-frame: each 4 KiB pattern window has two bank
// registers, and reading specific tile addresses ($FD8/$FE8 rows)
// flips a latch selecting between them, that is how Punch-Out! shows
// giant opponents with tiny sprite budgets. They differ in PRG layout
// and in the latch address ranges for the low window.
type MMC2 struct {
	base

	mmc4 bool // MMC4: 16 KiB PRG banking and range-based low-window latch

	prgReg byte
	// chrBank[window][latchState]: bank when the latch reads FD / FE.
	chrBank          [2][2]byte
	latch            [2]byte // 0 = FD selected, 1 = FE selected
	mirrorHorizontal bool
}

// NewMMC2 wires an MMC2 (Punch-Out!!) board.
func NewMMC2(c *cartridge.Cartridge) *MMC2 {
	return &MMC2{base: makeBase(c)}
}

// NewMMC4 wires an MMC4 board (Fire Emblem, Famicom Wars).
func NewMMC4(c *cartridge.Cartridge) *MMC2 {
	return &MMC2{base: makeBase(c), mmc4: true}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC2) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		if m.mmc4 {
			// 16 KiB switchable + fixed last bank.
			if addr < 0xC000 {
				return window(m.prg, int(m.prgReg&0x0F), 0x4000)[addr&0x3FFF]
			}
			return window(m.prg, -1, 0x4000)[addr&0x3FFF]
		}
		// MMC2: 8 KiB switchable, last three banks fixed.
		if addr < 0xA000 {
			return window(m.prg, int(m.prgReg&0x0F), 0x2000)[addr&0x1FFF]
		}
		bank := -3 + int(addr-0xA000)>>13
		return window(m.prg, bank, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC2) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0xA000:
		m.writeRegister(addr, v)
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

func (m *MMC2) writeRegister(addr uint16, v byte) {
	switch addr >> 12 {
	case 0xA:
		m.prgReg = v
	case 0xB:
		m.chrBank[0][0] = v & 0x1F // low window, FD
	case 0xC:
		m.chrBank[0][1] = v & 0x1F // low window, FE
	case 0xD:
		m.chrBank[1][0] = v & 0x1F // high window, FD
	case 0xE:
		m.chrBank[1][1] = v & 0x1F // high window, FE
	case 0xF:
		m.mirrorHorizontal = v&1 != 0
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC2) ReadCHR(addr uint16) byte {
	win := int(addr >> 12)
	v := m.chrRead(int(m.chrBank[win][m.latch[win]]), 4096, addr)
	m.updateLatch(addr)
	return v
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC2) WriteCHR(addr uint16, v byte) {
	win := int(addr >> 12)
	m.chrWrite(int(m.chrBank[win][m.latch[win]]), 4096, addr, v)
	m.updateLatch(addr)
}

// updateLatch flips the window's latch after an access to the trigger
// tiles. MMC2's low window triggers on exactly $0FD8/$0FE8; everything
// else (MMC2 high window, both MMC4 windows) on the 8-byte rows
// $xFD8-$xFDF / $xFE8-$xFEF.
func (m *MMC2) updateLatch(addr uint16) {
	win := addr >> 12
	if win == 0 && !m.mmc4 {
		switch addr {
		case 0x0FD8:
			m.latch[0] = 0
		case 0x0FE8:
			m.latch[0] = 1
		}
		return
	}
	switch addr &^ 0x0007 {
	case win<<12 | 0x0FD8:
		m.latch[win] = 0
	case win<<12 | 0x0FE8:
		m.latch[win] = 1
	}
}

// Mirroring reports the board's current nametable mirroring.
func (m *MMC2) Mirroring() cartridge.Mirroring {
	if m.mirrorHorizontal {
		return cartridge.Horizontal
	}
	return cartridge.Vertical
}

// Save writes the board's mapper-specific state into s.
func (m *MMC2) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg
	s.Regs[1] = m.chrBank[0][0]
	s.Regs[2] = m.chrBank[0][1]
	s.Regs[3] = m.chrBank[1][0]
	s.Regs[4] = m.chrBank[1][1]
	s.Regs[5] = m.latch[0]
	s.Regs[6] = m.latch[1]
	s.Regs[7] = boolByte(m.mirrorHorizontal)
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC2) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg = s.Regs[0]
	m.chrBank[0][0] = s.Regs[1]
	m.chrBank[0][1] = s.Regs[2]
	m.chrBank[1][0] = s.Regs[3]
	m.chrBank[1][1] = s.Regs[4]
	m.latch[0] = s.Regs[5]
	m.latch[1] = s.Regs[6]
	m.mirrorHorizontal = s.Regs[7] != 0
}
