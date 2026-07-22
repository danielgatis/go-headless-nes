package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// txcChip is the TXC protection ASIC (JV001 and the plain variant) shared
// by mappers 36/132/172/173. It is a tiny accumulator/inverter unit the
// games poll for a magic sequence; the board wires its output to the PRG/
// CHR banks. Ported from the reference emulator.
type txcChip struct {
	accumulator byte
	inverter    byte
	staging     byte
	output      byte
	increase    bool
	yFlag       bool
	invert      bool

	mask    byte
	isJV001 bool
}

func newTxcChip(isJV001 bool) txcChip {
	c := txcChip{isJV001: isJV001}
	if isJV001 {
		c.mask = 0x0F
		c.invert = true
	} else {
		c.mask = 0x07
	}
	return c
}

func (c *txcChip) read() byte {
	inv := byte(0)
	if c.invert {
		inv = 0xFF
	}
	value := (c.accumulator & c.mask) | ((c.inverter ^ inv) &^ c.mask)
	c.yFlag = !c.invert || (value&0x10 != 0)
	return value
}

func (c *txcChip) write(addr uint16, value byte) {
	if addr < 0x8000 {
		switch addr & 0xE103 {
		case 0x4100:
			if c.increase {
				c.accumulator++
			} else {
				inv := byte(0)
				if c.invert {
					inv = 0xFF
				}
				c.accumulator = ((c.accumulator &^ c.mask) | (c.staging & c.mask)) ^ inv
			}
		case 0x4101:
			c.invert = value&0x01 != 0
		case 0x4102:
			c.staging = value & c.mask
			c.inverter = value &^ c.mask
		case 0x4103:
			c.increase = value&0x01 != 0
		}
	} else {
		if c.isJV001 {
			c.output = (c.accumulator & 0x0F) | (c.inverter & 0xF0)
		} else {
			c.output = (c.accumulator & 0x0F) | ((c.inverter & 0x08) << 1)
		}
	}
	c.yFlag = !c.invert || (value&0x10 != 0)
}

func (c *txcChip) save(r []byte) {
	r[0] = c.accumulator
	r[1] = c.inverter
	r[2] = c.staging
	r[3] = c.output
	r[4] = boolByte(c.increase)
	r[5] = boolByte(c.yFlag)
	r[6] = boolByte(c.invert)
}

func (c *txcChip) restore(r []byte) {
	c.accumulator = r[0]
	c.inverter = r[1]
	c.staging = r[2]
	c.output = r[3]
	c.increase = r[4] != 0
	c.yFlag = r[5] != 0
	c.invert = r[6] != 0
}

// Txc22000 (mapper 36): the plain TXC chip drives a 32 KiB PRG bank; a
// $4200-region write sets an 8 KiB CHR bank.
type Txc22000 struct {
	base
	txc     txcChip
	chrBank byte
}

// NewTxc22000 wires the board.
func NewTxc22000(c *cartridge.Cartridge) *Txc22000 {
	return &Txc22000{base: makeBase(c), txc: newTxcChip(false)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Txc22000) WritePRG(addr uint16, v byte) {
	if addr&0xF200 == 0x4200 {
		m.chrBank = v
	}
	m.txc.write(addr, (v>>4)&0x03)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Txc22000) ReadPRG(addr uint16) byte {
	if addr >= 0x4100 && addr < 0x6000 {
		out := m.openBus()
		if addr&0x103 == 0x100 {
			out = (out & 0xCF) | ((m.txc.read() << 4) & 0x30)
		}
		return out
	}
	if addr >= 0x8000 {
		return window(m.prg, int(m.txc.output&0x03), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Txc22000) ReadCHR(addr uint16) byte { return m.chrRead(int(m.chrBank), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Txc22000) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.chrBank), 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Txc22000) Save(s *State) {
	m.saveRAM(s)
	m.txc.save(s.Regs[0:7])
	s.Regs[7] = m.chrBank
}

// Restore loads the board's mapper-specific state from s.
func (m *Txc22000) Restore(s *State) {
	m.restoreRAM(s)
	m.txc.restore(s.Regs[0:7])
	m.chrBank = s.Regs[7]
}

// Txc22211A (mapper 132): the plain chip drives a 16 KiB-granular PRG bit
// and a 2-bit CHR bank.
type Txc22211A struct {
	base
	txc txcChip
}

// NewTxc22211A wires the board.
func NewTxc22211A(c *cartridge.Cartridge) *Txc22211A {
	return &Txc22211A{base: makeBase(c), txc: newTxcChip(false)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Txc22211A) WritePRG(addr uint16, v byte) { m.txc.write(addr, v&0x0F) }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Txc22211A) ReadPRG(addr uint16) byte {
	if addr >= 0x4020 && addr < 0x6000 {
		out := m.openBus()
		if addr&0x100 == 0x100 {
			out = (out & 0xF0) | (m.txc.read() & 0x0F)
		}
		return out
	}
	if addr >= 0x8000 {
		return window(m.prg, int(m.txc.output>>2)&0x01, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Txc22211A) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.txc.output&0x03), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Txc22211A) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.txc.output&0x03), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Txc22211A) Save(s *State) { m.saveRAM(s); m.txc.save(s.Regs[0:7]) }

// Restore loads the board's mapper-specific state from s.
func (m *Txc22211A) Restore(s *State) { m.restoreRAM(s); m.txc.restore(s.Regs[0:7]) }

// txcConvert172 scrambles the data bus of the JV001-based boards
// (mappers 172/173).
func txcConvert172(v byte) byte {
	return (v&0x01)<<5 | (v&0x02)<<3 | (v&0x04)<<1 | (v&0x08)>>1 | (v&0x10)>>3 | (v&0x20)>>5
}

// Txc22211B (mapper 172): the JV001 chip drives an 8 KiB CHR bank with a
// scrambled data bus and register-controlled mirroring.
type Txc22211B struct {
	base
	txc    txcChip
	mirror cartridge.Mirroring
}

// NewTxc22211B wires the board.
func NewTxc22211B(c *cartridge.Cartridge) *Txc22211B {
	return &Txc22211B{base: makeBase(c), txc: newTxcChip(true), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Txc22211B) WritePRG(addr uint16, v byte) {
	m.txc.write(addr, txcConvert172(v))
	if addr >= 0x8000 {
		m.mirror = hvMirror(m.txc.invert)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Txc22211B) ReadPRG(addr uint16) byte {
	if addr >= 0x4020 && addr < 0x6000 {
		out := m.openBus()
		if addr&0x103 == 0x100 {
			out = (out & 0xC0) | txcConvert172(m.txc.read())
		}
		return out
	}
	if addr >= 0x8000 {
		return window(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Txc22211B) ReadCHR(addr uint16) byte { return m.chrRead(int(m.txc.output), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Txc22211B) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.txc.output), 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Txc22211B) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Txc22211B) Save(s *State) {
	m.saveRAM(s)
	m.txc.save(s.Regs[0:7])
	s.Regs[7] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Txc22211B) Restore(s *State) {
	m.restoreRAM(s)
	m.txc.restore(s.Regs[0:7])
	m.mirror = cartridge.Mirroring(s.Regs[7])
}

// Txc22211C (mapper 173): like 22211A but the chip's Y flag gates CHR,
// selecting the bank from the output bits (and disabling CHR when Y is
// clear on 8 KiB carts).
type Txc22211C struct {
	Txc22211A
}

// NewTxc22211C wires the board.
func NewTxc22211C(c *cartridge.Cartridge) *Txc22211C {
	return &Txc22211C{Txc22211A{base: makeBase(c), txc: newTxcChip(false)}}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Txc22211C) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, 0, 0x8000)[addr&0x7FFF] // PRG fixed to bank 0
	}
	return m.Txc22211A.ReadPRG(addr)
}

func (m *Txc22211C) chrBank() int {
	if len(m.chr) > 0x2000 {
		y := 0
		if m.txc.yFlag {
			y = 0x02
		}
		return int(m.txc.output&0x01) | y | int(m.txc.output&0x02)<<1
	}
	return 0
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Txc22211C) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank(), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Txc22211C) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank(), 0x2000, addr, v) }
