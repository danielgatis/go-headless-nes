package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// BMC (multi-game) boards ported from the reference emulator. These latch bank state from
// the write address and/or value and switch large PRG/CHR windows.

// Caltron41 (mapper 41): a $6000 register sets a 32 KiB PRG bank, the CHR
// high bits and mirroring; an $8000 write sets the CHR low bits, but only
// while the PRG bank is 4-7.
type Caltron41 struct {
	base
	prgBank byte
	chrBank byte
	mirror  cartridge.Mirroring
}

// NewCaltron41 wires the board.
func NewCaltron41(c *cartridge.Cartridge) *Caltron41 {
	return &Caltron41{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Caltron41) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x6000 && addr <= 0x67FF:
		m.prgBank = byte(addr & 0x07)
		m.chrBank = m.chrBank&0x03 | byte((addr>>1)&0x0C)
		m.mirror = hvMirror(addr&0x20 == 0)
	case addr >= 0x8000:
		if m.prgBank >= 4 {
			m.chrBank = m.chrBank&0x0C | v&0x03
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Caltron41) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Caltron41) ReadCHR(addr uint16) byte { return m.chrRead(int(m.chrBank), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Caltron41) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.chrBank), 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Caltron41) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Caltron41) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
	s.Regs[2] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Caltron41) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.mirror = cartridge.Mirroring(s.Regs[2])
}

// Bmc255 (mapper 255): a purely address-latched 110-in-1 board with
// 16/32 KiB PRG windows and an 8 KiB CHR bank.
type Bmc255 struct{ dualPRG16 }

// NewBmc255 wires the board.
func NewBmc255(c *cartridge.Cartridge) *Bmc255 {
	m := &Bmc255{dualPRG16{base: makeBase(c)}}
	m.decode(0x8000)
	return m
}

func (m *Bmc255) decode(addr uint16) {
	prgBit := 1
	if addr&0x1000 != 0 {
		prgBit = 0
	}
	bank := int(addr>>8)&0x40 | int(addr>>6)&0x3F
	m.prg0 = bank &^ prgBit
	m.prg1 = bank | prgBit
	m.chrBank = int(addr>>8)&0x40 | int(addr&0x3F)
	m.mirror = hvMirror(addr&0x2000 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc255) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Bmc255) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Bmc255) Restore(s *State) { m.restoreDual(s) }

// Bb (mapper 108, Bubble Bobble pirate): a switchable 8 KiB PRG-ROM
// window at $6000 (the last 32 KiB fixed at $8000) and an 8 KiB CHR bank.
type Bb struct {
	base
	prgReg byte
	chrReg byte
}

// NewBb wires the board.
func NewBb(c *cartridge.Cartridge) *Bb {
	return &Bb{base: makeBase(c), prgReg: 0xFF}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bb) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	if addr&0x9000 == 0x8000 || addr >= 0xF000 {
		m.prgReg = v
		m.chrReg = v
	} else {
		m.chrReg = v & 0x01
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bb) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return m.win(m.prg, -4+int((addr-0x8000)>>13), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.win(m.prg, int(m.prgReg), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bb) ReadCHR(addr uint16) byte { return m.chrRead(int(m.chrReg), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bb) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.chrReg), 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Bb) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg
	s.Regs[1] = m.chrReg
}

// Restore loads the board's mapper-specific state from s.
func (m *Bb) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg = s.Regs[0]
	m.chrReg = s.Regs[1]
}

// Bmc51 (mapper 51, 11-in-1): a mode/bank pair drives four 8 KiB PRG
// windows (or two 16 KiB windows) plus a PRG-ROM window at $6000.
type Bmc51 struct {
	base
	bank byte
	mode byte
}

// NewBmc51 wires the board.
func NewBmc51(c *cartridge.Cartridge) *Bmc51 {
	return &Bmc51{base: makeBase(c), mode: 1}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc51) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x6000 && addr <= 0x7FFF:
		m.mode = (v>>3)&0x02 | (v>>1)&0x01
	case addr >= 0xC000 && addr <= 0xDFFF:
		m.bank = v & 0x0F
		m.mode = (v>>3)&0x02 | m.mode&0x01
	case addr >= 0x8000:
		m.bank = v & 0x0F
	default:
		return
	}
}

// prg8 returns the 8 KiB PRG bank for a CPU slot 0-3 ($8000..$E000).
func (m *Bmc51) prg8(slot int) int {
	if m.mode&0x01 != 0 {
		return int(m.bank)<<2 | slot
	}
	// 16 KiB mode: slots 0-1 use (bank<<2|mode), slots 2-3 use bank<<2|0x0E.
	if slot < 2 {
		return (int(m.bank)<<2 | int(m.mode)) + slot
	}
	return (int(m.bank)<<2 | 0x0E) + (slot - 2)
}

func (m *Bmc51) prg6000() int {
	if m.mode&0x01 != 0 {
		return 0x23 | int(m.bank)<<2
	}
	return 0x2F | int(m.bank)<<2
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc51) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := int((addr - 0x8000) >> 13)
		return m.win(m.prg, m.prg8(slot), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.win(m.prg, m.prg6000(), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc51) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc51) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc51) Mirroring() cartridge.Mirroring {
	if m.mode == 0x03 {
		return cartridge.Horizontal
	}
	return cartridge.Vertical
}

// Save writes the board's mapper-specific state into s.
func (m *Bmc51) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bank
	s.Regs[1] = m.mode
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc51) Restore(s *State) {
	m.restoreRAM(s)
	m.bank = s.Regs[0]
	m.mode = s.Regs[1]
}

// bmc235Config is the reference's per-ROM-size table: for each PRG size mode and
// each of four address groups, an outer-bank OR value and an open-bus
// flag.
var bmc235Config = [4][4][2]byte{
	{{0x00, 0}, {0x00, 1}, {0x00, 1}, {0x00, 1}},
	{{0x00, 0}, {0x00, 1}, {0x20, 0}, {0x00, 1}},
	{{0x00, 0}, {0x00, 1}, {0x20, 0}, {0x40, 0}},
	{{0x00, 0}, {0x20, 0}, {0x40, 0}, {0x60, 0}},
}

// Bmc235 (mapper 235, "Golden Game" 260-in-1): the write address selects
// a 16/32 KiB PRG bank through a per-size config table; some address
// groups leave the bus floating. CHR is 8 KiB RAM.
type Bmc235 struct {
	base
	prg0     int
	prg1     int
	floating bool
	mirror   cartridge.Mirroring
}

// NewBmc235 wires the board.
func NewBmc235(c *cartridge.Cartridge) *Bmc235 {
	m := &Bmc235{base: makeBase(c), mirror: c.Mirroring}
	m.prg0, m.prg1 = 0, 1
	return m
}

func (m *Bmc235) sizeMode() int {
	switch len(m.prg) / 0x4000 {
	case 64:
		return 0
	case 128:
		return 1
	case 256:
		return 2
	default:
		return 3
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc235) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	if addr&0x0400 != 0 {
		m.mirror = cartridge.SingleLow
	} else {
		m.mirror = hvMirror(addr&0x2000 == 0)
	}

	cfg := bmc235Config[m.sizeMode()][addr>>8&0x03]
	bank := int(cfg[0]) | int(addr&0x1F)
	m.floating = false
	switch {
	case cfg[1] != 0:
		m.floating = true
	case addr&0x800 != 0:
		b := bank<<1 | int(addr>>12&0x01)
		m.prg0, m.prg1 = b, b
	default:
		m.prg0, m.prg1 = bank<<1, bank<<1|1
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc235) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		if m.floating {
			return m.openBus()
		}
		if addr < 0xC000 {
			return m.win(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
		}
		return m.win(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc235) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc235) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc235) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc235) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.prg1)
	s.Regs[2] = byte(m.prg0 >> 8)
	s.Regs[3] = byte(m.prg1 >> 8)
	s.Regs[4] = boolByte(m.floating)
	s.Regs[5] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc235) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0]) | int(s.Regs[2])<<8
	m.prg1 = int(s.Regs[1]) | int(s.Regs[3])<<8
	m.floating = s.Regs[4] != 0
	m.mirror = cartridge.Mirroring(s.Regs[5])
}

// Bmc80013B (mappers 271, 274): two registers selected by the write
// address drive a 16 KiB switchable window at $8000 and a second 16 KiB
// window at $C000, with an outer bank in the higher register.
type Bmc80013B struct {
	base
	regs   [2]byte
	mode   byte
	mirror cartridge.Mirroring
}

// NewBmc80013B wires the board.
func NewBmc80013B(c *cartridge.Cartridge) *Bmc80013B {
	return &Bmc80013B{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc80013B) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	reg := byte(addr>>13) & 0x03
	if reg == 0 {
		m.regs[0] = v
	} else {
		m.regs[1] = v
		m.mode = reg
	}
	m.mirror = hvMirror(m.regs[0]&0x10 != 0) // bit4 set -> Vertical
}

func (m *Bmc80013B) prg0() int {
	if m.mode&0x02 != 0 {
		return int(m.regs[0]&0x0F) | int(m.regs[1]&0x70)
	}
	return int(m.regs[0] & 0x03)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc80013B) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, int(m.regs[1]&0x7F), 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prg0(), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc80013B) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc80013B) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc80013B) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc80013B) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.regs[0]
	s.Regs[1] = m.regs[1]
	s.Regs[2] = m.mode
	s.Regs[3] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc80013B) Restore(s *State) {
	m.restoreRAM(s)
	m.regs[0] = s.Regs[0]
	m.regs[1] = s.Regs[1]
	m.mode = s.Regs[2]
	m.mirror = cartridge.Mirroring(s.Regs[3])
}

// Bmc12in1 (mapper 331): two bank registers and a mode register at
// $A000/$C000/$E000 drive two 4 KiB CHR windows and a 16/32 KiB PRG
// window, all offset by a 3-bit outer block.
type Bmc12in1 struct {
	base
	regs   [2]byte
	mode   byte
	mirror cartridge.Mirroring
}

// NewBmc12in1 wires the board.
func NewBmc12in1(c *cartridge.Cartridge) *Bmc12in1 {
	return &Bmc12in1{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc12in1) WritePRG(addr uint16, v byte) {
	switch addr & 0xE000 {
	case 0xA000:
		m.regs[0] = v
	case 0xC000:
		m.regs[1] = v
	case 0xE000:
		m.mode = v & 0x0F
	default:
		if addr >= 0x6000 && addr < 0x8000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	m.mirror = hvMirror(m.mode&0x04 == 0)
}

func (m *Bmc12in1) block() int { return int(m.mode&0x03) << 3 }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc12in1) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			return m.readPRGRAM(addr)
		}
		return m.openBus()
	}
	bank := m.block()
	var page int
	if m.mode&0x08 != 0 { // 32 KiB
		base := bank | int(m.regs[0]&0x06)
		page = base + int(addr>>13&1)
	} else { // 16 KiB switchable + fixed top of block
		if addr < 0xC000 {
			page = bank | int(m.regs[0]&0x07)
		} else {
			page = bank | 0x07
		}
	}
	return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *Bmc12in1) chrBank(addr uint16) int {
	bank := m.block()
	if addr < 0x1000 {
		return int(m.regs[0]>>3) | bank<<2
	}
	return int(m.regs[1]>>3) | bank<<2
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc12in1) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrBank(addr), 0x1000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Bmc12in1) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrBank(addr), 0x1000, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc12in1) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc12in1) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.regs[0]
	s.Regs[1] = m.regs[1]
	s.Regs[2] = m.mode
	s.Regs[3] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc12in1) Restore(s *State) {
	m.restoreRAM(s)
	m.regs[0] = s.Regs[0]
	m.regs[1] = s.Regs[1]
	m.mode = s.Regs[2]
	m.mirror = cartridge.Mirroring(s.Regs[3])
}
