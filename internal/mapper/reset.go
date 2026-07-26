package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Boards whose bank mode advances on the console RESET line. They
// implement Reset(soft bool), which the console calls; a power cycle
// (soft=false) clears the counter, a soft reset advances it.

// Mapper60 (mapper 60, reset-based 4-in-1): each soft reset advances a
// 2-bit counter that selects the 16 KiB PRG bank (mirrored) and the 8 KiB
// CHR bank.
type Mapper60 struct {
	base
	sel byte
}

// NewMapper60 wires the board.
func NewMapper60(c *cartridge.Cartridge) *Mapper60 {
	return &Mapper60{base: makeBase(c)}
}

// Reset returns the board to its power-on banking.
func (m *Mapper60) Reset(soft bool) {
	if soft {
		m.sel = (m.sel + 1) & 0x03
	} else {
		m.sel = 0
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper60) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper60) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, int(m.sel), 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper60) ReadCHR(addr uint16) byte { return m.chrRead(int(m.sel), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper60) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.sel), 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper60) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.sel }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper60) Restore(s *State) { m.restoreRAM(s); m.sel = s.Regs[0] }

// Mapper230 (mapper 230, "22-in-1"/Contra): the reset line toggles between
// a fixed Contra layout and a menu that banks PRG from the written value.
type Mapper230 struct {
	base
	contraMode bool
	prg0       int
	prg1       int
	mirror     cartridge.Mirroring
}

// NewMapper230 wires the board.
func NewMapper230(c *cartridge.Cartridge) *Mapper230 {
	m := &Mapper230{base: makeBase(c), mirror: c.Mirroring}
	m.applyMode()
	return m
}

// Reset returns the board to its power-on banking.
func (m *Mapper230) Reset(soft bool) {
	if soft {
		m.contraMode = !m.contraMode
		m.applyMode()
	}
}

func (m *Mapper230) applyMode() {
	if m.contraMode {
		m.prg0, m.prg1 = 0, 7
		m.mirror = cartridge.Vertical
	} else {
		m.prg0, m.prg1 = 8, 9
		m.mirror = cartridge.Horizontal
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper230) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	if m.contraMode {
		m.prg0 = int(v & 0x07)
		return
	}
	if v&0x20 != 0 {
		b := int(v&0x1F) + 8
		m.prg0, m.prg1 = b, b
	} else {
		b := int(v&0x1E) + 8
		m.prg0, m.prg1 = b, b+1
	}
	m.mirror = hvMirror(v&0x40 != 0)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper230) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		if m.contraMode {
			return m.win(m.prg, 7, 0x4000)[addr&0x3FFF]
		}
		return m.win(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper230) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper230) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper230) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Mapper230) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = boolByte(m.contraMode)
	s.Regs[1] = byte(m.prg0)
	s.Regs[2] = byte(m.prg1)
	s.Regs[3] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper230) Restore(s *State) {
	m.restoreRAM(s)
	m.contraMode = s.Regs[0] != 0
	m.prg0 = int(s.Regs[1])
	m.prg1 = int(s.Regs[2])
	m.mirror = cartridge.Mirroring(s.Regs[3])
}

// Mapper233 (mapper 233): the Mapper226 board with a reset bit added to
// the PRG page, so each soft reset selects a different half.
type Mapper233 struct {
	Mapper226
	reset byte
}

// NewMapper233 wires the board.
func NewMapper233(c *cartridge.Cartridge) *Mapper233 {
	m := &Mapper233{Mapper226: *NewMapper226(c)}
	m.update233()
	return m
}

// Reset returns the board to its power-on banking.
func (m *Mapper233) Reset(soft bool) {
	if soft {
		m.reset ^= 0x01
	} else {
		m.reset = 0
	}
	m.update233()
}

// prgPage233 mirrors Mapper226's page but inserts the reset bit at bit 5.
func (m *Mapper233) prgPage233() int {
	return int(m.reg[0]&0x1F) | int(m.reset)<<5 | int(m.reg[1]&0x01)<<6
}

func (m *Mapper233) update233() {
	p := m.prgPage233()
	if m.reg[0]&0x20 != 0 {
		m.prg0, m.prg1 = p, p
	} else {
		m.prg0, m.prg1 = p&0xFE, p&0xFE|1
	}
	if m.reg[0]&0x40 != 0 {
		m.mirror = cartridge.Vertical
	} else {
		m.mirror = cartridge.Horizontal
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper233) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0x8001 {
	case 0x8000:
		m.reg[0] = v
	case 0x8001:
		m.reg[1] = v
	}
	m.update233()
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper233) Save(s *State) {
	m.Mapper226.Save(s)
	s.Regs[9] = m.reset
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper233) Restore(s *State) {
	m.Mapper226.Restore(s)
	m.reset = s.Regs[9]
}

// ResetTxrom (mapper 313): an MMC3 whose reset counter (advanced by soft
// reset) supplies the top PRG and CHR bank bits, cycling through four
// games.
type ResetTxrom struct {
	MMC3
	resetCounter byte
}

// NewResetTxrom wires the board.
func NewResetTxrom(c *cartridge.Cartridge) *ResetTxrom {
	return &ResetTxrom{MMC3: *NewMMC3(c)}
}

// Reset returns the board to its power-on banking.
func (m *ResetTxrom) Reset(soft bool) {
	if soft {
		m.resetCounter = (m.resetCounter + 1) & 0x03
	} else {
		m.resetCounter = 0
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *ResetTxrom) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	page := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if page < 0 {
		page += n
	}
	page = page&0x0F | int(m.resetCounter)<<4
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *ResetTxrom) chrPage(addr uint16) int {
	return m.chrPage1K(addr)&0x7F | int(m.resetCounter)<<7
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *ResetTxrom) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *ResetTxrom) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *ResetTxrom) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.resetCounter }

// Restore loads the board's mapper-specific state from s.
func (m *ResetTxrom) Restore(s *State) { m.MMC3.Restore(s); m.resetCounter = s.Regs[17] }
