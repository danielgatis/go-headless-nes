package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Fk23C (mapper 176): the FK23C/FK23CA multicart ASIC, an MMC3 core
// wrapped in outer-bank registers at $5010-$5FFF that carve the (often
// multi-megabyte) ROM into sub-cartridges. Besides the MMC3 modes it has
// whole-window 16/32 KiB PRG modes, a CNROM-style latched CHR mode, an
// extended MMC3 mode with twelve bank registers, up to 32 KiB of work
// RAM (bankable at both $6000 and $4020-$5FFF) and a CHR-RAM option.
// The IRQ is the MMC3 scanline counter with a two-cycle assert delay.
type Fk23C struct {
	base

	// $5010-$5013 outer registers.
	prgMode       byte
	outerChrSmall bool // halve the inner CHR mask
	selectChrRAM  bool
	mmc3ChrMode   bool
	cnromChrMode  bool
	prgBase       uint16
	chrBase       byte
	extendedMmc3  bool

	// $A001 configuration.
	wramBank          byte
	ramInFirstChrBank bool
	allowSingleScreen bool
	fk23RegsEnabled   bool
	wramConfigEnabled bool
	wramEnabled       bool
	wramWriteProtect  bool

	// MMC3 core.
	invertPrgA14 bool
	invertChrA12 bool
	curReg       byte
	mmc3Regs     [12]byte
	mirrorReg    byte
	cnromChrReg  byte

	irqLatch    byte
	irqCounter  byte
	irqReload   bool
	irqEnabled  bool
	irqAsserted bool
	irqDelay    byte

	hasBattery bool

	wram   [32768]byte
	bigCHR [262144]byte // up to 256 KiB of CHR RAM on some carts
}

// NewFk23C wires the board.
func NewFk23C(c *cartridge.Cartridge) *Fk23C {
	m := &Fk23C{
		base:        makeBase(c),
		mmc3ChrMode: true,
		hasBattery:  c.HasBattery,
		mmc3Regs:    [12]byte{0, 2, 4, 5, 6, 7, 0, 1, 0xFE, 0xFF, 0xFF, 0xFF},
	}
	// Subtype 1 (1 MiB PRG + 1 MiB CHR) boots in the second 512 KiB.
	if len(c.PRG) == 1024*1024 && len(c.CHR) == 1024*1024 {
		m.prgBase = 0x20
	}
	return m
}

// prgBank resolves the 8 KiB PRG bank for a CPU window.
func (m *Fk23C) prgBank(addr uint16) int {
	slot := int(addr>>13) & 3
	mode := m.prgMode
	if mode > 4 {
		// Modes 5-7 are undocumented; the reference leaves the previous
		// mapping in place, which in practice means the MMC3 layout.
		mode = 0
	}
	switch mode {
	case 3: // 16 KiB whole-window mode
		return int(m.prgBase)<<1 + (slot & 1)
	case 4: // 32 KiB whole-window mode
		return int(m.prgBase&0xFFE)<<1 + slot
	default: // MMC3 modes 0-2, with widening inner masks
		swap := 0
		if m.invertPrgA14 {
			swap = 2
		}
		var r [4]int
		if m.extendedMmc3 {
			outer := int(m.prgBase) << 1
			r[0^swap] = int(m.mmc3Regs[6]) | outer
			r[1] = int(m.mmc3Regs[7]) | outer
			r[2^swap] = int(m.mmc3Regs[8]) | outer
			r[3] = int(m.mmc3Regs[9]) | outer
		} else {
			innerMask := 0x3F >> mode
			outer := (int(m.prgBase) << 1) &^ innerMask
			r[0^swap] = int(m.mmc3Regs[6])&innerMask | outer
			r[1] = int(m.mmc3Regs[7])&innerMask | outer
			r[2^swap] = 0xFE&innerMask | outer
			r[3] = 0xFF&innerMask | outer
		}
		return r[slot]
	}
}

// chrPage resolves the 1 KiB CHR page for a PPU address.
func (m *Fk23C) chrPage(addr uint16) int {
	slot := int(addr>>10) & 7
	if !m.mmc3ChrMode {
		inner := 0
		if m.cnromChrMode {
			if m.outerChrSmall {
				inner = 1
			} else {
				inner = 3
			}
		}
		return (int(m.cnromChrReg)&inner|int(m.chrBase))<<3 + slot
	}
	if m.invertChrA12 {
		slot ^= 4
	}
	if m.extendedMmc3 {
		regIdx := [8]int{0, 10, 1, 11, 2, 3, 4, 5}
		return int(m.mmc3Regs[regIdx[slot]]) | int(m.chrBase)<<3
	}
	innerMask := 0xFF
	if m.outerChrSmall {
		innerMask = 0x7F
	}
	outer := (int(m.chrBase) << 3) &^ innerMask
	var page int
	switch slot {
	case 0:
		page = int(m.mmc3Regs[0] &^ 1)
	case 1:
		page = int(m.mmc3Regs[0] | 1)
	case 2:
		page = int(m.mmc3Regs[1] &^ 1)
	case 3:
		page = int(m.mmc3Regs[1] | 1)
	default:
		page = int(m.mmc3Regs[slot-2])
	}
	return page&innerMask | outer
}

// chrIsRAM reports whether a CHR page is served by the board's RAM.
func (m *Fk23C) chrIsRAM(page int) bool {
	return m.chr == nil || m.selectChrRAM ||
		(m.wramConfigEnabled && m.ramInFirstChrBank && page <= 7)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Fk23C) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return m.win(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.wramConfigEnabled {
			return m.wram[int(m.wramBank)<<13|int(addr&0x1FFF)]
		}
		if m.wramEnabled {
			return m.wram[addr&0x1FFF]
		}
		return m.openBus()
	default: // $4020-$5FFF: the next WRAM bank when the config mode is on
		if m.wramConfigEnabled {
			return m.wram[int((m.wramBank+1)&3)<<13|int(addr&0x1FFF)]
		}
		return m.openBus()
	}
}

// WritePRG handles a CPU write into the PRG address space.
func (m *Fk23C) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.writeMMC3(addr, v)
	case addr >= 0x6000:
		if m.wramConfigEnabled {
			m.wram[int(m.wramBank)<<13|int(addr&0x1FFF)] = v
		} else if m.wramEnabled && !m.wramWriteProtect {
			m.wram[addr&0x1FFF] = v
		}
	case addr >= 0x5000:
		if m.fk23RegsEnabled || !m.wramConfigEnabled {
			// Registers respond only when A4 is set ($5010-style decoding).
			if addr&0x0010 != 0 {
				m.writeFkReg(addr, v)
			}
		} else {
			// Registers disabled: this range is part of the WRAM window.
			m.wram[int((m.wramBank+1)&3)<<13|int(addr&0x1FFF)] = v
		}
	default: // $4020-$4FFF
		if m.wramConfigEnabled {
			m.wram[int((m.wramBank+1)&3)<<13|int(addr&0x1FFF)] = v
		}
	}
}

// writeFkReg decodes the four outer registers at $5010/$5011/$5012/$5013.
func (m *Fk23C) writeFkReg(addr uint16, v byte) {
	switch addr & 0x03 {
	case 0:
		m.prgMode = v & 0x07
		m.outerChrSmall = v&0x10 != 0
		m.selectChrRAM = v&0x20 != 0
		m.mmc3ChrMode = v&0x40 == 0
		m.prgBase = m.prgBase&^0x180 | uint16(v&0x80)<<1 | uint16(v&0x08)<<4
	case 1:
		m.prgBase = m.prgBase&^0x7F | uint16(v&0x7F)
	case 2:
		m.prgBase = m.prgBase&^0x200 | uint16(v&0x40)<<3
		m.chrBase = v
		m.cnromChrReg = 0
	case 3:
		m.extendedMmc3 = v&0x02 != 0
		m.cnromChrMode = v&0x44 != 0
	}
}

// writeMMC3 decodes the MMC3-compatible register pairs at $8000-$FFFF.
func (m *Fk23C) writeMMC3(addr uint16, v byte) {
	if m.cnromChrMode && (addr <= 0x9FFF || addr >= 0xC000) {
		m.cnromChrReg = v & 0x03
	}
	switch addr & 0xE001 {
	case 0x8000:
		// Subtype 2 (16 MiB PRG, no CHR ROM) swaps MMC3 commands $46/$47.
		if len(m.prg) == 16384*1024 && (v == 0x46 || v == 0x47) {
			v ^= 1
		}
		m.invertPrgA14 = v&0x40 != 0
		m.invertChrA12 = v&0x80 != 0
		m.curReg = v & 0x0F
	case 0x8001:
		mask := byte(0x07)
		if m.extendedMmc3 {
			mask = 0x0F
		}
		if r := m.curReg & mask; r < 12 {
			m.mmc3Regs[r] = v
		}
	case 0xA000:
		m.mirrorReg = v & 0x03
	case 0xA001:
		if v&0x20 == 0 {
			// Without bit 5 only the enable/protect bits are honored.
			v &= 0xC0
		}
		m.wramBank = v & 0x03
		m.ramInFirstChrBank = v&0x04 != 0
		m.allowSingleScreen = v&0x08 != 0
		m.wramConfigEnabled = v&0x20 != 0
		m.fk23RegsEnabled = v&0x40 != 0
		m.wramWriteProtect = v&0x40 != 0
		m.wramEnabled = v&0x80 != 0
	case 0xC000:
		m.irqLatch = v
	case 0xC001:
		m.irqCounter = 0
		m.irqReload = true
	case 0xE000:
		m.irqEnabled = false
		m.irqAsserted = false
	case 0xE001:
		m.irqEnabled = true
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Fk23C) ReadCHR(addr uint16) byte {
	page := m.chrPage(addr)
	if m.chrIsRAM(page) {
		return m.bigCHR[(page<<10|int(addr&0x3FF))%len(m.bigCHR)]
	}
	return m.win(m.chr, page, 0x400)[addr&0x3FF]
}

// WriteCHR handles a write into the CHR address space.
func (m *Fk23C) WriteCHR(addr uint16, v byte) {
	page := m.chrPage(addr)
	if m.chrIsRAM(page) {
		m.bigCHR[(page<<10|int(addr&0x3FF))%len(m.bigCHR)] = v
	}
}

// Mirroring reports the board's current nametable mirroring.
func (m *Fk23C) Mirroring() cartridge.Mirroring {
	mask := byte(0x01)
	if m.allowSingleScreen {
		mask = 0x03
	}
	switch m.mirrorReg & mask {
	case 0:
		return cartridge.Vertical
	case 1:
		return cartridge.Horizontal
	case 2:
		return cartridge.SingleLow
	default:
		return cartridge.SingleHigh
	}
}

// Scanline clocks the MMC3-style IRQ counter; hitting zero schedules the
// assert two CPU cycles later (the FK23C's observed delay).
func (m *Fk23C) Scanline() {
	if m.irqCounter == 0 || m.irqReload {
		m.irqCounter = m.irqLatch
	} else {
		m.irqCounter--
	}
	if m.irqCounter == 0 && m.irqEnabled {
		m.irqDelay = 2
	}
	m.irqReload = false
}

// Tick counts down the IRQ assert delay.
func (m *Fk23C) Tick() {
	if m.irqDelay > 0 {
		m.irqDelay--
		if m.irqDelay == 0 {
			m.irqAsserted = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Fk23C) IRQ() bool { return m.irqAsserted }

// Reset observes the console reset line: on battery-backed CHR-RAM
// configurations a soft reset returns to the multicart menu bank.
func (m *Fk23C) Reset(soft bool) {
	if soft && m.wramConfigEnabled && m.selectChrRAM && m.hasBattery {
		m.prgBase = 0
	}
}

// Save writes the board's mapper-specific state into s. The 32 KiB work
// RAM fills the PRG-RAM area exactly; of the (rarely fully used) CHR
// RAM only the first 8 KiB rides in the snapshot, like the Coolboy.
func (m *Fk23C) Save(s *State) {
	copy(s.PRGRAM[:], m.wram[:])
	copy(s.CHRRAM[:], m.bigCHR[:len(s.CHRRAM)])
	s.Regs[0] = m.prgMode
	s.Regs[1] = boolByte(m.outerChrSmall)
	s.Regs[2] = boolByte(m.selectChrRAM)
	s.Regs[3] = boolByte(m.mmc3ChrMode)
	s.Regs[4] = boolByte(m.cnromChrMode)
	s.Regs[5] = byte(m.prgBase)
	s.Regs[6] = byte(m.prgBase >> 8)
	s.Regs[7] = m.chrBase
	s.Regs[8] = boolByte(m.extendedMmc3)
	s.Regs[9] = m.wramBank
	s.Regs[10] = boolByte(m.ramInFirstChrBank)
	s.Regs[11] = boolByte(m.allowSingleScreen)
	s.Regs[12] = boolByte(m.fk23RegsEnabled)
	s.Regs[13] = boolByte(m.wramConfigEnabled)
	s.Regs[14] = boolByte(m.wramEnabled)
	s.Regs[15] = boolByte(m.wramWriteProtect)
	s.Regs[16] = boolByte(m.invertPrgA14)
	s.Regs[17] = boolByte(m.invertChrA12)
	s.Regs[18] = m.curReg
	s.Regs[19] = m.mirrorReg
	s.Regs[20] = m.cnromChrReg
	s.Regs[21] = m.irqLatch
	s.Regs[22] = m.irqCounter
	s.Regs[23] = boolByte(m.irqReload)
	s.Regs[24] = boolByte(m.irqEnabled)
	s.Regs[25] = boolByte(m.irqAsserted)
	s.Regs[26] = m.irqDelay
	copy(s.Regs[27:39], m.mmc3Regs[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Fk23C) Restore(s *State) {
	copy(m.wram[:], s.PRGRAM[:])
	copy(m.bigCHR[:len(s.CHRRAM)], s.CHRRAM[:])
	m.prgMode = s.Regs[0]
	m.outerChrSmall = s.Regs[1] != 0
	m.selectChrRAM = s.Regs[2] != 0
	m.mmc3ChrMode = s.Regs[3] != 0
	m.cnromChrMode = s.Regs[4] != 0
	m.prgBase = uint16(s.Regs[5]) | uint16(s.Regs[6])<<8
	m.chrBase = s.Regs[7]
	m.extendedMmc3 = s.Regs[8] != 0
	m.wramBank = s.Regs[9]
	m.ramInFirstChrBank = s.Regs[10] != 0
	m.allowSingleScreen = s.Regs[11] != 0
	m.fk23RegsEnabled = s.Regs[12] != 0
	m.wramConfigEnabled = s.Regs[13] != 0
	m.wramEnabled = s.Regs[14] != 0
	m.wramWriteProtect = s.Regs[15] != 0
	m.invertPrgA14 = s.Regs[16] != 0
	m.invertChrA12 = s.Regs[17] != 0
	m.curReg = s.Regs[18]
	m.mirrorReg = s.Regs[19]
	m.cnromChrReg = s.Regs[20]
	m.irqLatch = s.Regs[21]
	m.irqCounter = s.Regs[22]
	m.irqReload = s.Regs[23] != 0
	m.irqEnabled = s.Regs[24] != 0
	m.irqAsserted = s.Regs[25] != 0
	m.irqDelay = s.Regs[26]
	copy(m.mmc3Regs[:], s.Regs[27:39])
}
