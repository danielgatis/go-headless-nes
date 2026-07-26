package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// MMC1 (mapper 1, SxROM) is Nintendo's serial-interface ASIC: five
// write-only registers loaded one bit at a time through a shift
// register. It banks PRG in 16/32 KiB modes, CHR in 4/8 KiB modes,
// controls mirroring, and carries battery-backed PRG RAM.
//
// Not modeled: the SUROM 512 KiB PRG extension via CHR bank bit 4.
// The consecutive-cycle write filter is handled by the bus, which
// consults FiltersConsecutiveWrites for read-modify-write stores.
type MMC1 struct {
	base

	// ramAlwaysOn models mapper 155 (MMC1A), whose PRG-RAM disable bit
	// is not connected.
	ramAlwaysOn bool

	shift byte // bits collect here, LSB first
	count byte // writes since last reset/commit

	control byte // mirroring, PRG mode, CHR mode
	chr0    byte
	chr1    byte
	prgBank byte
}

// NewMMC1 wires an MMC1 board. Power-on fixes the last PRG bank at
// $C000 (PRG mode 3), which games rely on for their reset vectors.
func NewMMC1(c *cartridge.Cartridge) *MMC1 {
	return &MMC1{base: makeBase(c), control: 0x0C}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC1) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return m.prgWindow(addr)[addr&0x3FFF]
	case addr >= 0x6000:
		if m.ramDisabled() {
			return m.openBus()
		}
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ramDisabled reports the MMC1B PRG-RAM chip-enable, wired active-low
// to bit 4 of the PRG bank register.
func (m *MMC1) ramDisabled() bool { return !m.ramAlwaysOn && m.prgBank&0x10 != 0 }

// FiltersConsecutiveWrites reports whether the board drops the second of two consecutive writes.
func (m *MMC1) FiltersConsecutiveWrites() bool { return true }

// prgBankNum resolves the 16 KiB PRG bank number visible at addr under the
// current PRG mode. Boards that overlay an outer bank on the MMC1 (e.g.
// FaridSlrom) call this, transform the result, and read the ROM directly.
func (m *MMC1) prgBankNum(addr uint16) int {
	bank := int(m.prgBank & 0x0F)
	low := addr < 0xC000
	switch m.control >> 2 & 3 {
	case 0, 1: // 32 KiB mode: bit 0 of the bank number is ignored
		if low {
			return bank &^ 1
		}
		return bank | 1
	case 2: // first bank fixed at $8000
		if low {
			return 0
		}
		return bank
	default: // 3: last bank fixed at $C000
		if low {
			return bank
		}
		return -1
	}
}

// prgWindow resolves the 16 KiB bank visible at addr under the current
// PRG mode.
func (m *MMC1) prgWindow(addr uint16) []byte {
	return m.win(m.prg, m.prgBankNum(addr), 0x4000)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC1) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.loadShift(addr, v)
	case addr >= 0x6000:
		if !m.ramDisabled() {
			m.writePRGRAM(addr, v)
		}
	}
}

// loadShift feeds one bit into the serial port. Bit 7 of any write
// resets the shifter and re-fixes the last PRG bank, which is how
// games get to a known state.
func (m *MMC1) loadShift(addr uint16, v byte) {
	if v&0x80 != 0 {
		m.shift = 0
		m.count = 0
		m.control |= 0x0C
		return
	}
	m.shift |= (v & 1) << m.count
	m.count++
	if m.count < 5 {
		return
	}
	// Fifth bit: commit to the register selected by address bits 13-14.
	switch addr >> 13 & 3 {
	case 0:
		m.control = m.shift
	case 1:
		m.chr0 = m.shift
	case 2:
		m.chr1 = m.shift
	case 3:
		m.prgBank = m.shift
	}
	m.shift = 0
	m.count = 0
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC1) ReadCHR(addr uint16) byte {
	bank, size := m.chrWindow(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC1) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrWindow(addr)
	m.chrWrite(bank, size, addr, v)
}

// chrWindow resolves the CHR bank under the current CHR mode.
func (m *MMC1) chrWindow(addr uint16) (bank, size int) {
	if m.control&0x10 == 0 {
		// 8 KiB mode: bit 0 of the bank number is ignored.
		return int(m.chr0) >> 1, 8192
	}
	if addr < 0x1000 {
		return int(m.chr0), 4096
	}
	return int(m.chr1), 4096
}

// Mirroring reports the board's current nametable mirroring.
func (m *MMC1) Mirroring() cartridge.Mirroring {
	switch m.control & 3 {
	case 0:
		return cartridge.SingleLow
	case 1:
		return cartridge.SingleHigh
	case 2:
		return cartridge.Vertical
	default:
		return cartridge.Horizontal
	}
}

// Save writes the board's mapper-specific state into s.
func (m *MMC1) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.shift
	s.Regs[1] = m.count
	s.Regs[2] = m.control
	s.Regs[3] = m.chr0
	s.Regs[4] = m.chr1
	s.Regs[5] = m.prgBank
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC1) Restore(s *State) {
	m.restoreRAM(s)
	m.shift = s.Regs[0]
	m.count = s.Regs[1]
	m.control = s.Regs[2]
	m.chr0 = s.Regs[3]
	m.chr1 = s.Regs[4]
	m.prgBank = s.Regs[5]
}

// FaridSlrom (mapper 323): an MMC1 with a $6000-region outer-bank register
// (gated by WRAM-enable, lockable by bit 3) that overlays high PRG and CHR
// bank bits, giving a 512 KiB PRG / 512 KiB CHR multicart over the SLROM
// board. Ported from the reference emulator.
type FaridSlrom struct {
	MMC1

	outerBank byte
	locked    bool
}

// NewFaridSlrom wires the board.
func NewFaridSlrom(c *cartridge.Cartridge) *FaridSlrom {
	return &FaridSlrom{MMC1: *NewMMC1(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *FaridSlrom) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if !m.ramDisabled() && !m.locked {
			m.outerBank = (v & 0x70) >> 1
			m.locked = v&0x08 != 0
		}
		return
	}
	m.MMC1.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *FaridSlrom) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		bank := int(m.outerBank) | (m.prgBankNum(addr) & 0x07)
		return m.win(m.prg, bank, 0x4000)[addr&0x3FFF]
	}
	return m.MMC1.ReadPRG(addr)
}

// chr4K returns the 4 KiB CHR bank for a PPU half after the outer overlay.
func (m *FaridSlrom) chr4K(addr uint16) int {
	bank, size := m.chrWindow(addr)
	if size == 8192 {
		bank = bank<<1 | int(addr>>12&1)
	}
	return int(m.outerBank)<<2 | (bank & 0x1F)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *FaridSlrom) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chr4K(addr), 0x1000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *FaridSlrom) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chr4K(addr), 0x1000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *FaridSlrom) Save(s *State) {
	m.MMC1.Save(s)
	s.Regs[10] = m.outerBank
	s.Regs[11] = boolByte(m.locked)
}

// Restore loads the board's mapper-specific state from s.
func (m *FaridSlrom) Restore(s *State) {
	m.MMC1.Restore(s)
	m.outerBank = s.Regs[10]
	m.locked = s.Regs[11] != 0
}
