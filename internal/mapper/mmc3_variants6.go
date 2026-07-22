package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Mapper91 (mapper 91, JY/pirate): a $6000-$7FFF board that reuses the
// MMC3 scanline IRQ but has its own simple banking - four 1 KiB CHR
// windows via $6000-$6003, two switchable 8 KiB PRG windows via
// $7000/$7001 (the last two fixed), and $7002/$7003 driving the IRQ.
type Mapper91 struct {
	MMC3

	prgReg [2]byte
	chrReg [4]byte
}

// NewMapper91 wires the board.
func NewMapper91(c *cartridge.Cartridge) *Mapper91 {
	return &Mapper91{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper91) WritePRG(addr uint16, v byte) {
	// The board only decodes $6000-$7FFF; $8000+ is inert.
	if addr < 0x6000 || addr >= 0x8000 {
		return
	}
	switch addr & 0x7003 {
	case 0x6000:
		m.chrReg[0] = v
	case 0x6001:
		m.chrReg[1] = v
	case 0x6002:
		m.chrReg[2] = v
	case 0x6003:
		m.chrReg[3] = v
	case 0x7000:
		m.prgReg[0] = v & 0x0F
	case 0x7001:
		m.prgReg[1] = v & 0x0F
	case 0x7002:
		// IRQ disable + acknowledge.
		m.writeRegister(0xE000, v)
	case 0x7003:
		// Reload the counter with a latch of 7 and enable IRQs.
		m.writeRegister(0xC000, 0x07)
		m.writeRegister(0xC001, v)
		m.writeRegister(0xE001, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper91) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -2+int((addr-0xC000)>>13), 0x2000)[addr&0x1FFF]
	case addr >= 0xA000:
		return window(m.prg, int(m.prgReg[1]), 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgReg[0]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper91) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>11&3]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper91) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>11&3]), 0x800, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper91) Mirroring() cartridge.Mirroring { return m.mirroring }

// Save writes the board's mapper-specific state into s.
func (m *Mapper91) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.prgReg[0]
	s.Regs[18] = m.prgReg[1]
	copy(s.Regs[19:23], m.chrReg[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper91) Restore(s *State) {
	m.MMC3.Restore(s)
	m.prgReg[0] = s.Regs[17]
	m.prgReg[1] = s.Regs[18]
	copy(m.chrReg[:], s.Regs[19:23])
}

// MMC3Kof97 (mapper 263, KOF97): an MMC3 whose register data bus and a
// few register addresses are scrambled. We descramble both and forward to
// the stock MMC3.
type MMC3Kof97 struct {
	MMC3
}

// NewMMC3Kof97 wires the board.
func NewMMC3Kof97(c *cartridge.Cartridge) *MMC3Kof97 {
	return &MMC3Kof97{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Kof97) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		m.MMC3.WritePRG(addr, v)
		return
	}
	v = v&0xD8 | (v&0x20)>>4 | (v&0x04)<<3 | (v&0x02)>>1 | (v&0x01)<<2
	switch addr {
	case 0x9000:
		addr = 0x8001
	case 0xD000:
		addr = 0xC001
	case 0xF000:
		addr = 0xE001
	}
	m.MMC3.WritePRG(addr, v)
}

// MMC3BmcF15 (mapper 259, BMC-F-15): an MMC3 multicart with a $6000 outer
// register (gated by the $A001 WRAM-enable bit) that forces a 16/32 KiB
// PRG window.
type MMC3BmcF15 struct {
	MMC3

	exReg byte
}

// NewMMC3BmcF15 wires the board.
func NewMMC3BmcF15(c *cartridge.Cartridge) *MMC3BmcF15 {
	return &MMC3BmcF15{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3BmcF15) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if m.ramEnabled { // $A001 bit 7 gates the register
			m.exReg = v & 0x0F
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3BmcF15) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	bank := int(m.exReg & 0x0F)
	mode := int(m.exReg&0x08) >> 3
	mask := ^mode & 0x0F
	// Two 16 KiB windows built from (bank & mask); in 16 KiB mode both
	// halves address the same bank.
	var page int
	if addr < 0xC000 {
		page = (bank & mask) << 1
	} else {
		page = ((bank & mask) | mode) << 1
	}
	page |= int(addr>>13) & 1
	return window(m.prg, page, 0x2000)[addr&0x1FFF]
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3BmcF15) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.exReg }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3BmcF15) Restore(s *State) { m.MMC3.Restore(s); m.exReg = s.Regs[17] }

// MMC3199 (mapper 199, Waixing): an MMC3 with two extra PRG registers
// mapping the normally-fixed $C000/$E000 slots, 4-way mirroring, and a CHR
// map where 2 KiB pages below 8 address the 8 KiB CHR RAM and the rest
// address ROM. the reference emulator flags this board as possibly incorrect; this is a
// faithful port of that reference.
type MMC3199 struct {
	MMC3

	exReg     [4]byte
	mirrorReg byte // raw low 2 bits of the last $A000 write (4-way)
}

// NewMMC3199 wires the board.
func NewMMC3199(c *cartridge.Cartridge) *MMC3199 {
	m := &MMC3199{MMC3: *NewMMC3(c)}
	m.exReg = [4]byte{0xFE, 0xFF, 0x01, 0x03}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3199) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		if addr&0xE001 == 0x8001 && m.bankSelect&0x08 != 0 {
			m.exReg[m.bankSelect&0x03] = v
			return
		}
		if addr&0xE001 == 0xA000 {
			m.mirrorReg = v & 0x03
		}
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3199) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	switch addr >> 13 & 3 {
	case 2: // $C000 -> exReg0
		return window(m.prg, int(m.exReg[0]), 0x2000)[addr&0x1FFF]
	case 3: // $E000 -> exReg1
		return window(m.prg, int(m.exReg[1]), 0x2000)[addr&0x1FFF]
	default:
		return window(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	}
}

// chr2K resolves the 2 KiB CHR window for a PPU address: windows 0-3 use
// R0, exReg2, R1, exReg3. A page below 8 (2 KiB units) selects CHR RAM.
func (m *MMC3199) chr2K(addr uint16) (page int, ram bool) {
	win := addr >> 11 & 3
	switch win {
	case 0:
		page = int(m.banks[0])
	case 1:
		page = int(m.exReg[2])
	case 2:
		page = int(m.banks[1])
	default:
		page = int(m.exReg[3])
	}
	return page, page < 8
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3199) ReadCHR(addr uint16) byte {
	page, ram := m.chr2K(addr)
	if ram {
		return window(m.chrRAM[:], page, 0x800)[addr&0x7FF]
	}
	return window(m.chr, page, 0x800)[addr&0x7FF]
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3199) WriteCHR(addr uint16, v byte) {
	page, ram := m.chr2K(addr)
	if ram {
		window(m.chrRAM[:], page, 0x800)[addr&0x7FF] = v
	}
}

// Mirroring reports the board's current nametable mirroring.
func (m *MMC3199) Mirroring() cartridge.Mirroring {
	switch m.mirrorReg {
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

// Save writes the board's mapper-specific state into s.
func (m *MMC3199) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:21], m.exReg[:])
	s.Regs[21] = m.mirrorReg
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3199) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.exReg[:], s.Regs[17:21])
	m.mirrorReg = s.Regs[21]
}

// MMC3MaliSB (mapper 325): an MMC3 clone whose register-select address and
// the PRG/CHR bank bits are permuted. We descramble the write address and
// apply the fixed bank-bit permutation on read.
type MMC3MaliSB struct {
	MMC3
}

// NewMMC3MaliSB wires the board.
func NewMMC3MaliSB(c *cartridge.Cartridge) *MMC3MaliSB {
	return &MMC3MaliSB{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3MaliSB) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		m.MMC3.WritePRG(addr, v)
		return
	}
	if addr >= 0xC000 {
		addr = addr&0xFFFE | (addr>>2)&0x01 | (addr>>3)&0x01
	} else {
		addr = addr&0xFFFE | (addr>>3)&0x01
	}
	m.MMC3.WritePRG(addr, v)
}

func scrambleMaliPRG(p int) int {
	return (p & 0x03) | (p&0x08)>>1 | (p&0x04)<<1
}

func scrambleMaliCHR(p int) int {
	return (p & 0xDD) | (p&0x20)>>4 | (p&0x02)<<4
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3MaliSB) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	return window(m.prg, scrambleMaliPRG(p), 0x2000)[addr&0x1FFF]
}

func (m *MMC3MaliSB) chrPage(addr uint16) int { return scrambleMaliCHR(m.chrPage1K(addr)) }

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3MaliSB) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3MaliSB) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// MMC3StreetHeroes (mapper 262): an MMC3 with a $4100 register that adds a
// per-window CHR high bit (or forces the whole CHR map to 8 KiB CHR RAM),
// plus a reset switch read back at $4100. The reset switch toggles on each
// soft reset.
type MMC3StreetHeroes struct {
	MMC3

	exReg       byte
	resetSwitch byte
}

// NewMMC3StreetHeroes wires the board.
func NewMMC3StreetHeroes(c *cartridge.Cartridge) *MMC3StreetHeroes {
	return &MMC3StreetHeroes{MMC3: *NewMMC3(c)}
}

// Reset returns the board to its power-on banking.
func (m *MMC3StreetHeroes) Reset(soft bool) {
	if soft {
		m.resetSwitch ^= 0xFF
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3StreetHeroes) WritePRG(addr uint16, v byte) {
	if addr == 0x4100 {
		m.exReg = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3StreetHeroes) ReadPRG(addr uint16) byte {
	if addr == 0x4100 {
		return m.resetSwitch
	}
	return m.MMC3.ReadPRG(addr)
}

// chrHighBit returns the CHR A-high bit contributed by exReg for a window.
func (m *MMC3StreetHeroes) chrHighBit(win int) int {
	switch win {
	case 0, 1:
		return int(m.exReg&0x08) << 5
	case 2, 3:
		return int(m.exReg&0x04) << 6
	case 4, 5:
		return int(m.exReg&0x01) << 8
	default:
		return int(m.exReg&0x02) << 7
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3StreetHeroes) ReadCHR(addr uint16) byte {
	if m.exReg&0x40 != 0 {
		return m.chrRAM[addr&0x1FFF] // whole CHR map is 8 KiB CHR RAM
	}
	a := addr
	if m.bankSelect&0x80 != 0 {
		a ^= 0x1000
	}
	page := m.chrPage1K(addr) | m.chrHighBit(int(a>>10&7))
	return window(m.chr, page, 0x400)[addr&0x3FF]
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3StreetHeroes) WriteCHR(addr uint16, v byte) {
	if m.exReg&0x40 != 0 {
		m.chrRAM[addr&0x1FFF] = v
	}
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3StreetHeroes) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.exReg
	s.Regs[18] = m.resetSwitch
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3StreetHeroes) Restore(s *State) {
	m.MMC3.Restore(s)
	m.exReg = s.Regs[17]
	m.resetSwitch = s.Regs[18]
}

// BmcGn45 (mapper 366): an MMC3 multicart with a $6000-region block-select
// register that adds high PRG (bits 4-5) and CHR (block<<3) bits, gated by
// a WRAM-enable bit.
type BmcGn45 struct {
	MMC3

	block  byte
	wramOn bool
}

// NewBmcGn45 wires the board.
func NewBmcGn45(c *cartridge.Cartridge) *BmcGn45 {
	return &BmcGn45{MMC3: *NewMMC3(c)}
}

// Reset returns the board to its power-on banking.
func (m *BmcGn45) Reset(soft bool) {
	if soft {
		m.block = 0
		m.wramOn = false
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *BmcGn45) WritePRG(addr uint16, v byte) {
	switch {
	case addr < 0x7000:
		if !m.wramOn {
			m.block = byte(addr) & 0x30
			m.wramOn = addr&0x80 != 0
		} else {
			m.writePRGRAM(addr, v)
		}
	case addr < 0x8000:
		if !m.wramOn {
			m.block = v & 0x30
		} else {
			m.writePRGRAM(addr, v)
		}
	default:
		m.MMC3.WritePRG(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *BmcGn45) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	page := p&0x0F | int(m.block)
	return window(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *BmcGn45) chrPage(addr uint16) int {
	return m.chrPage1K(addr)&0x7F | int(m.block)<<3
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *BmcGn45) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *BmcGn45) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *BmcGn45) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = m.block
	s.Regs[18] = boolByte(m.wramOn)
}

// Restore loads the board's mapper-specific state from s.
func (m *BmcGn45) Restore(s *State) {
	m.MMC3.Restore(s)
	m.block = s.Regs[17]
	m.wramOn = s.Regs[18] != 0
}

// Unl158B (mapper 258): an MMC3 clone with a $5000-region protection
// register that can force a 16/32 KiB PRG window, plus a protection-LUT
// read back. PRG banks are masked to 4 bits in the normal mode.
type Unl158B struct {
	MMC3

	reg byte
}

// NewUnl158B wires the board.
func NewUnl158B(c *cartridge.Cartridge) *Unl158B {
	return &Unl158B{MMC3: *NewMMC3(c)}
}

var unl158bProtLut = [8]byte{0x00, 0x00, 0x00, 0x01, 0x02, 0x04, 0x0F, 0x00}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Unl158B) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x6000 {
		if addr&0x07 == 0 {
			m.reg = v
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Unl158B) ReadPRG(addr uint16) byte {
	if addr >= 0x5000 && addr < 0x6000 {
		return m.openBus() | unl158bProtLut[addr&0x07]
	}
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	if m.reg&0x80 != 0 {
		bank := int(m.reg & 0x07)
		var page int
		if m.reg&0x20 != 0 { // 32 KiB
			page = (bank&0x06)<<1 | int(addr>>13&3)
		} else { // 16 KiB mirrored
			page = bank<<1 | int(addr>>13&1)
		}
		return window(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	return window(m.prg, p&0x0F, 0x2000)[addr&0x1FFF]
}

// Save writes the board's mapper-specific state into s.
func (m *Unl158B) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *Unl158B) Restore(s *State) { m.MMC3.Restore(s); m.reg = s.Regs[17] }

// MMC3Bmc411120C (mapper 287): an MMC3 multicart with a $6000-region
// register (latched from the low address byte) adding CHR bits 7-8 and
// PRG bits 4-5, or forcing a fixed 32 KiB window. DIP handling is taken
// as 0.
type MMC3Bmc411120C struct {
	MMC3

	exReg byte
}

// NewMMC3Bmc411120C wires the board.
func NewMMC3Bmc411120C(c *cartridge.Cartridge) *MMC3Bmc411120C {
	return &MMC3Bmc411120C{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3Bmc411120C) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.exReg = byte(addr)
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3Bmc411120C) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	if m.exReg&0x08 != 0 { // forced 32 KiB (DIP=0)
		page := (int(m.exReg>>4)&0x03|0x0C)<<2 | int(addr>>13&3)
		return window(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	page := p&0x0F | int(m.exReg&0x03)<<4
	return window(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *MMC3Bmc411120C) chrPage(addr uint16) int {
	return m.chrPage1K(addr) | int(m.exReg&0x03)<<7
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3Bmc411120C) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3Bmc411120C) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3Bmc411120C) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.exReg }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3Bmc411120C) Restore(s *State) { m.MMC3.Restore(s); m.exReg = s.Regs[17] }

// mmc3_208ProtLut is the 256-byte protection table Street Fighter IV
// (mapper 208) polls: a value written to $5000-$57FF indexes it, and the
// XOR of the result descrambles reads from $5800-$5FFF.
var mmc3_208ProtLut = [256]byte{
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x49, 0x19, 0x09, 0x59, 0x49, 0x19, 0x09,
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x51, 0x41, 0x11, 0x01, 0x51, 0x41, 0x11, 0x01,
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x49, 0x19, 0x09, 0x59, 0x49, 0x19, 0x09,
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x51, 0x41, 0x11, 0x01, 0x51, 0x41, 0x11, 0x01,
	0x00, 0x10, 0x40, 0x50, 0x00, 0x10, 0x40, 0x50, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x08, 0x18, 0x48, 0x58, 0x08, 0x18, 0x48, 0x58, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x10, 0x40, 0x50, 0x00, 0x10, 0x40, 0x50, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x08, 0x18, 0x48, 0x58, 0x08, 0x18, 0x48, 0x58, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x58, 0x48, 0x18, 0x08, 0x58, 0x48, 0x18, 0x08,
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x50, 0x40, 0x10, 0x00, 0x50, 0x40, 0x10, 0x00,
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x58, 0x48, 0x18, 0x08, 0x58, 0x48, 0x18, 0x08,
	0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x59, 0x50, 0x40, 0x10, 0x00, 0x50, 0x40, 0x10, 0x00,
	0x01, 0x11, 0x41, 0x51, 0x01, 0x11, 0x41, 0x51, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x09, 0x19, 0x49, 0x59, 0x09, 0x19, 0x49, 0x59, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x11, 0x41, 0x51, 0x01, 0x11, 0x41, 0x51, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x09, 0x19, 0x49, 0x59, 0x09, 0x19, 0x49, 0x59, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// MMC3208 (mapper 208, Street Fighter IV pirate): an MMC3 whose $4800-
// region register forces a 32 KiB PRG bank, with a protection LUT polled
// through $5000-$5FFF.
type MMC3208 struct {
	MMC3

	exRegs [6]byte
}

// NewMMC3208 wires the board.
func NewMMC3208(c *cartridge.Cartridge) *MMC3208 {
	m := &MMC3208{MMC3: *NewMMC3(c)}
	m.exRegs[5] = 3
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3208) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x4800 && addr < 0x5000:
		m.exRegs[5] = (v & 0x01) | (v>>3)&0x02
	case addr >= 0x5000 && addr <= 0x57FF:
		m.exRegs[4] = v
	case addr >= 0x5800 && addr < 0x6000:
		m.exRegs[addr&0x03] = v ^ mmc3_208ProtLut[m.exRegs[4]]
	case addr >= 0x6800 && addr < 0x7000:
		m.exRegs[5] = (v & 0x01) | (v>>3)&0x02
	default:
		m.MMC3.WritePRG(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3208) ReadPRG(addr uint16) byte {
	if addr >= 0x5800 && addr < 0x6000 {
		return m.exRegs[addr&0x03]
	}
	if addr >= 0x8000 {
		page := int(m.exRegs[5])<<2 | int(addr>>13&3)
		return window(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	return m.MMC3.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3208) Save(s *State) { m.MMC3.Save(s); copy(s.Regs[17:23], m.exRegs[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3208) Restore(s *State) { m.MMC3.Restore(s); copy(m.exRegs[:], s.Regs[17:23]) }

// MMC3198 (mapper 198, Waixing FDS conversions): an MMC3 whose four PRG
// slots are driven directly by four extra registers (loaded via $8001 for
// bank-select values >= 6), 4 KiB of work RAM mirrored across $5000-$7FFF,
// and a CHR-RAM mode when all CHR registers are zero. the reference emulator itself flags
// this board as most-likely-incorrect; this is a faithful port of it.
type MMC3198 struct {
	MMC3

	exRegs [4]byte
	wram   [0x1000]byte // 4 KiB, mirrored $5000-$7FFF
}

// NewMMC3198 wires the board.
func NewMMC3198(c *cartridge.Cartridge) *MMC3198 {
	m := &MMC3198{MMC3: *NewMMC3(c)}
	n := byte(len(m.prg) / 0x2000)
	m.exRegs = [4]byte{0, 1, n - 2, n - 1}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3198) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x8000 {
		m.wram[addr&0x0FFF] = v
		return
	}
	if addr == 0x8001 && m.bankSelect&0x07 >= 6 {
		mask := byte(0x3F)
		if v >= 0x40 {
			mask = 0x4F
		}
		m.exRegs[m.bankSelect&0x07-6] = v & mask
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3198) ReadPRG(addr uint16) byte {
	if addr >= 0x5000 && addr < 0x8000 {
		return m.wram[addr&0x0FFF]
	}
	if addr >= 0x8000 {
		slot := int((addr - 0x8000) >> 13)
		return window(m.prg, int(m.exRegs[slot]), 0x2000)[addr&0x1FFF]
	}
	return m.MMC3.ReadPRG(addr)
}

// chrRAMMode reports the all-registers-zero state that maps 8 KiB of CHR
// RAM straight through.
func (m *MMC3198) chrRAMMode() bool {
	for i := 0; i < 6; i++ {
		if m.banks[i] != 0 {
			return false
		}
	}
	return m.chr == nil
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3198) ReadCHR(addr uint16) byte {
	if m.chrRAMMode() {
		return m.chrRAM[addr&0x1FFF]
	}
	return m.MMC3.ReadCHR(addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3198) WriteCHR(addr uint16, v byte) {
	if m.chrRAMMode() {
		m.chrRAM[addr&0x1FFF] = v
		return
	}
	m.MMC3.WriteCHR(addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3198) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:21], m.exRegs[:])
	copy(s.PRGRAM[0x4000:0x5000], m.wram[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3198) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.exRegs[:], s.Regs[17:21])
	copy(m.wram[:], s.PRGRAM[0x4000:0x5000])
}

// Bmc8in1 (mapper 333): an MMC3 multicart with a register (written to the
// odd $x000 addresses) that adds a 128 KiB block to PRG/CHR, or forces a
// fixed 32 KiB PRG bank when its bit 4 is clear.
type Bmc8in1 struct {
	MMC3
	reg byte
}

// NewBmc8in1 wires the board.
func NewBmc8in1(c *cartridge.Cartridge) *Bmc8in1 {
	return &Bmc8in1{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc8in1) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 && addr&0x1000 != 0 {
		m.reg = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc8in1) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	if m.reg&0x10 == 0 {
		// Forced 32 KiB bank from the block.
		page := (int(m.reg&0x0F) << 2) | int(addr>>13&3)
		return window(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	page := (int(m.reg&0x0C) << 2) | (p & 0x0F)
	return window(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *Bmc8in1) chrPage(addr uint16) int {
	return (int(m.reg&0x0C) << 5) | (m.chrPage1K(addr) & 0x7F)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc8in1) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc8in1) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Bmc8in1) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *Bmc8in1) Restore(s *State) { m.MMC3.Restore(s); m.reg = s.Regs[17] }

// MMC3219 (mapper 219): an MMC3-shell board that ignores the normal MMC3
// banking entirely and drives four 8 KiB PRG windows plus eight 1 KiB CHR
// windows through its own $8000/$8001/$8002 state machine. It has no IRQ.
type MMC3219 struct {
	MMC3

	exReg  [3]byte
	prgReg [4]byte
	chrReg [8]byte
}

// NewMMC3219 wires the board.
func NewMMC3219(c *cartridge.Cartridge) *MMC3219 {
	m := &MMC3219{MMC3: *NewMMC3(c)}
	m.prgReg = [4]byte{0, 0, 0, 0xFF}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3219) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		m.MMC3.WritePRG(addr, v)
		return
	}
	if addr < 0xA000 {
		switch addr & 0xE003 {
		case 0x8000:
			m.exReg[0] = 0
			m.exReg[1] = v
		case 0x8002:
			m.exReg[0] = v
			m.exReg[1] = 0
		case 0x8001:
			if m.exReg[0] >= 0x23 && m.exReg[0] <= 0x26 {
				prgBank := (v&0x20)>>5 | (v&0x10)>>3 | (v&0x08)>>1 | (v&0x04)<<1
				m.prgReg[0x26-m.exReg[0]] = prgBank
			}
			switch m.exReg[1] {
			case 0x08, 0x0A, 0x0E, 0x12, 0x16, 0x1A, 0x1E:
				m.exReg[2] = v << 4
			case 0x09:
				m.chrReg[0] = m.exReg[2] | (v>>1)&0x0E
			case 0x0B:
				m.chrReg[1] = m.exReg[2] | (v >> 1) | 0x01
			case 0x0C, 0x0D:
				m.chrReg[2] = m.exReg[2] | (v>>1)&0x0E
			case 0x0F:
				m.chrReg[3] = m.exReg[2] | (v >> 1) | 0x01
			case 0x10, 0x11:
				m.chrReg[4] = m.exReg[2] | (v>>1)&0x0F
			case 0x14, 0x15:
				m.chrReg[5] = m.exReg[2] | (v>>1)&0x0F
			case 0x18, 0x19:
				m.chrReg[6] = m.exReg[2] | (v>>1)&0x0F
			case 0x1C, 0x1D:
				m.chrReg[7] = m.exReg[2] | (v>>1)&0x0F
			}
		}
		return
	}
	// $A000+ keeps the stock MMC3 mirroring/IRQ registers.
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3219) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	slot := int((addr - 0x8000) >> 13)
	return window(m.prg, int(m.prgReg[slot]), 0x2000)[addr&0x1FFF]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3219) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3219) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>10&7]), 0x400, addr, v)
}

// Scanline is a no-op: mapper 219 has no IRQ counter.
func (m *MMC3219) Scanline() {}

// IRQ reports whether the board is asserting the IRQ line.
func (m *MMC3219) IRQ() bool { return false }

// Save writes the board's mapper-specific state into s.
func (m *MMC3219) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:20], m.exReg[:])
	copy(s.Regs[20:24], m.prgReg[:])
	copy(s.Regs[24:32], m.chrReg[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3219) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.exReg[:], s.Regs[17:20])
	copy(m.prgReg[:], s.Regs[20:24])
	copy(m.chrReg[:], s.Regs[24:32])
}

// lut217 permutes the low 3 bits of a bank-select write in the alternate
// register-decode mode of mapper 217.
var lut217 = [8]byte{0, 6, 3, 7, 5, 2, 4, 1}

// MMC3217 (mapper 217): an MMC3 clone with a $5000-region register set.
// exReg1 supplies outer PRG/CHR bank bits and a 16/32-bank window mode;
// exReg2, when nonzero, swaps the register-decode order and descrambles
// the bank-select field; exReg0 bit 7 forces a fixed 32 KiB PRG bank.
type MMC3217 struct {
	MMC3

	exReg [4]byte
}

// NewMMC3217 wires the board.
func NewMMC3217(c *cartridge.Cartridge) *MMC3217 {
	m := &MMC3217{MMC3: *NewMMC3(c)}
	m.exReg = [4]byte{0, 0xFF, 0x03, 0}
	return m
}

// Reset returns the board to its power-on banking.
func (m *MMC3217) Reset(_ bool) {
	m.exReg = [4]byte{0, 0xFF, 0x03, 0}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3217) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		switch addr {
		case 0x5000:
			m.exReg[0] = v
		case 0x5001:
			m.exReg[1] = v
		case 0x5007:
			m.exReg[2] = v
		}
		return
	}
	switch addr & 0xE001 {
	case 0x8000:
		if m.exReg[2] != 0 {
			m.MMC3.WritePRG(0xC000, v)
		} else {
			m.MMC3.WritePRG(0x8000, v)
		}
	case 0x8001:
		if m.exReg[2] != 0 {
			v = v&0xC0 | lut217[v&0x07]
			m.exReg[3] = 1
			m.MMC3.WritePRG(0x8000, v)
		} else {
			m.MMC3.WritePRG(0x8001, v)
		}
	case 0xA000:
		if m.exReg[2] != 0 {
			if m.exReg[3] != 0 && (m.exReg[0]&0x80 == 0 || m.bankSelect&0x07 < 6) {
				m.exReg[3] = 0
				m.MMC3.WritePRG(0x8001, v)
			}
		} else {
			m.mirrorHorizontal = v&0x01 != 0
		}
	case 0xA001:
		if m.exReg[2] != 0 {
			m.mirrorHorizontal = v&0x01 != 0
		} else {
			m.MMC3.WritePRG(0xA001, v)
		}
	default:
		m.MMC3.WritePRG(addr, v)
	}
}

func (m *MMC3217) prgBankOuter(page int) int {
	if m.exReg[1]&0x08 != 0 {
		page &= 0x1F
	} else {
		page = page&0x0F | int(m.exReg[1]&0x10)
	}
	return int(m.exReg[1])<<5&0x60 | page
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3217) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	if m.exReg[0]&0x80 != 0 {
		// Forced 32 KiB bank.
		v := int(m.exReg[0]&0x0F) | int(m.exReg[1]<<4)&0x30
		page := m.prgBankOuter((v << 1) | int(addr>>13&1))
		return window(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	return window(m.prg, m.prgBankOuter(p), 0x2000)[addr&0x1FFF]
}

func (m *MMC3217) chrPageOuter(page int) int {
	if m.exReg[1]&0x08 == 0 {
		page = int(m.exReg[1]<<3)&0x80 | page&0x7F
	}
	return int(m.exReg[1])<<8&0x0300 | page
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3217) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrPageOuter(m.chrPage1K(addr)), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3217) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPageOuter(m.chrPage1K(addr)), 0x400, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3217) Save(s *State) { m.MMC3.Save(s); copy(s.Regs[17:21], m.exReg[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3217) Restore(s *State) { m.MMC3.Restore(s); copy(m.exReg[:], s.Regs[17:21]) }

// UNL-8237 (mapper 215): an MMC3 clone with a $5000-region register set
// that descrambles the $8000+ register addresses and bank-select fields
// through two per-config LUTs, and adds outer PRG/CHR banks with a forced
// window mode. Ported from the reference emulator.
var lutReg215 = [8][8]byte{
	{0, 1, 2, 3, 4, 5, 6, 7},
	{0, 2, 6, 1, 7, 3, 4, 5},
	{0, 5, 4, 1, 7, 2, 6, 3},
	{0, 6, 3, 7, 5, 2, 4, 1},
	{0, 2, 5, 3, 6, 1, 7, 4},
	{0, 1, 2, 3, 4, 5, 6, 7},
	{0, 1, 2, 3, 4, 5, 6, 7},
	{0, 1, 2, 3, 4, 5, 6, 7},
}
var lutAddr215 = [8][8]byte{
	{0, 1, 2, 3, 4, 5, 6, 7},
	{3, 2, 0, 4, 1, 5, 6, 7},
	{0, 1, 2, 3, 4, 5, 6, 7},
	{5, 0, 1, 2, 3, 7, 6, 4},
	{3, 1, 0, 5, 2, 4, 6, 7},
	{0, 1, 2, 3, 4, 5, 6, 7},
	{0, 1, 2, 3, 4, 5, 6, 7},
	{0, 1, 2, 3, 4, 5, 6, 7},
}

// MMC3215 (mapper 215) is an MMC3 clone with three extra registers
// selecting outer PRG/CHR banks and a fixed-bank mode.
type MMC3215 struct {
	MMC3
	exReg [3]byte
}

// NewMMC3215 wires the board.
func NewMMC3215(c *cartridge.Cartridge) *MMC3215 {
	m := &MMC3215{MMC3: *NewMMC3(c)}
	m.exReg = [3]byte{0, 3, 0}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3215) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x6000 {
		switch addr {
		case 0x5000:
			m.exReg[0] = v
		case 0x5001:
			m.exReg[1] = v
		case 0x5007:
			m.exReg[2] = v & 0x07
		}
		return
	}
	if addr < 0x8000 {
		m.MMC3.WritePRG(addr, v)
		return
	}
	lutValue := lutAddr215[m.exReg[2]][(addr>>12)&0x06|(addr&0x01)]
	na := uint16(lutValue&0x01) | uint16(lutValue&0x06)<<12 | 0x8000
	if lutValue == 0 {
		v = v&0xC0 | lutReg215[m.exReg[2]][v&0x07]
	}
	m.MMC3.WritePRG(na, v)
}

func (m *MMC3215) chrPage(addr uint16) int {
	page := m.chrPage1K(addr)
	if m.exReg[0]&0x40 != 0 {
		return int(m.exReg[1]&0x0C)<<6 | page&0x7F | int(m.exReg[1]&0x20)<<2
	}
	return int(m.exReg[1]&0x0C)<<6 | page
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3215) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3215) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

func (m *MMC3215) prgSlot(slot int) int {
	var sbank, bank, mask int
	if m.exReg[0]&0x40 != 0 {
		mask = 0x0F
		sbank = int(m.exReg[1] & 0x10)
		if m.exReg[0]&0x80 != 0 {
			bank = int(m.exReg[1]&0x03)<<4 | int(m.exReg[0]&0x07) | sbank>>1
		}
	} else {
		mask = 0x1F
		if m.exReg[0]&0x80 != 0 {
			bank = int(m.exReg[1]&0x03)<<4 | int(m.exReg[0]&0x0F)
		}
	}
	if m.exReg[0]&0x80 != 0 {
		bank <<= 1
		if m.exReg[0]&0x20 != 0 { // 32 KiB
			return bank + slot
		}
		// 16 KiB mirrored into both halves.
		return bank + (slot & 1)
	}
	p := m.prgBankRaw(slot)
	return int(m.exReg[1]&0x03)<<5 | (p & mask) | sbank
}

// prgBankRaw resolves the stock MMC3 8 KiB bank for a CPU slot 0-3.
func (m *MMC3215) prgBankRaw(slot int) int {
	addr := uint16(0x8000 + slot*0x2000)
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	return p
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3215) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return window(m.prg, m.prgSlot(int((addr-0x8000)>>13)), 0x2000)[addr&0x1FFF]
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3215) Save(s *State) { m.MMC3.Save(s); copy(s.Regs[17:20], m.exReg[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3215) Restore(s *State) { m.MMC3.Restore(s); copy(m.exReg[:], s.Regs[17:20]) }

// MMC3126 (mapper 126): an MMC3 multicart whose $6000-region registers
// add outer PRG/CHR banks and a forced 16/32 KiB PRG window mode.
type MMC3126 struct {
	MMC3
	exReg [4]byte
}

// NewMMC3126 wires the board.
func NewMMC3126(c *cartridge.Cartridge) *MMC3126 {
	return &MMC3126{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3126) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		a := addr & 0x03
		if a == 0x01 || a == 0x02 || ((a == 0x00 || a == 0x03) && m.exReg[3]&0x80 == 0) {
			m.exReg[a] = v
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

func (m *MMC3126) chrOuter() int {
	reg := int(m.exReg[0])
	return (^reg & 0x0080 & int(m.exReg[2])) |
		(reg << 4 & 0x0080 & reg) |
		(reg << 3 & 0x0100) |
		(reg << 5 & 0x0200)
}

func (m *MMC3126) prgPage(slot int) int {
	reg := int(m.exReg[0])
	p := m.prgBankRaw(slot)
	p &= (^reg >> 2 & 0x10) | 0x0F
	p |= (reg & (0x06 | (reg&0x40)>>6)) << 4
	p |= (reg & 0x10) << 3

	if m.exReg[3]&0x03 == 0 {
		return p
	}
	// Fixed-window mode: base from the swap slot.
	base := p
	if m.exReg[3]&0x03 == 0x03 {
		return base + slot // 32 KiB
	}
	// 16 KiB mirrored.
	return base + (slot & 1)
}

func (m *MMC3126) prgBankRaw(slot int) int {
	addr := uint16(0x8000 + slot*0x2000)
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	return p
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3126) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return window(m.prg, m.prgPage(int((addr-0x8000)>>13)), 0x2000)[addr&0x1FFF]
}

func (m *MMC3126) chrPage(addr uint16) int {
	if m.exReg[3]&0x10 != 0 {
		// 8 KiB CHR mode.
		page := m.chrOuter() | int(m.exReg[2]&0x0F)<<3
		return page + int(addr>>10&7)
	}
	return m.chrOuter() | (m.chrPage1K(addr) & (int(m.exReg[0]&0x80) - 1))
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3126) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3126) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *MMC3126) Save(s *State) { m.MMC3.Save(s); copy(s.Regs[17:21], m.exReg[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3126) Restore(s *State) { m.MMC3.Restore(s); copy(m.exReg[:], s.Regs[17:21]) }

// BmcHpxx (mapper 260): an MMC3 multicart with a $5000-region register set
// (lockable) that adds outer PRG/CHR banks with several window modes, and
// an alternate mode where $8000+ writes feed a CHR outer register instead
// of the MMC3 register file.
type BmcHpxx struct {
	MMC3
	exReg  [5]byte
	locked bool
}

// NewBmcHpxx wires the board.
func NewBmcHpxx(c *cartridge.Cartridge) *BmcHpxx {
	return &BmcHpxx{MMC3: *NewMMC3(c)}
}

// Reset returns the board to its power-on banking.
func (m *BmcHpxx) Reset(_ bool) {
	m.exReg = [5]byte{}
	m.locked = false
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *BmcHpxx) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x6000 {
		if !m.locked {
			m.exReg[addr&0x03] = v
			m.locked = v&0x80 != 0
		}
		return
	}
	if addr >= 0x8000 && m.exReg[0]&0x04 != 0 {
		m.exReg[4] = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

func (m *BmcHpxx) chrPage(addr uint16) int {
	p := m.chrPage1K(addr)
	if m.exReg[0]&0x04 != 0 {
		switch m.exReg[0] & 0x03 {
		case 0, 1:
			return int(m.exReg[2]&0x3F)<<3 + int(addr>>10&7)
		case 2:
			return (int(m.exReg[2]&0x3E)|int(m.exReg[4]&0x01))<<3 + int(addr>>10&7)
		default:
			return (int(m.exReg[2]&0x3C)|int(m.exReg[4]&0x03))<<3 + int(addr>>10&7)
		}
	}
	var base, mask int
	if m.exReg[0]&0x01 != 0 {
		base, mask = int(m.exReg[2]&0x30), 0x7F
	} else {
		base, mask = int(m.exReg[2]&0x20), 0xFF
	}
	return (p & mask) | base<<3
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *BmcHpxx) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *BmcHpxx) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

func (m *BmcHpxx) prgSlot(slot int) int {
	if m.exReg[0]&0x04 != 0 {
		if m.exReg[0]&0x0F == 0x04 {
			return int(m.exReg[1]&0x1F)<<1 + (slot & 1)
		}
		return int(m.exReg[1]&0x1E)<<1 + slot
	}
	var base, mask int
	if m.exReg[0]&0x02 != 0 {
		base, mask = int(m.exReg[1]&0x18), 0x0F
	} else {
		base, mask = int(m.exReg[1]&0x10), 0x1F
	}
	p := m.prgBankRawHpxx(slot)
	return (p & mask) | base<<1
}

func (m *BmcHpxx) prgBankRawHpxx(slot int) int {
	addr := uint16(0x8000 + slot*0x2000)
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	return p
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *BmcHpxx) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return window(m.prg, m.prgSlot(int((addr-0x8000)>>13)), 0x2000)[addr&0x1FFF]
}

// Mirroring reports the board's current nametable mirroring.
func (m *BmcHpxx) Mirroring() cartridge.Mirroring {
	if m.exReg[0]&0x04 != 0 {
		if m.exReg[4]&0x04 != 0 {
			return cartridge.Vertical
		}
		return cartridge.Horizontal
	}
	return m.MMC3.Mirroring()
}

// Save writes the board's mapper-specific state into s.
func (m *BmcHpxx) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:22], m.exReg[:])
	s.Regs[22] = boolByte(m.locked)
}

// Restore loads the board's mapper-specific state from s.
func (m *BmcHpxx) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.exReg[:], s.Regs[17:22])
	m.locked = s.Regs[22] != 0
}

// MMC3121 (mapper 121, K-H1002/pirate MMC3): adds a $5000-region exreg
// state machine (UpdateExRegs) that can force fixed PRG windows, plus a
// CHR outer bank. The $8001 bank-select data is descrambled and, for the
// "Super 3-in-1" hack, the CHR outer bit shifts. Ported from the reference emulator.
type MMC3121 struct {
	MMC3

	exReg [8]byte
}

// NewMMC3121 wires the board.
func NewMMC3121(c *cartridge.Cartridge) *MMC3121 {
	m := &MMC3121{MMC3: *NewMMC3(c)}
	m.exReg[3] = 0x80
	return m
}

func (m *MMC3121) updateExRegs() {
	switch m.exReg[5] & 0x3F {
	case 0x20:
		m.exReg[7] = 1
		m.exReg[0] = m.exReg[6]
	case 0x29:
		m.exReg[7] = 1
		m.exReg[0] = m.exReg[6]
	case 0x26:
		m.exReg[7] = 0
		m.exReg[0] = m.exReg[6]
	case 0x2B:
		m.exReg[7] = 1
		m.exReg[0] = m.exReg[6]
	case 0x2C:
		m.exReg[7] = 1
		if m.exReg[6] != 0 {
			m.exReg[0] = m.exReg[6]
		}
	case 0x3C, 0x3F:
		m.exReg[7] = 1
		m.exReg[0] = m.exReg[6]
	case 0x28:
		m.exReg[7] = 0
		m.exReg[1] = m.exReg[6]
	case 0x2A:
		m.exReg[7] = 0
		m.exReg[2] = m.exReg[6]
	case 0x2F:
		// keep
	default:
		m.exReg[5] = 0
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3121) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x5000 && addr < 0x6000:
		// Protection LUT is read-side; the write only latches on $5180.
		if addr&0x5180 == 0x5180 {
			m.exReg[3] = v
		}
	case addr < 0x8000:
		m.MMC3.WritePRG(addr, v)
	case addr < 0xA000:
		switch {
		case addr&0x03 == 0x03:
			m.exReg[5] = v
			m.updateExRegs()
			m.MMC3.WritePRG(0x8000, v)
		case addr&0x01 != 0:
			m.exReg[6] = (v&0x01)<<5 | (v&0x02)<<3 | (v&0x04)<<1 | (v&0x08)>>1 | (v&0x10)>>3 | (v&0x20)>>5
			if m.exReg[7] == 0 {
				m.updateExRegs()
			}
			m.MMC3.WritePRG(0x8001, v)
		default:
			m.MMC3.WritePRG(0x8000, v)
		}
	default:
		m.MMC3.WritePRG(addr, v)
	}
}

// prgSlot121 implements the reference's per-slot PRG select.
func (m *MMC3121) prgSlot(slot int) int {
	or := int(m.exReg[3]&0x80) >> 2
	if m.exReg[5]&0x3F != 0 {
		switch slot {
		case 1:
			return int(m.exReg[2]) | or
		case 2:
			return int(m.exReg[1]) | or
		case 3:
			return int(m.exReg[0]) | or
		}
	}
	p := m.prgBankRaw121(slot)
	return (p & 0x1F) | or
}

func (m *MMC3121) prgBankRaw121(slot int) int {
	addr := uint16(0x8000 + slot*0x2000)
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	return p
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3121) ReadPRG(addr uint16) byte {
	// $5000-$5FFF returns a small protection table.
	if addr >= 0x5000 && addr < 0x6000 {
		lut := [4]byte{0x83, 0x83, 0x42, 0x00}
		return lut[m.exReg[3]&0x03]
	}
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return window(m.prg, m.prgSlot(int((addr-0x8000)>>13)), 0x2000)[addr&0x1FFF]
}

func (m *MMC3121) chrPage(addr uint16) int {
	p := m.chrPage1K(addr)
	if m.chr != nil && len(m.prg) == len(m.chr) {
		// Super 3-in-1 hack: PRG and CHR the same size.
		return p | int(m.exReg[3]&0x80)<<1
	}
	sel := addr >> 12 & 1
	if m.bankSelect&0x80 != 0 {
		sel ^= 1
	}
	if sel != 0 {
		p |= 0x100
	}
	return p
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3121) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3121) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *MMC3121) Save(s *State) { m.MMC3.Save(s); copy(s.Regs[17:25], m.exReg[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3121) Restore(s *State) { m.MMC3.Restore(s); copy(m.exReg[:], s.Regs[17:25]) }

// MMC314 (mapper 14, "Gouder SL-1632"): a dual-mode board. A $A131 mode
// register selects between a stock-MMC3 path (with three CHR outer-bank
// groups) and a VRC-like path with its own PRG/CHR registers and
// mirroring. Ported from the reference emulator.
type MMC314 struct {
	MMC3

	vrcChr  [8]byte
	vrcPrg  [2]byte
	vrcMirr byte
	mode    byte
}

// NewMMC314 wires the board.
func NewMMC314(c *cartridge.Cartridge) *MMC314 {
	return &MMC314{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC314) WritePRG(addr uint16, v byte) {
	if addr == 0xA131 {
		m.mode = v
	}
	if m.mode&0x02 != 0 {
		m.MMC3.WritePRG(addr, v)
		return
	}
	// VRC mode.
	if addr >= 0xB000 && addr <= 0xEFFF {
		reg := ((((addr >> 12) & 0x07) - 3) << 1) + ((addr >> 1) & 0x01)
		if addr&0x01 == 0 {
			m.vrcChr[reg] = m.vrcChr[reg]&0xF0 | v&0x0F
		} else {
			m.vrcChr[reg] = m.vrcChr[reg]&0x0F | (v&0x0F)<<4
		}
		return
	}
	switch addr & 0xF003 {
	case 0x8000:
		m.vrcPrg[0] = v
	case 0x9000:
		m.vrcMirr = v
	case 0xA000:
		m.vrcPrg[1] = v
	}
}

// chrOuter14 returns the three CHR outer-bank bits for MMC3 mode.
func (m *MMC314) chrOuter(win int) int {
	switch {
	case win < 2:
		if m.mode&0x08 != 0 {
			return 0x100
		}
	case win < 4:
		if m.mode&0x20 != 0 {
			return 0x100
		}
	default:
		if m.mode&0x80 != 0 {
			return 0x100
		}
	}
	return 0
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC314) ReadCHR(addr uint16) byte {
	if m.mode&0x02 == 0 {
		// VRC mode: eight 1 KiB windows.
		return m.chrRead(int(m.vrcChr[addr>>10&7]), 0x400, addr)
	}
	// MMC3 mode with the three-group CHR outer bank.
	a := addr
	if m.bankSelect&0x80 != 0 {
		a ^= 0x1000
	}
	win := int(a >> 10 & 7)
	page := m.chrPage1K(addr) | m.chrOuter(win)
	return m.chrRead(page, 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC314) WriteCHR(addr uint16, v byte) {
	if m.mode&0x02 == 0 {
		m.chrWrite(int(m.vrcChr[addr>>10&7]), 0x400, addr, v)
		return
	}
	a := addr
	if m.bankSelect&0x80 != 0 {
		a ^= 0x1000
	}
	win := int(a >> 10 & 7)
	page := m.chrPage1K(addr) | m.chrOuter(win)
	m.chrWrite(page, 0x400, addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC314) ReadPRG(addr uint16) byte {
	if m.mode&0x02 != 0 {
		return m.MMC3.ReadPRG(addr)
	}
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	switch addr >> 13 & 3 {
	case 0:
		return window(m.prg, int(m.vrcPrg[0]), 0x2000)[addr&0x1FFF]
	case 1:
		return window(m.prg, int(m.vrcPrg[1]), 0x2000)[addr&0x1FFF]
	case 2:
		return window(m.prg, -2, 0x2000)[addr&0x1FFF]
	default:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	}
}

// Mirroring reports the board's current nametable mirroring.
func (m *MMC314) Mirroring() cartridge.Mirroring {
	if m.mode&0x02 == 0 {
		return hvMirror(m.vrcMirr&0x01 == 0)
	}
	return m.MMC3.Mirroring()
}

// Save writes the board's mapper-specific state into s.
func (m *MMC314) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:25], m.vrcChr[:])
	s.Regs[25] = m.vrcPrg[0]
	s.Regs[26] = m.vrcPrg[1]
	s.Regs[27] = m.vrcMirr
	s.Regs[28] = m.mode
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC314) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.vrcChr[:], s.Regs[17:25])
	m.vrcPrg[0] = s.Regs[25]
	m.vrcPrg[1] = s.Regs[26]
	m.vrcMirr = s.Regs[27]
	m.mode = s.Regs[28]
}

// Mapper116 (mapper 116, "SOMARI-P" / Huang-1/2): a triple-personality
// board. A $4100 mode register selects between an emulated VRC2, MMC3, or
// MMC1, each with its own register file; a global CHR outer bank (mode bit
// 2) sits on top. The MMC3 personality's A12 IRQ clocks off Scanline().
// Ported from the reference emulator.
type Mapper116 struct {
	base

	mode byte

	vrc2Chr  [8]byte
	vrc2Prg  [2]byte
	vrc2Mirr byte

	mmc3Regs [10]byte
	mmc3Ctrl byte
	mmc3Mirr byte

	mmc1Regs   [4]byte
	mmc1Buffer byte
	mmc1Shift  byte

	irqCounter byte
	irqReload  byte
	irqReloadF bool
	irqEnabled bool
	irqLine    bool
}

// NewMapper116 wires the board.
func NewMapper116(c *cartridge.Cartridge) *Mapper116 {
	m := &Mapper116{base: makeBase(c)}
	m.vrc2Chr = [8]byte{0xFF, 0xFF, 0xFF, 0xFF, 4, 5, 6, 7}
	m.vrc2Prg = [2]byte{0, 1}
	m.mmc3Regs = [10]byte{0, 2, 4, 5, 6, 7, 0xFC, 0xFD, 0xFE, 0xFF}
	m.mmc1Regs[0] = 0x0C
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper116) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr&0x4100 == 0x4100 {
			m.mode = v
			if addr&0x01 != 0 {
				m.mmc1Regs[0] = 0x0C
				m.mmc1Regs[3] = 0
				m.mmc1Buffer = 0
				m.mmc1Shift = 0
			}
		}
		return
	}
	switch m.mode & 0x03 {
	case 0:
		m.writeVrc2(addr, v)
	case 1:
		m.writeMmc3(addr, v)
	default:
		m.writeMmc1(addr, v)
	}
}

func (m *Mapper116) writeVrc2(addr uint16, v byte) {
	if addr >= 0xB000 && addr <= 0xE003 {
		reg := ((((addr & 0x02) | (addr >> 10)) >> 1) + 2) & 0x07
		shift := (addr & 1) << 2
		m.vrc2Chr[reg] = m.vrc2Chr[reg]&byte(0xF0>>shift) | (v&0x0F)<<shift
		return
	}
	switch addr & 0xF000 {
	case 0x8000:
		m.vrc2Prg[0] = v
	case 0xA000:
		m.vrc2Prg[1] = v
	case 0x9000:
		m.vrc2Mirr = v
	}
}

func (m *Mapper116) writeMmc3(addr uint16, v byte) {
	switch addr & 0xE001 {
	case 0x8000:
		m.mmc3Ctrl = v
	case 0x8001:
		m.mmc3Regs[m.mmc3Ctrl&0x07] = v
	case 0xA000:
		m.mmc3Mirr = v
	case 0xC000:
		m.irqReload = v
	case 0xC001:
		m.irqReloadF = true
	case 0xE000:
		m.irqEnabled = false
		m.irqLine = false
	case 0xE001:
		m.irqEnabled = true
	}
}

func (m *Mapper116) writeMmc1(addr uint16, v byte) {
	if v&0x80 != 0 {
		m.mmc1Regs[0] |= 0x0C
		m.mmc1Buffer, m.mmc1Shift = 0, 0
		return
	}
	reg := (addr >> 13) - 4
	m.mmc1Buffer |= (v & 0x01) << m.mmc1Shift
	m.mmc1Shift++
	if m.mmc1Shift == 5 {
		m.mmc1Regs[reg&3] = m.mmc1Buffer
		m.mmc1Buffer, m.mmc1Shift = 0, 0
	}
}

// prgSlot returns the 8 KiB PRG bank for a CPU slot 0-3.
func (m *Mapper116) prgSlot(slot int) int {
	switch m.mode & 0x03 {
	case 0:
		switch slot {
		case 0:
			return int(m.vrc2Prg[0])
		case 1:
			return int(m.vrc2Prg[1])
		case 2:
			return -2
		default:
			return -1
		}
	case 1:
		prgMode := int(m.mmc3Ctrl>>5) & 0x02
		switch slot {
		case 0:
			return int(m.mmc3Regs[6+prgMode])
		case 1:
			return int(m.mmc3Regs[7])
		case 2:
			return int(m.mmc3Regs[6+(prgMode^0x02)])
		default:
			return int(m.mmc3Regs[9])
		}
	default: // MMC1
		bank := int(m.mmc1Regs[3] & 0x0F)
		if m.mmc1Regs[0]&0x08 != 0 {
			if m.mmc1Regs[0]&0x04 != 0 {
				if slot < 2 {
					return (bank << 1) + slot
				}
				return (0x0F << 1) + (slot - 2)
			}
			if slot < 2 {
				return slot
			}
			return (bank << 1) + (slot - 2)
		}
		return ((bank & 0xFE) << 1) + slot
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper116) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prgSlot(int((addr-0x8000)>>13)), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// chrSlot returns the 1 KiB CHR bank for a PPU window 0-7.
func (m *Mapper116) chrSlot(win int) int {
	outer := int(m.mode&0x04) << 6
	switch m.mode & 0x03 {
	case 0:
		return outer | int(m.vrc2Chr[win])
	case 1:
		swap := 0
		if m.mmc3Ctrl&0x80 != 0 {
			swap = 4
		}
		w := win ^ swap
		switch w {
		case 0:
			return outer | int(m.mmc3Regs[0]&0xFE)
		case 1:
			return outer | int(m.mmc3Regs[0]|1)
		case 2:
			return outer | int(m.mmc3Regs[1]&0xFE)
		case 3:
			return outer | int(m.mmc3Regs[1]|1)
		default:
			return outer | int(m.mmc3Regs[2+(w-4)])
		}
	default: // MMC1
		if m.mmc1Regs[0]&0x10 != 0 {
			if win < 4 {
				return int(m.mmc1Regs[1])<<2 + win
			}
			return int(m.mmc1Regs[2])<<2 + (win - 4)
		}
		return (int(m.mmc1Regs[1]&0xFE) << 2) + win
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper116) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrSlot(int(addr>>10&7)), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper116) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrSlot(int(addr>>10&7)), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper116) Mirroring() cartridge.Mirroring {
	switch m.mode & 0x03 {
	case 0:
		return hvMirror(m.vrc2Mirr&0x01 == 0)
	case 1:
		return hvMirror(m.mmc3Mirr&0x01 == 0)
	default:
		switch m.mmc1Regs[0] & 0x03 {
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
}

// Scanline clocks the MMC3-personality A12 IRQ (only active in mode 1).
func (m *Mapper116) Scanline() {
	if m.mode&0x03 != 1 {
		return
	}
	if m.irqCounter == 0 || m.irqReloadF {
		m.irqCounter = m.irqReload
		m.irqReloadF = false
	} else {
		m.irqCounter--
	}
	if m.irqCounter == 0 && m.irqEnabled {
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper116) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper116) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.mode
	copy(s.Regs[1:9], m.vrc2Chr[:])
	s.Regs[9] = m.vrc2Prg[0]
	s.Regs[10] = m.vrc2Prg[1]
	s.Regs[11] = m.vrc2Mirr
	copy(s.Regs[12:22], m.mmc3Regs[:])
	s.Regs[22] = m.mmc3Ctrl
	s.Regs[23] = m.mmc3Mirr
	copy(s.Regs[24:28], m.mmc1Regs[:])
	s.Regs[28] = m.mmc1Buffer
	s.Regs[29] = m.mmc1Shift
	s.Regs[30] = m.irqCounter
	s.Regs[31] = m.irqReload
	s.Regs[32] = boolByte(m.irqReloadF)
	s.Regs[33] = boolByte(m.irqEnabled)
	s.Regs[34] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper116) Restore(s *State) {
	m.restoreRAM(s)
	m.mode = s.Regs[0]
	copy(m.vrc2Chr[:], s.Regs[1:9])
	m.vrc2Prg[0] = s.Regs[9]
	m.vrc2Prg[1] = s.Regs[10]
	m.vrc2Mirr = s.Regs[11]
	copy(m.mmc3Regs[:], s.Regs[12:22])
	m.mmc3Ctrl = s.Regs[22]
	m.mmc3Mirr = s.Regs[23]
	copy(m.mmc1Regs[:], s.Regs[24:28])
	m.mmc1Buffer = s.Regs[28]
	m.mmc1Shift = s.Regs[29]
	m.irqCounter = s.Regs[30]
	m.irqReload = s.Regs[31]
	m.irqReloadF = s.Regs[32] != 0
	m.irqEnabled = s.Regs[33] != 0
	m.irqLine = s.Regs[34] != 0
}
