package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Waixing boards, ported from the reference emulator.

// Waixing162 (mapper 162): a single 32 KiB PRG window selected from four
// $5000-region registers, decoded through a small mode table. CHR is
// fixed 8 KiB RAM.
type Waixing162 struct {
	base
	regs [4]byte
}

// NewWaixing162 wires the board.
func NewWaixing162(c *cartridge.Cartridge) *Waixing162 {
	m := &Waixing162{base: makeBase(c)}
	m.regs = [4]byte{3, 0, 0, 7}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Waixing162) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x6000 {
		m.regs[(addr>>8)&0x03] = v
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

func (m *Waixing162) prgBank() int {
	r0, r1, r2 := int(m.regs[0]), int(m.regs[1]), int(m.regs[2])
	switch m.regs[3] & 0x05 {
	case 0:
		return (r0 & 0x0C) | (r1 & 0x02) | (r2&0x0F)<<4
	case 1:
		return (r0 & 0x0C) | (r2&0x0F)<<4
	case 4:
		return (r0 & 0x0E) | (r1>>1)&0x01 | (r2&0x0F)<<4
	default: // 5
		return (r0 & 0x0F) | (r2&0x0F)<<4
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Waixing162) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank(), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Waixing162) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Waixing162) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Waixing162) Save(s *State) { m.saveRAM(s); copy(s.Regs[0:4], m.regs[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *Waixing162) Restore(s *State) { m.restoreRAM(s); copy(m.regs[:], s.Regs[0:4]) }

// Waixing164 (mapper 164): a single 32 KiB PRG window whose bank is built
// from two nibbles written to $5000/$5100. CHR is fixed 8 KiB RAM.
type Waixing164 struct {
	base
	prgBank byte
}

// NewWaixing164 wires the board.
func NewWaixing164(c *cartridge.Cartridge) *Waixing164 {
	return &Waixing164{base: makeBase(c), prgBank: 0x0F}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Waixing164) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x6000 {
		switch addr & 0x7300 {
		case 0x5000:
			m.prgBank = m.prgBank&0xF0 | v&0x0F
		case 0x5100:
			m.prgBank = m.prgBank&0x0F | (v&0x0F)<<4
		}
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Waixing164) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Waixing164) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Waixing164) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Waixing164) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.prgBank }

// Restore loads the board's mapper-specific state from s.
func (m *Waixing164) Restore(s *State) { m.restoreRAM(s); m.prgBank = s.Regs[0] }

// Waixing178 (mapper 178): four $4800-region registers select a 16/32 KiB
// PRG window with a block (bbank) and sub-bank (sbank) split, plus H/V
// mirroring. The board has 32 KiB of work RAM banked at $6000; the repo
// models a single 8 KiB WRAM, so that bank select is not applied.
type Waixing178 struct {
	base
	regs   [4]byte
	mirror cartridge.Mirroring
}

// NewWaixing178 wires the board.
func NewWaixing178(c *cartridge.Cartridge) *Waixing178 {
	m := &Waixing178{base: makeBase(c), mirror: cartridge.Vertical}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Waixing178) WritePRG(addr uint16, v byte) {
	if addr >= 0x4800 && addr < 0x5000 {
		m.regs[addr&0x03] = v
		m.mirror = hvMirror(m.regs[0]&0x01 == 0)
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// prgSlots returns the two 16 KiB PRG banks at $8000 and $C000.
func (m *Waixing178) prgSlots() (bank0, bank1 int) {
	sbank := int(m.regs[1] & 0x07)
	bbank := int(m.regs[2])
	if m.regs[0]&0x02 != 0 {
		bank0 = bbank<<3 | sbank
		if m.regs[0]&0x04 != 0 {
			bank1 = bbank<<3 | 0x06 | int(m.regs[1]&0x01)
		} else {
			bank1 = bbank<<3 | 0x07
		}
		return bank0, bank1
	}
	bank := bbank<<3 | sbank
	if m.regs[0]&0x04 != 0 {
		return bank, bank
	}
	return bank & 0xFE, bank&0xFE | 1
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Waixing178) ReadPRG(addr uint16) byte {
	if addr < 0x6000 {
		return m.openBus()
	}
	if addr < 0x8000 {
		return m.readPRGRAM(addr)
	}
	b0, b1 := m.prgSlots()
	if addr < 0xC000 {
		return m.win(m.prg, b0, 0x4000)[addr&0x3FFF]
	}
	return m.win(m.prg, b1, 0x4000)[addr&0x3FFF]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Waixing178) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Waixing178) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Waixing178) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Waixing178) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.regs[:])
	s.Regs[4] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Waixing178) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.regs[:], s.Regs[0:4])
	m.mirror = cartridge.Mirroring(s.Regs[4])
}

// Waixing252 (mapper 252): a VRC-like board with two switchable 8 KiB PRG
// windows, eight 1 KiB CHR windows loaded a nibble at a time through a
// scrambled address decode, and the shared VRC cycle IRQ.
type Waixing252 struct {
	base

	prg0, prg1 byte
	chrRegs    [8]byte
	irq        vrcIRQ
}

// NewWaixing252 wires the board.
func NewWaixing252(c *cartridge.Cartridge) *Waixing252 {
	return &Waixing252{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Waixing252) WritePRG(addr uint16, v byte) {
	switch {
	case addr < 0x6000:
		return
	case addr < 0x8000:
		m.writePRGRAM(addr, v)
	case addr <= 0x8FFF:
		m.prg0 = v
	case addr >= 0xA000 && addr <= 0xAFFF:
		m.prg1 = v
	case addr >= 0xB000 && addr <= 0xEFFF:
		shift := addr & 0x04
		bank := (((addr - 0xB000) >> 1 & 0x1800) | (addr << 7 & 0x0400)) / 0x400
		m.chrRegs[bank] = m.chrRegs[bank]&byte(0xF0>>shift) | (v&0x0F)<<shift
	default: // $F000-$FFFF
		switch addr & 0xF00C {
		case 0xF000:
			m.irq.setReloadLow(v)
		case 0xF004:
			m.irq.setReloadHigh(v)
		case 0xF008:
			m.irq.setControl(v)
		case 0xF00C:
			m.irq.ack()
		}
	}
}

func (m *Waixing252) prgBank(addr uint16) int {
	switch (addr >> 13) & 3 {
	case 0:
		return int(m.prg0)
	case 1:
		return int(m.prg1)
	case 2:
		return -2
	default:
		return -1
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Waixing252) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Waixing252) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrRegs[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Waixing252) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrRegs[addr>>10&7]), 0x400, addr, v)
}

// Tick advances the board by one cycle.
func (m *Waixing252) Tick() { m.irq.tick() }

// IRQ reports whether the board is asserting the IRQ line.
func (m *Waixing252) IRQ() bool { return m.irq.line }

// Save writes the board's mapper-specific state into s.
func (m *Waixing252) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prg0
	s.Regs[1] = m.prg1
	copy(s.Regs[2:10], m.chrRegs[:])
	m.irq.save(s.Regs[10:17])
}

// Restore loads the board's mapper-specific state from s.
func (m *Waixing252) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = s.Regs[0]
	m.prg1 = s.Regs[1]
	copy(m.chrRegs[:], s.Regs[2:10])
	m.irq.restore(s.Regs[10:17])
}
