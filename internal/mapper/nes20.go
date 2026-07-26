package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// NES 2.0-only multicart boards ported from the reference emulator.

// A65AS (mapper 285): a value-latched multicart with a 16/32 KiB PRG
// window and register-controlled single-screen or H/V mirroring.
type A65AS struct{ dualPRG16 }

// NewA65AS wires the board.
func NewA65AS(c *cartridge.Cartridge) *A65AS {
	m := &A65AS{dualPRG16{base: makeBase(c)}}
	m.decode(0)
	return m
}

func (m *A65AS) decode(v byte) {
	if v&0x40 != 0 {
		b := int(v & 0x1E)
		m.prg0, m.prg1 = b, b|1
	} else {
		m.prg0 = int(v&0x30)>>1 | int(v&0x07)
		m.prg1 = int(v&0x30)>>1 | 0x07
	}
	if v&0x80 != 0 {
		if v&0x20 != 0 {
			m.mirror = cartridge.SingleHigh
		} else {
			m.mirror = cartridge.SingleLow
		}
	} else {
		m.mirror = hvMirror(v&0x08 == 0)
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *A65AS) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(v)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *A65AS) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *A65AS) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *A65AS) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *A65AS) Restore(s *State) { m.restoreDual(s) }

// T262 (mapper 265): a multicart whose outer base and mode are latched
// from the write address (until locked), and the inner bank from the
// data. The two 16 KiB windows combine the base with the inner bank.
type T262 struct {
	base
	locked bool
	baseB  byte
	bank   byte
	mode   bool
	mirror cartridge.Mirroring
}

// NewT262 wires the board.
func NewT262(c *cartridge.Cartridge) *T262 {
	return &T262{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *T262) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	if !m.locked {
		m.baseB = byte((addr&0x60)>>2) | byte((addr&0x100)>>3)
		m.mode = addr&0x80 != 0
		m.locked = addr&0x2000 != 0
		m.mirror = hvMirror(addr&0x02 == 0)
	}
	m.bank = v & 0x07
}

func (m *T262) prgSlots() (int, int) {
	b0 := int(m.baseB | m.bank)
	inner := byte(7)
	if m.mode {
		inner = m.bank
	}
	return b0, int(m.baseB | inner)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *T262) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		p0, p1 := m.prgSlots()
		if addr < 0xC000 {
			return m.win(m.prg, p0, 0x4000)[addr&0x3FFF]
		}
		return m.win(m.prg, p1, 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *T262) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *T262) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *T262) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *T262) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = boolByte(m.locked)
	s.Regs[1] = m.baseB
	s.Regs[2] = m.bank
	s.Regs[3] = boolByte(m.mode)
	s.Regs[4] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *T262) Restore(s *State) {
	m.restoreRAM(s)
	m.locked = s.Regs[0] != 0
	m.baseB = s.Regs[1]
	m.bank = s.Regs[2]
	m.mode = s.Regs[3] != 0
	m.mirror = cartridge.Mirroring(s.Regs[4])
}

// Gs2004 (mapper 283): a 32 KiB PRG window from the low 3 bits of the
// written value, plus a fixed PRG-ROM window at $6000.
type Gs2004 struct {
	base
	prgBank int
}

// NewGs2004 wires the board.
func NewGs2004(c *cartridge.Cartridge) *Gs2004 {
	return &Gs2004{base: makeBase(c), prgBank: 0x07}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Gs2004) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = int(v & 0x07)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Gs2004) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return m.win(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	case addr >= 0x6000:
		// A fixed 8 KiB PRG-ROM window (the reference maps bank 0x20 in 8 KiB units).
		return m.win(m.prg, 0x20, 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Gs2004) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Gs2004) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Gs2004) Save(s *State) { m.saveRAM(s); s.Regs[0] = byte(m.prgBank) }

// Restore loads the board's mapper-specific state from s.
func (m *Gs2004) Restore(s *State) { m.restoreRAM(s); m.prgBank = int(s.Regs[0]) }

// Gkcx1 (mapper 288): the write address selects a 32 KiB PRG bank (bits
// 3-4) and an 8 KiB CHR bank (bits 0-2).
type Gkcx1 struct {
	base
	prgBank int
	chrBank int
}

// NewGkcx1 wires the board.
func NewGkcx1(c *cartridge.Cartridge) *Gkcx1 {
	return &Gkcx1{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Gkcx1) WritePRG(addr uint16, _ byte) {
	if addr >= 0x8000 {
		m.prgBank = int(addr>>3) & 0x03
		m.chrBank = int(addr & 0x07)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Gkcx1) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Gkcx1) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Gkcx1) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Gkcx1) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
}

// Restore loads the board's mapper-specific state from s.
func (m *Gkcx1) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
}

// Edu2000 (mapper 329): a value write sets a 32 KiB PRG bank (bits 0-4)
// and a banked 8 KiB work RAM window (bits 6-7). The 32 KiB banked WRAM
// is approximated by the repo's single 8 KiB PRG-RAM.
type Edu2000 struct {
	base
	reg byte
}

// NewEdu2000 wires the board.
func NewEdu2000(c *cartridge.Cartridge) *Edu2000 {
	return &Edu2000{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Edu2000) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.reg = v
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Edu2000) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, int(m.reg&0x1F), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Edu2000) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Edu2000) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Edu2000) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *Edu2000) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// Smb2j (mapper 304): a Famicom SMB2j pirate board with 4 KiB PRG windows
// (two switchable 16 KiB halves via $4022), fixed ROM windows at
// $5000-$7FFF, and a 12-bit cycle IRQ armed by $4122.
type Smb2j struct {
	base
	prgHalf    int // 0 or 1: outer 16 KiB select for the low half
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewSmb2j wires the board.
func NewSmb2j(c *cartridge.Cartridge) *Smb2j {
	return &Smb2j{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Smb2j) WritePRG(addr uint16, v byte) {
	switch addr {
	case 0x4022:
		m.prgHalf = int(v & 0x01)
	case 0x4122:
		m.irqEnabled = v&0x03 != 0
		m.irqCounter = 0
		m.irqLine = false
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Smb2j) ReadPRG(addr uint16) byte {
	n := len(m.prg) / 0x1000
	switch {
	case addr >= 0x8000:
		// Eight 4 KiB banks: low four from prgHalf*4, high four fixed at 4-7.
		slot := int((addr - 0x8000) >> 12)
		if slot < 4 {
			return m.win(m.prg, m.prgHalf*4+slot, 0x1000)[addr&0x0FFF]
		}
		return m.win(m.prg, slot, 0x1000)[addr&0x0FFF]
	case addr >= 0x7000:
		return m.win(m.prg, n-1, 0x1000)[addr&0x0FFF]
	case addr >= 0x6000:
		return m.win(m.prg, n-2, 0x1000)[addr&0x0FFF]
	case addr >= 0x5000:
		return m.win(m.prg, n-3, 0x1000)[addr&0x0FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Smb2j) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Smb2j) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Tick advances the board by one cycle.
func (m *Smb2j) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter = (m.irqCounter + 1) & 0xFFF
	if m.irqCounter == 0 {
		m.irqEnabled = false
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Smb2j) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Smb2j) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgHalf)
	s.Regs[1] = byte(m.irqCounter)
	s.Regs[2] = byte(m.irqCounter >> 8)
	s.Regs[3] = boolByte(m.irqEnabled)
	s.Regs[4] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Smb2j) Restore(s *State) {
	m.restoreRAM(s)
	m.prgHalf = int(s.Regs[0])
	m.irqCounter = uint16(s.Regs[1]) | uint16(s.Regs[2])<<8
	m.irqEnabled = s.Regs[3] != 0
	m.irqLine = s.Regs[4] != 0
}

// Lh51 (mappers 308, 309): a switchable 8 KiB PRG window at $8000 with
// the top of the ROM fixed at $A000-$FFFF, an 8 KiB CHR RAM, and H/V
// mirroring at $E000.
type Lh51 struct {
	base
	prg0   byte
	mirror cartridge.Mirroring
}

// NewLh51 wires the board.
func NewLh51(c *cartridge.Cartridge) *Lh51 {
	return &Lh51{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Lh51) WritePRG(addr uint16, v byte) {
	switch addr & 0xE000 {
	case 0x8000:
		m.prg0 = v & 0x0F
	case 0xE000:
		m.mirror = hvMirror(v&0x08 == 0)
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Lh51) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xA000:
		// Three fixed top banks (13, 14, 15 for a 128 KiB ROM); use the
		// last three 8 KiB banks of the ROM generically.
		slot := int((addr - 0xA000) >> 13) // 0..2
		return m.win(m.prg, -3+slot, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prg0), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Lh51) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Lh51) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Lh51) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Lh51) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prg0
	s.Regs[1] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Lh51) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = s.Regs[0]
	m.mirror = cartridge.Mirroring(s.Regs[1])
}

// Rt01 (mappers 327, 328): a protection board that returns a semi-random
// value from two address windows; banking is fixed. Hardware returns
// 0xF2 | (rand & 0x0D); we return the stable base 0xF2.
type Rt01 struct{ base }

// NewRt01 wires the board.
func NewRt01(c *cartridge.Cartridge) *Rt01 {
	return &Rt01{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Rt01) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Rt01) ReadPRG(addr uint16) byte {
	if (addr >= 0xCE80 && addr < 0xCF00) || (addr >= 0xFE80 && addr < 0xFF00) {
		return 0xF2
	}
	if addr >= 0x8000 {
		return m.win(m.prg, int((addr-0x8000)>>14), 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Rt01) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Rt01) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Rt01) Save(s *State) { m.saveRAM(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Rt01) Restore(s *State) { m.restoreRAM(s) }

// T230 (mappers 524-529): a VRC4-like board (reusing the shared VRC IRQ)
// with two switchable 8 KiB PRG windows, an outer bank, eight 1 KiB CHR
// windows loaded a nibble at a time, and 4-way mirroring.
type T230 struct {
	base

	prgReg0 byte
	prgReg1 byte
	prgMode byte
	outer   int
	loCHR   [8]byte
	hiCHR   [8]byte
	mirror  cartridge.Mirroring
	irq     vrcIRQ
}

// NewT230 wires the board.
func NewT230(c *cartridge.Cartridge) *T230 {
	return &T230{base: makeBase(c), mirror: c.Mirroring}
}

// t230Addr collapses the scrambled A0/A1 lines like the reference does.
func t230Addr(addr uint16) uint16 {
	a := addr & 0xF000
	if addr&0x2A != 0 {
		a |= 0x02
	}
	if addr&0x15 != 0 {
		a |= 0x01
	}
	return a
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *T230) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	a := t230Addr(addr)
	switch {
	case a >= 0x9000 && a <= 0x9001:
		switch v & 0x03 {
		case 0:
			m.mirror = cartridge.Vertical
		case 1:
			m.mirror = cartridge.Horizontal
		case 2:
			m.mirror = cartridge.SingleLow
		case 3:
			m.mirror = cartridge.SingleHigh
		}
	case a >= 0x9002 && a <= 0x9003:
		m.prgMode = (v >> 1) & 0x01
	case a >= 0xA000 && a <= 0xA003:
		m.prgReg0 = (v & 0x1F) << 1
		m.prgReg1 = (v&0x1F)<<1 | 0x01
	case a >= 0xB000 && a <= 0xE003:
		if m.chr == nil {
			m.outer = int(v&0x08) << 2
		} else {
			reg := ((((a >> 12) & 0x07) - 3) << 1) | ((a >> 1) & 0x01)
			if a&0x01 == 0 {
				m.loCHR[reg] = v & 0x0F
			} else {
				m.hiCHR[reg] = v & 0x1F
			}
		}
	case a == 0xF000:
		m.irq.setReloadLow(v)
	case a == 0xF001:
		m.irq.setReloadHigh(v)
	case a == 0xF002:
		m.irq.setControl(v)
	case a == 0xF003:
		m.irq.ack()
	}
}

func (m *T230) prgBank(addr uint16) int {
	switch (addr >> 13) & 3 {
	case 0: // $8000
		if m.prgMode == 0 {
			return int(m.prgReg0) | m.outer
		}
		return (-2 & 0x1F) | m.outer
	case 1: // $A000
		return int(m.prgReg1)
	case 2: // $C000
		if m.prgMode == 0 {
			return (-2 & 0x1F) | m.outer
		}
		return int(m.prgReg0) | m.outer
	default: // $E000
		return -1
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *T230) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

func (m *T230) chrBank(addr uint16) int {
	if m.chr == nil {
		return 0
	}
	i := addr >> 10 & 7
	return int(m.loCHR[i]) | int(m.hiCHR[i])<<4
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *T230) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *T230) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank(addr), 0x400, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *T230) Mirroring() cartridge.Mirroring { return m.mirror }

// Tick advances the board by one cycle.
func (m *T230) Tick() { m.irq.tick() }

// IRQ reports whether the board is asserting the IRQ line.
func (m *T230) IRQ() bool { return m.irq.line }

// Save writes the board's mapper-specific state into s.
func (m *T230) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg0
	s.Regs[1] = m.prgReg1
	s.Regs[2] = m.prgMode
	s.Regs[3] = byte(m.outer)
	copy(s.Regs[4:12], m.loCHR[:])
	copy(s.Regs[12:20], m.hiCHR[:])
	s.Regs[20] = byte(m.mirror)
	m.irq.save(s.Regs[21:28])
}

// Restore loads the board's mapper-specific state from s.
func (m *T230) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg0 = s.Regs[0]
	m.prgReg1 = s.Regs[1]
	m.prgMode = s.Regs[2]
	m.outer = int(s.Regs[3])
	copy(m.loCHR[:], s.Regs[4:12])
	copy(m.hiCHR[:], s.Regs[12:20])
	m.mirror = cartridge.Mirroring(s.Regs[20])
	m.irq.restore(s.Regs[21:28])
}

// Tf1201 (mapper 298): a VRC-like board with two switchable 8 KiB PRG
// windows (swappable fixed slot), eight 1 KiB CHR windows loaded a nibble
// at a time, and a scanline-scaler cycle IRQ.
type Tf1201 struct {
	base
	prgReg  [2]byte
	chrReg  [8]byte
	swapPrg bool
	mirror  cartridge.Mirroring

	irqCounter byte
	irqReload  byte
	irqScaler  int16
	irqEnabled bool
	irqLine    bool
}

// NewTf1201 constructs the corresponding board.
func NewTf1201(c *cartridge.Cartridge) *Tf1201 {
	return &Tf1201{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Tf1201) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	addr = (addr & 0xF003) | (addr&0x0C)>>2
	if addr >= 0xB000 && addr <= 0xE003 {
		slot := (((addr >> 11) - 6) | (addr & 0x01)) & 0x07
		shift := (addr & 0x02) << 1
		m.chrReg[slot] = m.chrReg[slot]&byte(0xF0>>shift) | (v&0x0F)<<shift
		return
	}
	switch addr & 0xF003 {
	case 0x8000:
		m.prgReg[0] = v
	case 0xA000:
		m.prgReg[1] = v
	case 0x9000:
		m.mirror = hvMirror(v&0x01 == 0)
	case 0x9001:
		m.swapPrg = v&0x03 != 0
	case 0xF000:
		m.irqReload = m.irqReload&0xF0 | v&0x0F
	case 0xF002:
		m.irqReload = m.irqReload&0x0F | v<<4
	case 0xF001:
		m.irqEnabled = v&0x02 != 0
		if m.irqEnabled {
			m.irqScaler = 341
			m.irqCounter = m.irqReload
		}
		m.irqLine = false
	case 0xF003:
		m.irqLine = false
	}
}

func (m *Tf1201) prgBank(addr uint16) int {
	switch (addr >> 13) & 3 {
	case 0:
		if m.swapPrg {
			return -2
		}
		return int(m.prgReg[0])
	case 1:
		return int(m.prgReg[1])
	case 2:
		if m.swapPrg {
			return int(m.prgReg[0])
		}
		return -2
	default:
		return -1
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Tf1201) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Tf1201) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Tf1201) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Tf1201) Mirroring() cartridge.Mirroring { return m.mirror }

// Tick advances the board by one cycle.
func (m *Tf1201) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqScaler -= 3
	if m.irqScaler <= 0 {
		m.irqScaler += 341
		m.irqCounter++
		if m.irqCounter == 0 {
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Tf1201) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Tf1201) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg[0]
	s.Regs[1] = m.prgReg[1]
	copy(s.Regs[2:10], m.chrReg[:])
	s.Regs[10] = boolByte(m.swapPrg)
	s.Regs[11] = byte(m.mirror)
	s.Regs[12] = m.irqCounter
	s.Regs[13] = m.irqReload
	s.Regs[14] = byte(m.irqScaler)
	s.Regs[15] = byte(m.irqScaler >> 8)
	s.Regs[16] = boolByte(m.irqEnabled)
	s.Regs[17] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Tf1201) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg[0] = s.Regs[0]
	m.prgReg[1] = s.Regs[1]
	copy(m.chrReg[:], s.Regs[2:10])
	m.swapPrg = s.Regs[10] != 0
	m.mirror = cartridge.Mirroring(s.Regs[11])
	m.irqCounter = s.Regs[12]
	m.irqReload = s.Regs[13]
	m.irqScaler = int16(uint16(s.Regs[14]) | uint16(s.Regs[15])<<8)
	m.irqEnabled = s.Regs[16] != 0
	m.irqLine = s.Regs[17] != 0
}

// Ax5705 (mapper 530): two switchable 8 KiB PRG windows with scrambled
// data lines and eight 1 KiB CHR windows loaded a nibble at a time.
type Ax5705 struct {
	base
	prg0   int
	prg1   int
	chrReg [8]byte
	mirror cartridge.Mirroring
}

// NewAx5705 constructs the corresponding board.
func NewAx5705(c *cartridge.Cartridge) *Ax5705 {
	return &Ax5705{base: makeBase(c), mirror: c.Mirroring}
}

func ax5705Prg(v byte) int {
	return int(v&0x02)<<2 | int(v&0x08)>>2 | int(v&0x05)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Ax5705) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	if addr >= 0xA008 {
		low := addr&0x01 == 0
		var idx int
		switch addr & 0xF00E {
		case 0xA008:
			idx = 0
		case 0xA00A:
			idx = 1
		case 0xC000:
			idx = 2
		case 0xC002:
			idx = 3
		case 0xC008:
			idx = 4
		case 0xC00A:
			idx = 5
		case 0xE000:
			idx = 6
		case 0xE002:
			idx = 7
		default:
			return
		}
		if low {
			m.chrReg[idx] = m.chrReg[idx]&0xF0 | v&0x0F
		} else {
			hi := (v&0x04)>>1 | (v&0x02)<<1 | v&0x09
			m.chrReg[idx] = m.chrReg[idx]&0x0F | hi<<4
		}
		return
	}
	switch addr & 0xF00F {
	case 0x8000:
		m.prg0 = ax5705Prg(v)
	case 0x8008:
		m.mirror = hvMirror(v&0x01 == 0)
	case 0xA000:
		m.prg1 = ax5705Prg(v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Ax5705) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.win(m.prg, -2, 0x2000)[addr&0x1FFF]
	case addr >= 0xA000:
		return m.win(m.prg, m.prg1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prg0, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Ax5705) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Ax5705) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Ax5705) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Ax5705) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.prg1)
	copy(s.Regs[2:10], m.chrReg[:])
	s.Regs[10] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Ax5705) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.prg1 = int(s.Regs[1])
	copy(m.chrReg[:], s.Regs[2:10])
	m.mirror = cartridge.Mirroring(s.Regs[10])
}

// Mapper253 (253, Waixing VRC-like): two switchable 8 KiB PRG windows (top
// two fixed), eight 1 KiB CHR windows with 16-bit banks where banks 4/5
// route to 2 KiB CHR RAM, 4-way mirroring, and a scanline-scaler IRQ.
type Mapper253 struct {
	base
	prg0, prg1  byte
	chrLow      [8]byte
	chrHigh     [8]byte
	forceChrRom bool
	mirror      cartridge.Mirroring
	irqReload   byte
	irqCounter  byte
	irqScaler   uint16
	irqEnabled  bool
	irqLine     bool
}

// NewMapper253 constructs the corresponding board.
func NewMapper253(c *cartridge.Cartridge) *Mapper253 {
	return &Mapper253{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper253) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	if addr >= 0xB000 && addr <= 0xE00C {
		slot := ((((addr & 0x08) | (addr >> 8)) >> 3) + 2) & 0x07
		shift := addr & 0x04
		m.chrLow[slot] = m.chrLow[slot]&byte(0xF0>>shift) | v<<shift
		if slot == 0 {
			switch m.chrLow[0] {
			case 0xC8:
				m.forceChrRom = false
			case 0x88:
				m.forceChrRom = true
			}
		}
		if shift != 0 {
			m.chrHigh[slot] = v >> 4
		}
		return
	}
	switch addr {
	case 0x8010:
		m.prg0 = v
	case 0xA010:
		m.prg1 = v
	case 0x9400:
		switch v & 0x03 {
		case 0:
			m.mirror = cartridge.Vertical
		case 1:
			m.mirror = cartridge.Horizontal
		case 2:
			m.mirror = cartridge.SingleLow
		case 3:
			m.mirror = cartridge.SingleHigh
		}
	case 0xF000:
		m.irqReload = m.irqReload&0xF0 | v&0x0F
		m.irqLine = false
	case 0xF004:
		m.irqReload = m.irqReload&0x0F | v<<4
		m.irqLine = false
	case 0xF008:
		m.irqCounter = m.irqReload
		m.irqEnabled = v&0x02 != 0
		m.irqScaler = 0
		m.irqLine = false
	}
}

func (m *Mapper253) prgBank(addr uint16) int {
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
func (m *Mapper253) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

func (m *Mapper253) chrSel(addr uint16) (bank int, ram bool) {
	i := addr >> 10 & 7
	page := int(m.chrLow[i]) | int(m.chrHigh[i])<<8
	if (m.chrLow[i] == 4 || m.chrLow[i] == 5) && !m.forceChrRom {
		return page & 0x01, true
	}
	return page, false
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper253) ReadCHR(addr uint16) byte {
	bank, ram := m.chrSel(addr)
	if ram {
		return m.win(m.chrRAM[:], bank, 0x400)[addr&0x3FF]
	}
	return m.win(m.chr, bank, 0x400)[addr&0x3FF]
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper253) WriteCHR(addr uint16, v byte) {
	bank, ram := m.chrSel(addr)
	if ram {
		m.win(m.chrRAM[:], bank, 0x400)[addr&0x3FF] = v
	}
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper253) Mirroring() cartridge.Mirroring { return m.mirror }

// Tick advances the board by one cycle.
func (m *Mapper253) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqScaler++
	if m.irqScaler >= 114 {
		m.irqScaler = 0
		m.irqCounter++
		if m.irqCounter == 0 {
			m.irqCounter = m.irqReload
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper253) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper253) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prg0
	s.Regs[1] = m.prg1
	copy(s.Regs[2:10], m.chrLow[:])
	copy(s.Regs[10:18], m.chrHigh[:])
	s.Regs[18] = boolByte(m.forceChrRom)
	s.Regs[19] = byte(m.mirror)
	s.Regs[20] = m.irqReload
	s.Regs[21] = m.irqCounter
	s.Regs[22] = byte(m.irqScaler)
	s.Regs[23] = byte(m.irqScaler >> 8)
	s.Regs[24] = boolByte(m.irqEnabled)
	s.Regs[25] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper253) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = s.Regs[0]
	m.prg1 = s.Regs[1]
	copy(m.chrLow[:], s.Regs[2:10])
	copy(m.chrHigh[:], s.Regs[10:18])
	m.forceChrRom = s.Regs[18] != 0
	m.mirror = cartridge.Mirroring(s.Regs[19])
	m.irqReload = s.Regs[20]
	m.irqCounter = s.Regs[21]
	m.irqScaler = uint16(s.Regs[22]) | uint16(s.Regs[23])<<8
	m.irqEnabled = s.Regs[24] != 0
	m.irqLine = s.Regs[25] != 0
}
