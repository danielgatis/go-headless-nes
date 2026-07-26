package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Another batch of multicart/vendor boards from the reference emulator.

// Supervision (mapper 53): an outer/inner PRG register pair with a NROM/
// menu split and a PRG-ROM window at $6000. The CRC-based "EPROM first"
// layout of a couple of dumps is not detected; the common layout is used.
type Supervision struct {
	base
	regs [2]byte
}

// NewSupervision constructs the corresponding board.
func NewSupervision(c *cartridge.Cartridge) *Supervision {
	return &Supervision{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Supervision) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.regs[0] = v
		}
		return
	}
	m.regs[1] = v
}

func (m *Supervision) rBase() int { return int(m.regs[0]) << 3 & 0x78 }

func (m *Supervision) prg8(slot int) int {
	r := m.rBase()
	if m.regs[0]&0x10 != 0 {
		if slot < 2 {
			return ((r | int(m.regs[1]&0x07)) << 1) + slot
		}
		return ((r | 0x07) << 1) + (slot - 2)
	}
	// menu mode: fixed high banks
	return 0x80<<1 + slot
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Supervision) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		slot := int((addr - 0x8000) >> 13)
		return m.win(m.prg, m.prg8(slot), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.win(m.prg, m.rBase()<<1|0x0F, 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Supervision) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Supervision) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Supervision) Mirroring() cartridge.Mirroring {
	return hvMirror(m.regs[0]&0x20 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Supervision) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.regs[0]
	s.Regs[1] = m.regs[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *Supervision) Restore(s *State) {
	m.restoreRAM(s)
	m.regs[0] = s.Regs[0]
	m.regs[1] = s.Regs[1]
}

// Bs5 (mapper 286): four 8 KiB PRG windows and four 2 KiB CHR windows,
// selected via $8000/$A000. The DIP-switch gate on the PRG write is taken
// as open (DIP 0).
type Bs5 struct {
	base
	prgReg [4]byte
	chrReg [4]byte
}

// NewBs5 constructs the corresponding board.
func NewBs5(c *cartridge.Cartridge) *Bs5 {
	m := &Bs5{base: makeBase(c)}
	for i := range m.prgReg {
		m.prgReg[i] = 0xFF
		m.chrReg[i] = 0xFF
	}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bs5) WritePRG(addr uint16, _ byte) {
	if addr < 0x8000 {
		return
	}
	bank := (addr >> 10) & 0x03
	switch addr & 0xF000 {
	case 0x8000:
		m.chrReg[bank] = byte(addr & 0x1F)
	case 0xA000:
		if addr&(1<<(0+4)) != 0 { // DIP=0
			m.prgReg[bank] = byte(addr & 0x0F)
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bs5) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := (addr - 0x8000) >> 13
		return m.win(m.prg, int(m.prgReg[slot]), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bs5) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>11&3]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Bs5) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>11&3]), 0x800, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Bs5) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.prgReg[:])
	copy(s.Regs[4:8], m.chrReg[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Bs5) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgReg[:], s.Regs[0:4])
	copy(m.chrReg[:], s.Regs[4:8])
}

// Hp898f (mapper 315): two $6000-region registers drive a 16/32 KiB PRG
// window, a CHR bank and mirroring.
type Hp898f struct {
	base
	regs   [2]byte
	prg0   int
	prg1   int
	chr    int
	mirror cartridge.Mirroring
}

// NewHp898f constructs the corresponding board.
func NewHp898f(c *cartridge.Cartridge) *Hp898f {
	m := &Hp898f{base: makeBase(c)}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Hp898f) WritePRG(addr uint16, v byte) {
	if addr&0x6000 == 0x6000 {
		m.regs[(addr&0x04)>>2] = v
		m.update()
	}
}

func (m *Hp898f) update() {
	prgReg := int(m.regs[1]>>3) & 7
	prgMask := int(m.regs[1]>>4) & 4
	m.chr = int((m.regs[0]>>4)&0x07) &^ (int(m.regs[0]&0x01)<<2 | int(m.regs[0]&0x02))
	m.prg0 = prgReg &^ prgMask
	m.prg1 = prgReg | prgMask
	m.mirror = hvMirror(m.regs[1]&0x80 != 0)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Hp898f) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Hp898f) ReadCHR(addr uint16) byte { return m.chrRead(m.chr, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Hp898f) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chr, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Hp898f) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Hp898f) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.regs[0]
	s.Regs[1] = m.regs[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *Hp898f) Restore(s *State) {
	m.restoreRAM(s)
	m.regs[0] = s.Regs[0]
	m.regs[1] = s.Regs[1]
	m.update()
}

// Bmc60311C (mapper 289): inner/outer PRG registers with NROM-128/256 and
// UNROM sub-modes.
type Bmc60311C struct {
	base
	inner  byte
	outer  byte
	mode   byte
	mirror cartridge.Mirroring
}

// NewBmc60311C constructs the corresponding board.
func NewBmc60311C(c *cartridge.Cartridge) *Bmc60311C {
	m := &Bmc60311C{base: makeBase(c)}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc60311C) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.inner = v & 0x07
	} else {
		switch addr & 0xE001 {
		case 0x6000:
			m.mode = v & 0x0F
		case 0x6001:
			m.outer = v
		}
	}
	m.update()
}

func (m *Bmc60311C) update() {
	m.mirror = hvMirror(m.mode&0x08 == 0)
}

func (m *Bmc60311C) page() int {
	if m.mode&0x04 != 0 {
		return int(m.outer)
	}
	return int(m.outer) | int(m.inner)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc60311C) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			return m.readPRGRAM(addr)
		}
		return m.openBus()
	}
	p := m.page()
	switch m.mode & 0x03 {
	case 0: // NROM-128
		return m.win(m.prg, p, 0x4000)[addr&0x3FFF]
	case 1: // NROM-256
		bank := (p & 0xFE) + int(addr>>14&1)
		return m.win(m.prg, bank, 0x4000)[addr&0x3FFF]
	case 2: // UNROM
		if addr >= 0xC000 {
			return m.win(m.prg, int(m.outer)|7, 0x4000)[addr&0x3FFF]
		}
		return m.win(m.prg, p, 0x4000)[addr&0x3FFF]
	default:
		return m.win(m.prg, p, 0x4000)[addr&0x3FFF]
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc60311C) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc60311C) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc60311C) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc60311C) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.inner
	s.Regs[1] = m.outer
	s.Regs[2] = m.mode
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc60311C) Restore(s *State) {
	m.restoreRAM(s)
	m.inner = s.Regs[0]
	m.outer = s.Regs[1]
	m.mode = s.Regs[2]
	m.update()
}

// Bmc830425C4391T (mapper 320): inner (value) and outer (address) PRG
// registers with UNROM/UOROM modes.
type Bmc830425C4391T struct {
	base
	inner byte
	outer byte
	mode  byte
}

// NewBmc830425C4391T constructs the corresponding board.
func NewBmc830425C4391T(c *cartridge.Cartridge) *Bmc830425C4391T {
	return &Bmc830425C4391T{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc830425C4391T) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	m.inner = v & 0x0F
	if addr&0xFFE0 == 0xF0E0 {
		m.outer = byte(addr & 0x0F)
		m.mode = byte(addr>>4) & 0x01
	}
}

func (m *Bmc830425C4391T) prg0() int {
	if m.mode != 0 {
		return int(m.inner&0x07) | int(m.outer)<<3
	}
	return int(m.inner) | int(m.outer)<<3
}
func (m *Bmc830425C4391T) prg1() int {
	if m.mode != 0 {
		return 0x07 | int(m.outer)<<3
	}
	return 0x0F | int(m.outer)<<3
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc830425C4391T) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, m.prg1(), 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prg0(), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc830425C4391T) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc830425C4391T) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Bmc830425C4391T) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.inner
	s.Regs[1] = m.outer
	s.Regs[2] = m.mode
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc830425C4391T) Restore(s *State) {
	m.restoreRAM(s)
	m.inner = s.Regs[0]
	m.outer = s.Regs[1]
	m.mode = s.Regs[2]
}

// Kaiser7013B (mapper 312): a switchable 16 KiB PRG window at $8000 (last
// fixed at $C000) with H/V mirroring at $8000+.
type Kaiser7013B struct {
	base
	prg0   int
	mirror cartridge.Mirroring
}

// NewKaiser7013B constructs the corresponding board.
func NewKaiser7013B(c *cartridge.Cartridge) *Kaiser7013B {
	return &Kaiser7013B{base: makeBase(c), mirror: cartridge.Vertical}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7013B) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.prg0 = int(v)
		}
		return
	}
	m.mirror = hvMirror(v&0x01 == 0)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7013B) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7013B) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7013B) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Kaiser7013B) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7013B) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7013B) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.mirror = cartridge.Mirroring(s.Regs[1])
}
