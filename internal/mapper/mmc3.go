package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// MMC3 (mapper 4, TxROM) banks PRG in 8 KiB and CHR in 1/2 KiB
// windows through an index/data register pair, controls mirroring, and
// carries the scanline IRQ counter that raster effects in Super Mario
// Bros. 3 and many others depend on. The counter is clocked by PPU
// address line 12 rising once per rendered line, which reaches us
// through the Scanline hook.
//
// This implements the normal MMC3B IRQ behaviour: an IRQ asserts
// whenever the counter is zero after being clocked (so a zero latch IRQs
// every line).
type MMC3 struct {
	base

	bankSelect byte    // target register, PRG mode (bit 6), CHR mode (bit 7)
	banks      [8]byte // R0-R5 CHR, R6-R7 PRG

	mirrorHorizontal bool
	fourScreen       bool // solder-pad override: register is ignored

	ramEnabled      bool
	ramWriteProtect bool

	irqLatch    byte
	irqCounter  byte
	irqEnabled  bool
	irqReload   bool
	irqAsserted bool
}

// NewMMC3 wires a TxROM board.
func NewMMC3(c *cartridge.Cartridge) *MMC3 {
	return &MMC3{
		base:       makeBase(c),
		fourScreen: c.Mirroring == cartridge.FourScreen,
		ramEnabled: true,
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return window(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if !m.ramEnabled {
			return m.openBus()
		}
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// prgBank maps a CPU address to its 8 KiB PRG bank. Bit 6 of the bank
// select swaps which of $8000/$C000 is switchable and which is fixed
// to the second-to-last bank.
func (m *MMC3) prgBank(addr uint16) int {
	swap := m.bankSelect&0x40 != 0
	switch addr >> 13 & 3 { // 8 KiB window index
	case 0: // $8000
		if swap {
			return -2
		}
		return int(m.banks[6])
	case 1: // $A000
		return int(m.banks[7])
	case 2: // $C000
		if swap {
			return int(m.banks[6])
		}
		return -2
	default: // $E000
		return -1
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.writeRegister(addr, v)
	case addr >= 0x6000:
		if m.ramEnabled && !m.ramWriteProtect {
			m.writePRGRAM(addr, v)
		}
	}
}

// writeRegister decodes the four register pairs at $8000/$A000/$C000/
// $E000, even/odd.
func (m *MMC3) writeRegister(addr uint16, v byte) {
	odd := addr&1 != 0
	switch addr >> 13 & 3 {
	case 0: // $8000 bank select / $8001 bank data
		if odd {
			m.banks[m.bankSelect&7] = v
		} else {
			m.bankSelect = v
		}
	case 1: // $A000 mirroring / $A001 PRG RAM protect
		if odd {
			m.ramEnabled = v&0x80 != 0
			m.ramWriteProtect = v&0x40 != 0
		} else {
			m.mirrorHorizontal = v&1 != 0
		}
	case 2: // $C000 IRQ latch / $C001 IRQ reload
		if odd {
			m.irqCounter = 0
			m.irqReload = true
		} else {
			m.irqLatch = v
		}
	case 3: // $E000 IRQ disable+ack / $E001 IRQ enable
		if odd {
			m.irqEnabled = true
		} else {
			m.irqEnabled = false
			m.irqAsserted = false
		}
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3) ReadCHR(addr uint16) byte {
	bank, size := m.chrWindow(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrWindow(addr)
	m.chrWrite(bank, size, addr, v)
}

// chrPage1K resolves the raw 1 KiB CHR page for addr, keeping every
// register bit intact, the granularity multicart outer banks and
// TQROM's RAM-select bit operate on.
func (m *MMC3) chrPage1K(addr uint16) int {
	if m.bankSelect&0x80 != 0 {
		addr ^= 0x1000
	}
	if addr < 0x1000 {
		r := m.banks[addr>>11&1]
		return int(r&^1) + int((addr>>10)&1)
	}
	return int(m.banks[2+(addr>>10)&3])
}

// chrWindow maps a PPU address to its CHR bank: two 2 KiB windows
// (R0, R1) and four 1 KiB windows (R2-R5), with bit 7 of the bank
// select swapping the two halves of the pattern table space.
func (m *MMC3) chrWindow(addr uint16) (bank, size int) {
	if m.bankSelect&0x80 != 0 {
		addr ^= 0x1000
	}
	if addr < 0x1000 {
		// R0/R1 are 2 KiB banks counted in 1 KiB units, low bit ignored.
		r := addr >> 11 & 1
		return int(m.banks[r]&^1) >> 1, 2048
	}
	r := 2 + (addr>>10)&3
	return int(m.banks[r]), 1024
}

// Mirroring reports the board's current nametable mirroring.
func (m *MMC3) Mirroring() cartridge.Mirroring {
	switch {
	case m.fourScreen:
		return cartridge.FourScreen
	case m.mirrorHorizontal:
		return cartridge.Horizontal
	default:
		return cartridge.Vertical
	}
}

// Scanline clocks the IRQ counter on a filtered A12 rising edge: reload
// when zero or when a reload was requested, decrement otherwise; assert
// when the result is zero and IRQs are on.
//
// This is the normal MMC3B behaviour, where a zero latch re-asserts on
// every clock (mmc3_test's 5-MMC3). The alternate revision (6-MMC6 /
// 6-MMC3_alt), which suppresses the IRQ on a natural reload after the
// counter already hit zero, is a different silicon variant and is
// mutually exclusive with 5-MMC3, no single implementation passes both,
// so those two sub-tests fail by design. The normal behaviour is what
// the games this emulator targets (SMB3 and friends) depend on.
func (m *MMC3) Scanline() {
	if m.irqCounter == 0 || m.irqReload {
		m.irqCounter = m.irqLatch
		m.irqReload = false
	} else {
		m.irqCounter--
	}
	if m.irqCounter == 0 && m.irqEnabled {
		m.irqAsserted = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *MMC3) IRQ() bool { return m.irqAsserted }

// Save writes the board's mapper-specific state into s.
func (m *MMC3) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bankSelect
	copy(s.Regs[1:9], m.banks[:])
	s.Regs[9] = boolByte(m.mirrorHorizontal)
	s.Regs[10] = boolByte(m.ramEnabled)
	s.Regs[11] = boolByte(m.ramWriteProtect)
	s.Regs[12] = m.irqLatch
	s.Regs[13] = m.irqCounter
	s.Regs[14] = boolByte(m.irqEnabled)
	s.Regs[15] = boolByte(m.irqReload)
	s.Regs[16] = boolByte(m.irqAsserted)
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3) Restore(s *State) {
	m.restoreRAM(s)
	m.bankSelect = s.Regs[0]
	copy(m.banks[:], s.Regs[1:9])
	m.mirrorHorizontal = s.Regs[9] != 0
	m.ramEnabled = s.Regs[10] != 0
	m.ramWriteProtect = s.Regs[11] != 0
	m.irqLatch = s.Regs[12]
	m.irqCounter = s.Regs[13]
	m.irqEnabled = s.Regs[14] != 0
	m.irqReload = s.Regs[15] != 0
	m.irqAsserted = s.Regs[16] != 0
}
