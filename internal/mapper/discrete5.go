package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// A few more vendor one-offs ported from the reference emulator.

// Mapper221 (221, N625092 multicart): a $8000 write latches a mode from
// the address, a $C000 write an inner PRG register; the two combine into
// a 16/32 KiB PRG window.
type Mapper221 struct {
	base
	mode   uint16
	prgReg byte
	prg0   int
	prg1   int
	mirror cartridge.Mirroring
}

// NewMapper221 wires the board.
func NewMapper221(c *cartridge.Cartridge) *Mapper221 {
	m := &Mapper221{base: makeBase(c)}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper221) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0xC000 {
	case 0x8000:
		m.mode = addr
	case 0xC000:
		m.prgReg = byte(addr & 0x07)
	}
	m.update()
}

func (m *Mapper221) update() {
	outer := int(m.mode&0xFC) >> 2
	if m.mode&0x02 != 0 {
		if m.mode&0x0100 != 0 {
			m.prg0 = outer | int(m.prgReg)
			m.prg1 = outer | 0x07
		} else {
			b := outer | int(m.prgReg&0x06)
			m.prg0, m.prg1 = b, b|1
		}
	} else {
		p := outer | int(m.prgReg)
		m.prg0, m.prg1 = p, p
	}
	m.mirror = hvMirror(m.mode&0x01 == 0)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper221) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper221) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper221) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper221) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Mapper221) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.mode)
	s.Regs[1] = byte(m.mode >> 8)
	s.Regs[2] = m.prgReg
	s.Regs[3] = byte(m.prg0)
	s.Regs[4] = byte(m.prg1)
	s.Regs[5] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper221) Restore(s *State) {
	m.restoreRAM(s)
	m.mode = uint16(s.Regs[0]) | uint16(s.Regs[1])<<8
	m.prgReg = s.Regs[2]
	m.prg0 = int(s.Regs[3])
	m.prg1 = int(s.Regs[4])
	m.mirror = cartridge.Mirroring(s.Regs[5])
}

// MagicKidGooGoo (mapper 190): two 16 KiB PRG banks (a low and a high
// half selected by the register range) and four 2 KiB CHR windows.
type MagicKidGooGoo struct {
	base
	prg0    byte
	chrBank [4]byte
	mirror  cartridge.Mirroring
}

// NewMagicKidGooGoo wires the board.
func NewMagicKidGooGoo(c *cartridge.Cartridge) *MagicKidGooGoo {
	return &MagicKidGooGoo{base: makeBase(c), mirror: cartridge.Vertical}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MagicKidGooGoo) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000 && addr <= 0x9FFF:
		m.prg0 = v & 0x07
	case addr >= 0xC000 && addr <= 0xDFFF:
		m.prg0 = (v & 0x07) | 0x08
	case addr&0xA000 == 0xA000:
		m.chrBank[addr&0x03] = v
	case addr >= 0x6000 && addr < 0x8000:
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MagicKidGooGoo) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF] // fixed last 16 KiB
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prg0), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MagicKidGooGoo) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank[addr>>11&3]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MagicKidGooGoo) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank[addr>>11&3]), 0x800, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *MagicKidGooGoo) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *MagicKidGooGoo) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prg0
	copy(s.Regs[1:5], m.chrBank[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *MagicKidGooGoo) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = s.Regs[0]
	copy(m.chrBank[:], s.Regs[1:5])
}

// NtdecTc112 (mapper 193): one switchable 8 KiB PRG window plus three
// fixed top banks, a 4 KiB CHR window (two 2 KiB regs) and two 2 KiB CHR
// windows.
type NtdecTc112 struct {
	base
	prg0    byte
	chrBank [4]byte // four 2 KiB CHR windows
}

// NewNtdecTc112 wires the board.
func NewNtdecTc112(c *cartridge.Cartridge) *NtdecTc112 {
	return &NtdecTc112{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *NtdecTc112) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		switch addr & 0x03 {
		case 0:
			// 4 KiB CHR: two consecutive 2 KiB windows.
			m.chrBank[0] = v >> 1
			m.chrBank[1] = (v >> 1) + 1
		case 1:
			m.chrBank[2] = v >> 1
		case 2:
			m.chrBank[3] = v >> 1
		case 3:
			m.prg0 = v
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *NtdecTc112) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.win(m.prg, -2, 0x2000)[addr&0x1FFF]
	case addr >= 0xA000:
		return m.win(m.prg, -3, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prg0), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *NtdecTc112) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank[addr>>11&3]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *NtdecTc112) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank[addr>>11&3]), 0x800, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *NtdecTc112) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prg0
	copy(s.Regs[1:5], m.chrBank[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *NtdecTc112) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = s.Regs[0]
	copy(m.chrBank[:], s.Regs[1:5])
}

// ActionEnterprises (mapper 228, Action 52 / Cheetahmen II): the write
// address carries a chip-select, a wide PRG bank (16/32 KiB) and CHR high
// bits; the data byte supplies the CHR low bits.
type ActionEnterprises struct{ dualPRG16 }

// NewActionEnterprises wires the board.
func NewActionEnterprises(c *cartridge.Cartridge) *ActionEnterprises {
	m := &ActionEnterprises{dualPRG16{base: makeBase(c)}}
	m.decode(0x8000, 0)
	return m
}

func (m *ActionEnterprises) decode(addr uint16, v byte) {
	chip := (addr >> 11) & 0x03
	if chip == 3 {
		chip = 2
	}
	prg := int(addr>>6)&0x1F | int(chip)<<5
	if addr&0x20 != 0 {
		m.prg0, m.prg1 = prg, prg
	} else {
		m.prg0, m.prg1 = prg&0xFE, prg&0xFE|1
	}
	m.chrBank = int(addr&0x0F)<<2 | int(v&0x03)
	m.mirror = hvMirror(addr&0x2000 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *ActionEnterprises) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr, v)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *ActionEnterprises) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *ActionEnterprises) Restore(s *State) { m.restoreDual(s) }
