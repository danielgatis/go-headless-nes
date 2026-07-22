package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// TaitoTC0190 (mapper 33): two switchable 8 KiB PRG banks, two 2 KiB +
// four 1 KiB CHR banks and a mirroring bit in the PRG register.
type TaitoTC0190 struct {
	base

	prgBanks [2]byte
	chr2K    [2]byte
	chr1K    [4]byte
}

// NewTaitoTC0190 wires a Taito TC0190 board.
func NewTaitoTC0190(c *cartridge.Cartridge) *TaitoTC0190 {
	return &TaitoTC0190{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *TaitoTC0190) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		bank := -2
		if addr >= 0xE000 {
			bank = -1
		}
		return window(m.prg, bank, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBanks[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *TaitoTC0190) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0xA003 {
	case 0x8000:
		m.prgBanks[0] = v & 0x3F
		if v&0x40 != 0 {
			m.mirroring = cartridge.Horizontal
		} else {
			m.mirroring = cartridge.Vertical
		}
	case 0x8001:
		m.prgBanks[1] = v & 0x3F
	case 0x8002:
		m.chr2K[0] = v
	case 0x8003:
		m.chr2K[1] = v
	case 0xA000, 0xA001, 0xA002, 0xA003:
		m.chr1K[addr&0x03] = v
	}
}

func (m *TaitoTC0190) chrBank(addr uint16) (int, int) {
	if addr < 0x1000 {
		return int(m.chr2K[addr>>11]), 0x800
	}
	return int(m.chr1K[(addr-0x1000)>>10]), 0x400
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *TaitoTC0190) ReadCHR(addr uint16) byte {
	bank, size := m.chrBank(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *TaitoTC0190) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrBank(addr)
	m.chrWrite(bank, size, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *TaitoTC0190) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:2], m.prgBanks[:])
	copy(s.Regs[2:4], m.chr2K[:])
	copy(s.Regs[4:8], m.chr1K[:])
	s.Regs[8] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *TaitoTC0190) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:2])
	copy(m.chr2K[:], s.Regs[2:4])
	copy(m.chr1K[:], s.Regs[4:8])
	m.mirroring = cartridge.Mirroring(s.Regs[8])
}

// TaitoTC0690 (mapper 48): the TC0190 banking plus an MMC3-style
// scanline IRQ whose latch is written inverted and whose line rises
// ~22 CPU cycles later than the MMC3's (6 on submapper 1).
type TaitoTC0690 struct {
	TaitoTC0190

	sub1 bool

	irqLatch   byte
	irqCounter byte
	irqEnabled bool
	irqReload  bool
	irqDelay   byte
	irqLine    bool
}

// NewTaitoTC0690 wires a Taito TC0690 board.
func NewTaitoTC0690(c *cartridge.Cartridge) *TaitoTC0690 {
	return &TaitoTC0690{TaitoTC0190: TaitoTC0190{base: makeBase(c)}, sub1: c.Submapper == 1}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *TaitoTC0690) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0xE003 {
	case 0x8000:
		m.prgBanks[0] = v & 0x3F
	case 0x8001:
		m.prgBanks[1] = v & 0x3F
	case 0x8002:
		m.chr2K[0] = v
	case 0x8003:
		m.chr2K[1] = v
	case 0xA000, 0xA001, 0xA002, 0xA003:
		m.chr1K[addr&0x03] = v
	case 0xC000:
		m.irqLine = false
		m.irqLatch = ^v
		if m.sub1 {
			m.irqLatch++
		}
	case 0xC001:
		m.irqLine = false
		m.irqCounter = 0
		m.irqReload = true
	case 0xC002:
		m.irqEnabled = true
	case 0xC003:
		m.irqEnabled = false
		m.irqLine = false
	case 0xE000:
		if v&0x40 != 0 {
			m.mirroring = cartridge.Horizontal
		} else {
			m.mirroring = cartridge.Vertical
		}
	}
}

// Scanline clocks the MMC3-style counter; a zero result schedules the
// delayed IRQ.
func (m *TaitoTC0690) Scanline() {
	if m.irqCounter == 0 || m.irqReload {
		m.irqCounter = m.irqLatch
		m.irqReload = false
	} else {
		m.irqCounter--
	}
	if m.irqCounter == 0 && m.irqEnabled {
		if m.sub1 {
			m.irqDelay = 6
		} else {
			m.irqDelay = 22
		}
	}
}

// Tick advances the board by one cycle.
func (m *TaitoTC0690) Tick() {
	if m.irqDelay > 0 {
		m.irqDelay--
		if m.irqDelay == 0 {
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *TaitoTC0690) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *TaitoTC0690) Save(s *State) {
	m.TaitoTC0190.Save(s)
	s.Regs[9] = m.irqLatch
	s.Regs[10] = m.irqCounter
	s.Regs[11] = boolByte(m.irqEnabled) | boolByte(m.irqReload)<<1 | boolByte(m.irqLine)<<2
	s.Regs[12] = m.irqDelay
}

// Restore loads the board's mapper-specific state from s.
func (m *TaitoTC0690) Restore(s *State) {
	m.TaitoTC0190.Restore(s)
	m.irqLatch = s.Regs[9]
	m.irqCounter = s.Regs[10]
	m.irqEnabled = s.Regs[11]&1 != 0
	m.irqReload = s.Regs[11]&2 != 0
	m.irqLine = s.Regs[11]&4 != 0
	m.irqDelay = s.Regs[12]
}

// TaitoX1005 (mappers 80 and 207): three 8 KiB PRG banks, two 2 KiB +
// four 1 KiB CHR banks, a write-$A3-to-unlock 128-byte internal RAM at
// $7F00, and either register mirroring (80) or per-nametable selection
// from the CHR registers' top bit (207).
type TaitoX1005 struct {
	base

	altMirroring bool // mapper 207

	prgBanks [3]byte
	chr2K    [2]byte
	chr1K    [4]byte
	ramPerm  byte
	ntPages  [4]byte
	ram      [128]byte
}

// NewTaitoX1005 wires a Taito X1-005 board.
func NewTaitoX1005(c *cartridge.Cartridge) *TaitoX1005 {
	return &TaitoX1005{base: makeBase(c), altMirroring: c.MapperID == 207}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *TaitoX1005) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBanks[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x7F00:
		if m.ramPerm == 0xA3 {
			return m.ram[addr&0x7F]
		}
		return m.openBus()
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *TaitoX1005) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		// No registers above $8000 on this board.
	case addr >= 0x7F00:
		if m.ramPerm == 0xA3 {
			m.ram[addr&0x7F] = v
		}
	case addr >= 0x7EF0 && addr <= 0x7EFF:
		m.writeRegister(addr, v)
	}
}

func (m *TaitoX1005) writeRegister(addr uint16, v byte) {
	switch addr {
	case 0x7EF0:
		m.chr2K[0] = v
		if m.altMirroring {
			m.ntPages[0] = v >> 7
			m.ntPages[1] = v >> 7
		}
	case 0x7EF1:
		m.chr2K[1] = v
		if m.altMirroring {
			m.ntPages[2] = v >> 7
			m.ntPages[3] = v >> 7
		}
	case 0x7EF2:
		m.chr1K[0] = v
	case 0x7EF3:
		m.chr1K[1] = v
	case 0x7EF4:
		m.chr1K[2] = v
	case 0x7EF5:
		m.chr1K[3] = v
	case 0x7EF6, 0x7EF7:
		if !m.altMirroring {
			if v&0x01 != 0 {
				m.mirroring = cartridge.Vertical
			} else {
				m.mirroring = cartridge.Horizontal
			}
		}
	case 0x7EF8, 0x7EF9:
		m.ramPerm = v
	case 0x7EFA, 0x7EFB:
		m.prgBanks[0] = v
	case 0x7EFC, 0x7EFD:
		m.prgBanks[1] = v
	case 0x7EFE, 0x7EFF:
		m.prgBanks[2] = v
	}
}

// NametablePage implements mapper 207's CHR-register-driven mirroring.
func (m *TaitoX1005) NametablePage(table byte) byte {
	if !m.altMirroring {
		// Fall back to the plain mirroring modes.
		switch m.mirroring {
		case cartridge.Horizontal:
			return table >> 1
		case cartridge.Vertical:
			return table & 1
		default:
			return 0
		}
	}
	return m.ntPages[table&3]
}

func (m *TaitoX1005) chrBank(addr uint16) (int, int) {
	if addr < 0x1000 {
		// The 2 KiB registers hold a 1 KiB page number; the window is the
		// pair (page, page+1), which need not be 2 KiB aligned.
		return int(m.chr2K[addr>>11]) + int((addr>>10)&1), 0x400
	}
	return int(m.chr1K[(addr-0x1000)>>10]), 0x400
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *TaitoX1005) ReadCHR(addr uint16) byte {
	bank, size := m.chrBank(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *TaitoX1005) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrBank(addr)
	m.chrWrite(bank, size, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *TaitoX1005) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:3], m.prgBanks[:])
	copy(s.Regs[3:5], m.chr2K[:])
	copy(s.Regs[5:9], m.chr1K[:])
	s.Regs[9] = m.ramPerm
	copy(s.Regs[10:14], m.ntPages[:])
	s.Regs[14] = byte(m.mirroring)
	// The 128-byte internal RAM shares the PRGRAM snapshot space.
	copy(s.PRGRAM[8192-128:8192], m.ram[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *TaitoX1005) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:3])
	copy(m.chr2K[:], s.Regs[3:5])
	copy(m.chr1K[:], s.Regs[5:9])
	m.ramPerm = s.Regs[9]
	copy(m.ntPages[:], s.Regs[10:14])
	m.mirroring = cartridge.Mirroring(s.Regs[14])
	copy(m.ram[:], s.PRGRAM[8192-128:8192])
}

// TaitoX1017 (mapper 82): three 8 KiB PRG banks with reversed bank bits
// on some boards, a CHR mode bit that swaps the 2 KiB/1 KiB halves, and
// three magic-value-protected save RAM sections at $6000-$73FF.
type TaitoX1017 struct {
	base

	prgBanks [3]byte
	chrMode  byte
	chrRegs  [6]byte
	ramPerm  [3]byte
	scramble bool // mapper 552: the PRG bank data bus is scrambled
}

// NewTaitoX1017 wires a Taito X1-017 board. Mapper 552 is the same board
// with a scrambled PRG-bank data bus (82 uses value>>2).
func NewTaitoX1017(c *cartridge.Cartridge) *TaitoX1017 {
	return &TaitoX1017{base: makeBase(c), scramble: c.MapperID == 552}
}

// ramSectionOpen reports whether the protected section holding addr is
// unlocked ($6000-$67FF, $6800-$6FFF, $7000-$73FF).
func (m *TaitoX1017) ramSectionOpen(addr uint16) bool {
	switch {
	case addr < 0x6800:
		return m.ramPerm[0] == 0xCA
	case addr < 0x7000:
		return m.ramPerm[1] == 0x69
	case addr < 0x7400:
		return m.ramPerm[2] == 0x84
	}
	return false
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *TaitoX1017) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBanks[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000 && addr < 0x7400:
		if m.ramSectionOpen(addr) {
			return m.readPRGRAM(addr)
		}
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *TaitoX1017) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x7EF0 && addr <= 0x7EFC:
		m.writeRegister(addr, v)
	case addr >= 0x6000 && addr < 0x7400:
		if m.ramSectionOpen(addr) {
			m.writePRGRAM(addr, v)
		}
	}
}

func (m *TaitoX1017) writeRegister(addr uint16, v byte) {
	switch addr {
	case 0x7EF0, 0x7EF1, 0x7EF2, 0x7EF3, 0x7EF4, 0x7EF5:
		m.chrRegs[addr&0x0F] = v
	case 0x7EF6:
		if v&0x01 != 0 {
			m.mirroring = cartridge.Vertical
		} else {
			m.mirroring = cartridge.Horizontal
		}
		m.chrMode = (v & 0x02) >> 1
	case 0x7EF7, 0x7EF8, 0x7EF9:
		m.ramPerm[(addr&0x0F)-7] = v
	case 0x7EFA, 0x7EFB, 0x7EFC:
		if m.scramble {
			m.prgBanks[addr-0x7EFA] = (v&0x20)>>5 | (v&0x10)>>3 | (v&0x08)>>1 |
				(v&0x04)<<1 | (v&0x02)<<3 | (v&0x01)<<5
		} else {
			m.prgBanks[addr-0x7EFA] = v >> 2
		}
	}
}

func (m *TaitoX1017) chrBank(addr uint16) (int, int) {
	slot := addr >> 10 // 1 KiB slot 0-7
	if m.chrMode == 0 {
		if slot < 4 {
			// Two 2 KiB windows from regs 0-1 (LSB ignored).
			return int(m.chrRegs[slot>>1]&0xFE) >> 1, 0x800
		}
		return int(m.chrRegs[2+(slot-4)]), 0x400
	}
	if slot < 4 {
		return int(m.chrRegs[2+slot]), 0x400
	}
	return int(m.chrRegs[(slot-4)>>1]&0xFE) >> 1, 0x800
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *TaitoX1017) ReadCHR(addr uint16) byte {
	bank, size := m.chrBank(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *TaitoX1017) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrBank(addr)
	m.chrWrite(bank, size, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *TaitoX1017) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:3], m.prgBanks[:])
	s.Regs[3] = m.chrMode
	copy(s.Regs[4:10], m.chrRegs[:])
	copy(s.Regs[10:13], m.ramPerm[:])
	s.Regs[13] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *TaitoX1017) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:3])
	m.chrMode = s.Regs[3]
	copy(m.chrRegs[:], s.Regs[4:10])
	copy(m.ramPerm[:], s.Regs[10:13])
	m.mirroring = cartridge.Mirroring(s.Regs[13])
}
