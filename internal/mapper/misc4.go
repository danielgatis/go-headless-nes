package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// More vendor/multicart boards ported from the reference emulator.

// GoldenFive (mapper 104): an outer/inner PRG register pair driving a
// 16 KiB window; the fixed high window follows the outer bank.
type GoldenFive struct {
	base
	prgReg byte
	prg1   int
}

// NewGoldenFive constructs the corresponding board.
func NewGoldenFive(c *cartridge.Cartridge) *GoldenFive {
	return &GoldenFive{base: makeBase(c), prg1: 0x0F}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *GoldenFive) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0xC000:
		m.prgReg = m.prgReg&0xF0 | v&0x0F
	case addr >= 0x8000 && addr <= 0x9FFF:
		if v&0x08 != 0 {
			m.prgReg = m.prgReg&0x0F | (v<<4)&0x70
			m.prg1 = int(v<<4)&0x70 | 0x0F
		}
	case addr >= 0x6000 && addr < 0x8000:
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *GoldenFive) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgReg), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *GoldenFive) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *GoldenFive) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *GoldenFive) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg
	s.Regs[1] = byte(m.prg1)
}

// Restore loads the board's mapper-specific state from s.
func (m *GoldenFive) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg = s.Regs[0]
	m.prg1 = int(s.Regs[1])
}

// Lh32 (mapper 125): a $6000 register banks a PRG-ROM window there; the
// rest of the map is fixed (last two banks + a WRAM window).
type Lh32 struct {
	base
	prgReg byte
}

// NewLh32 constructs the corresponding board.
func NewLh32(c *cartridge.Cartridge) *Lh32 {
	return &Lh32{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Lh32) WritePRG(addr uint16, v byte) {
	if addr == 0x6000 {
		m.prgReg = v
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Lh32) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.readPRGRAM(addr) // WRAM window at $C000-$DFFF
	case addr >= 0xA000:
		return window(m.prg, -3, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, -4, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return window(m.prg, int(m.prgReg), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// WriteCHR handles a write into the CHR address space.
func (m *Lh32) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Lh32) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// Save writes the board's mapper-specific state into s.
func (m *Lh32) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.prgReg }

// Restore loads the board's mapper-specific state from s.
func (m *Lh32) Restore(s *State) { m.restoreRAM(s); m.prgReg = s.Regs[0] }

// DreamTech01 (mapper 521): a $5020 register selects a 16 KiB PRG bank at
// $8000; $C000 is fixed to bank 8.
type DreamTech01 struct {
	base
	prg0 byte
}

// NewDreamTech01 constructs the corresponding board.
func NewDreamTech01(c *cartridge.Cartridge) *DreamTech01 {
	return &DreamTech01{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *DreamTech01) WritePRG(addr uint16, v byte) {
	if addr == 0x5020 {
		m.prg0 = v & 0x07
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *DreamTech01) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, 8, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prg0), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *DreamTech01) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *DreamTech01) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *DreamTech01) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.prg0 }

// Restore loads the board's mapper-specific state from s.
func (m *DreamTech01) Restore(s *State) { m.restoreRAM(s); m.prg0 = s.Regs[0] }

// Super40in1Ws (mapper 332): a $6000-region register pair (odd = CHR, even
// = PRG + a lock bit) drives a 16/32 KiB PRG window.
type Super40in1Ws struct {
	base
	prg0    int
	prg1    int
	chrBank int
	lock    bool
	mirror  cartridge.Mirroring
}

// NewSuper40in1Ws constructs the corresponding board.
func NewSuper40in1Ws(c *cartridge.Cartridge) *Super40in1Ws {
	return &Super40in1Ws{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Super40in1Ws) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x7000 {
		if m.lock {
			return
		}
		if addr&0x01 != 0 {
			m.chrBank = int(v)
		} else {
			m.lock = v&0x20 == 0x20
			m.prg0 = int(v) &^ int(^v>>3&0x01)
			m.prg1 = int(v) | int(^v>>3&0x01)
			m.mirror = hvMirror(v&0x10 == 0)
		}
		return
	}
	if addr >= 0x7000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Super40in1Ws) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	case addr >= 0x7000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Super40in1Ws) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Super40in1Ws) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Super40in1Ws) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Super40in1Ws) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.prg1)
	s.Regs[2] = byte(m.chrBank)
	s.Regs[3] = boolByte(m.lock)
	s.Regs[4] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Super40in1Ws) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.prg1 = int(s.Regs[1])
	m.chrBank = int(s.Regs[2])
	m.lock = s.Regs[3] != 0
	m.mirror = cartridge.Mirroring(s.Regs[4])
}

// Bmc830118C (mapper 348): an MMC3 clone with a $6800-region outer
// register that adds high PRG/CHR bits.
type Bmc830118C struct {
	MMC3
	reg byte
}

// NewBmc830118C constructs the corresponding board.
func NewBmc830118C(c *cartridge.Cartridge) *Bmc830118C {
	return &Bmc830118C{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc830118C) WritePRG(addr uint16, v byte) {
	if addr >= 0x6800 && addr <= 0x68FF {
		m.reg = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc830118C) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	p := m.prgBank(addr)
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	page := int(m.reg&0x0C)<<2 | p&0x0F
	if m.reg&0x0C == 0x0C {
		// Slots 2/3 map a fixed high bank in this mode.
		if addr >= 0xC000 {
			page = 0x32 | p&0x0F
		}
	}
	return window(m.prg, page, 0x2000)[addr&0x1FFF]
}

func (m *Bmc830118C) chrPage(addr uint16) int {
	return int(m.reg&0x0C)<<5 | m.chrPage1K(addr)&0x7F
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc830118C) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc830118C) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Bmc830118C) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *Bmc830118C) Restore(s *State) { m.MMC3.Restore(s); m.reg = s.Regs[17] }

// SealieComputing (mapper 29): a value write sets a 16 KiB PRG bank
// ($8000, last fixed) and a CHR-RAM bank; 8 KiB WRAM at $6000.
type SealieComputing struct {
	base
	prg0 int
	chr  int
}

// NewSealieComputing constructs the corresponding board.
func NewSealieComputing(c *cartridge.Cartridge) *SealieComputing {
	return &SealieComputing{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *SealieComputing) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.chr = int(v & 0x03)
		m.prg0 = int(v>>2) & 0x07
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *SealieComputing) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *SealieComputing) ReadCHR(addr uint16) byte { return m.chrRead(m.chr, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *SealieComputing) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chr, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *SealieComputing) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.chr)
}

// Restore loads the board's mapper-specific state from s.
func (m *SealieComputing) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.chr = int(s.Regs[1])
}

// Bmc8157 (mapper 301): a purely address-latched multicart with inner/
// outer PRG bits and 16/32 KiB modes. The DIP-gated open-bus mode is not
// modelled (DIP 0).
type Bmc8157 struct {
	base
	prg0   int
	prg1   int
	mirror cartridge.Mirroring
}

// NewBmc8157 constructs the corresponding board.
func NewBmc8157(c *cartridge.Cartridge) *Bmc8157 {
	m := &Bmc8157{base: makeBase(c), mirror: c.Mirroring}
	m.decode(0x8000)
	return m
}

func (m *Bmc8157) decode(addr uint16) {
	innerPrg0 := int(addr>>2) & 0x07
	innerPrg1 := int(addr>>7)&0x01 | int(addr>>8)&0x02
	outer128 := int(addr>>5) & 0x03
	outer512 := int(addr>>8) & 0x01
	var baseBank int
	switch innerPrg1 {
	case 0:
		baseBank = 0
	case 1:
		baseBank = innerPrg0
	default:
		baseBank = 7
	}
	m.prg0 = outer512<<6 | outer128<<3 | innerPrg0
	m.prg1 = outer512<<6 | outer128<<3 | baseBank
	m.mirror = hvMirror(addr&0x02 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc8157) WritePRG(addr uint16, _ byte) {
	if addr >= 0x8000 {
		m.decode(addr)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc8157) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc8157) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc8157) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc8157) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc8157) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.prg1)
	s.Regs[2] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc8157) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.prg1 = int(s.Regs[1])
	m.mirror = cartridge.Mirroring(s.Regs[2])
}

// Bmc64in1NoRepeat (mapper 314): a $5000-region 4-register board driving a
// 16/32 KiB PRG window and CHR bank with mirroring.
type Bmc64in1NoRepeat struct {
	base
	regs   [4]byte
	prg0   int
	prg1   int
	chr    int
	mirror cartridge.Mirroring
}

// NewBmc64in1NoRepeat constructs the corresponding board.
func NewBmc64in1NoRepeat(c *cartridge.Cartridge) *Bmc64in1NoRepeat {
	m := &Bmc64in1NoRepeat{base: makeBase(c)}
	m.regs = [4]byte{0x80, 0x43, 0, 0}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc64in1NoRepeat) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x5000 && addr <= 0x5003 {
			m.regs[addr&0x03] = v
			m.update()
		}
		return
	}
	m.regs[3] = v
	m.update()
}

func (m *Bmc64in1NoRepeat) update() {
	if m.regs[0]&0x80 != 0 {
		if m.regs[1]&0x80 != 0 {
			b := int(m.regs[1]&0x1F) << 1
			m.prg0, m.prg1 = b, b+1
		} else {
			bank := int(m.regs[1]&0x1F)<<1 | int(m.regs[1]>>6)&0x01
			m.prg0, m.prg1 = bank, bank
		}
	} else {
		m.prg1 = int(m.regs[1]&0x1F)<<1 | int(m.regs[1]>>6)&0x01
	}
	m.mirror = hvMirror(m.regs[0]&0x20 == 0)
	m.chr = int(m.regs[2])<<2 | int(m.regs[0]>>1)&0x03
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc64in1NoRepeat) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc64in1NoRepeat) ReadCHR(addr uint16) byte { return m.chrRead(m.chr, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc64in1NoRepeat) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chr, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc64in1NoRepeat) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc64in1NoRepeat) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.regs[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc64in1NoRepeat) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.regs[:], s.Regs[0:4])
	m.update()
}

// FaridUnrom (mapper 324): a UNROM-like board with an outer bank latched
// once per soft-reset window; a lock bit freezes it. CHR is 8 KiB RAM.
type FaridUnrom struct {
	base
	reg byte
}

// NewFaridUnrom constructs the corresponding board.
func NewFaridUnrom(c *cartridge.Cartridge) *FaridUnrom {
	return &FaridUnrom{base: makeBase(c)}
}

// Reset returns the board to its power-on banking.
func (m *FaridUnrom) Reset(soft bool) {
	if soft {
		m.reg &= 0x87
	} else {
		m.reg = 0
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *FaridUnrom) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	// Bus conflict: the ROM byte ANDs the written value.
	v &= window(m.prg, m.prg0(), 0x4000)[addr&0x3FFF]
	locked := m.reg&0x08 != 0
	if !locked && m.reg&0x80 == 0 && v&0x80 != 0 {
		m.reg = m.reg&0x87 | v&0x78
	}
	m.reg = m.reg&0x78 | v&0x87
}

func (m *FaridUnrom) prg0() int { return int(m.reg&0x07) | int(m.reg&0x70)>>1 }
func (m *FaridUnrom) prg1() int { return 0x07 | int(m.reg&0x70)>>1 }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *FaridUnrom) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, m.prg1(), 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, m.prg0(), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *FaridUnrom) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *FaridUnrom) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *FaridUnrom) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *FaridUnrom) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// Mapper43 (43, SMB2j-style pirate): fixed PRG layout with a $6000 window
// switched by a lookup, a $5000 ROM window, and a free-running 12-bit
// cycle IRQ.
type Mapper43 struct {
	base
	reg        int
	swap       bool
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewMapper43 constructs the corresponding board.
func NewMapper43(c *cartridge.Cartridge) *Mapper43 {
	return &Mapper43{base: makeBase(c), reg: 0}
}

var mapper43Lut = [8]int{4, 3, 5, 3, 6, 3, 7, 3}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper43) WritePRG(addr uint16, v byte) {
	switch addr & 0xF1FF {
	case 0x4022:
		m.reg = mapper43Lut[v&0x07]
	case 0x4120:
		m.swap = v&0x01 != 0
	case 0x8122, 0x4122:
		m.irqEnabled = v&0x01 != 0
		m.irqLine = false
		m.irqCounter = 0
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper43) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		if m.swap {
			return window(m.prg, 8, 0x2000)[addr&0x1FFF]
		}
		return window(m.prg, 9, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return window(m.prg, m.reg, 0x2000)[addr&0x1FFF]
	case addr >= 0xA000:
		return window(m.prg, 0, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, 1, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.swap {
			return window(m.prg, 0, 0x2000)[addr&0x1FFF]
		}
		return window(m.prg, 2, 0x2000)[addr&0x1FFF]
	case addr >= 0x5000:
		return window(m.prg, 8, 0x1000)[addr&0x0FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper43) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper43) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Tick advances the board by one cycle.
func (m *Mapper43) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter++
	if m.irqCounter >= 4096 {
		m.irqEnabled = false
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper43) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper43) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.reg)
	s.Regs[1] = boolByte(m.swap)
	s.Regs[2] = byte(m.irqCounter)
	s.Regs[3] = byte(m.irqCounter >> 8)
	s.Regs[4] = boolByte(m.irqEnabled)
	s.Regs[5] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper43) Restore(s *State) {
	m.restoreRAM(s)
	m.reg = int(s.Regs[0])
	m.swap = s.Regs[1] != 0
	m.irqCounter = uint16(s.Regs[2]) | uint16(s.Regs[3])<<8
	m.irqEnabled = s.Regs[4] != 0
	m.irqLine = s.Regs[5] != 0
}

// Eh8813A (mapper 519): an address-latched multicart with a 16/32 KiB PRG
// window and CHR bank; the DIP-modified protection read returns fixed data.
type Eh8813A struct {
	base
	prg0   int
	prg1   int
	chr    int
	mirror cartridge.Mirroring
}

// NewEh8813A constructs the corresponding board.
func NewEh8813A(c *cartridge.Cartridge) *Eh8813A {
	return &Eh8813A{base: makeBase(c), mirror: cartridge.Vertical}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Eh8813A) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	if addr&0x0100 == 0 {
		if addr&0x80 != 0 {
			p := int(addr & 0x07)
			m.prg0, m.prg1 = p, p
		} else {
			b := int(addr & 0x06)
			m.prg0, m.prg1 = b, b|1
		}
		m.chr = int(v & 0x0F)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Eh8813A) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Eh8813A) ReadCHR(addr uint16) byte { return m.chrRead(m.chr, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Eh8813A) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chr, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Eh8813A) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Eh8813A) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.prg1)
	s.Regs[2] = byte(m.chr)
}

// Restore loads the board's mapper-specific state from s.
func (m *Eh8813A) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.prg1 = int(s.Regs[1])
	m.chr = int(s.Regs[2])
}

// Lh10 (mapper 522): an index/data register pair banks two 8 KiB PRG
// windows ($8000/$A000); $6000 and $C000 are a fixed ROM window and a WRAM
// window, $E000 the last bank.
type Lh10 struct {
	base
	currentReg byte
	regs       [8]byte
}

// NewLh10 constructs the corresponding board.
func NewLh10(c *cartridge.Cartridge) *Lh10 {
	return &Lh10{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Lh10) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0xE001 {
	case 0x8000:
		m.currentReg = v & 0x07
	case 0x8001:
		m.regs[m.currentReg] = v
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Lh10) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.readPRGRAM(addr) // WRAM window at $C000-$DFFF
	case addr >= 0xA000:
		return window(m.prg, int(m.regs[7]), 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.regs[6]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return window(m.prg, -2, 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Lh10) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Lh10) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Lh10) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.currentReg
	copy(s.Regs[1:9], m.regs[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Lh10) Restore(s *State) {
	m.restoreRAM(s)
	m.currentReg = s.Regs[0]
	copy(m.regs[:], s.Regs[1:9])
}

// NsfCart31 (mapper 31): eight 4 KiB PRG windows selected from a $5000-
// region register file (address low 3 bits = slot).
type NsfCart31 struct {
	base
	prgReg [8]byte
}

// NewNsfCart31 constructs the corresponding board.
func NewNsfCart31(c *cartridge.Cartridge) *NsfCart31 {
	m := &NsfCart31{base: makeBase(c)}
	for i := range m.prgReg {
		m.prgReg[i] = 0xFF
	}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *NsfCart31) WritePRG(addr uint16, v byte) {
	if addr >= 0x5000 && addr < 0x6000 {
		m.prgReg[addr&0x07] = v
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *NsfCart31) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := (addr - 0x8000) >> 12
		return window(m.prg, int(m.prgReg[slot]), 0x1000)[addr&0x0FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *NsfCart31) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *NsfCart31) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *NsfCart31) Save(s *State) { m.saveRAM(s); copy(s.Regs[0:8], m.prgReg[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *NsfCart31) Restore(s *State) { m.restoreRAM(s); copy(m.prgReg[:], s.Regs[0:8]) }

// Yoko (mapper 264): four 8 KiB PRG windows (mode-dependent), four 2 KiB
// CHR windows, a bank/mode register pair and a down-counting cycle IRQ.
// The four expansion registers at $8000-$83FF are readable but their DIP
// side is not modelled.
type Yoko struct {
	base
	regs       [7]byte
	exRegs     [4]byte
	mode       byte
	bank       byte
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewYoko constructs the corresponding board.
func NewYoko(c *cartridge.Cartridge) *Yoko {
	return &Yoko{base: makeBase(c)}
}

// Reset returns the board to its power-on banking.
func (m *Yoko) Reset(soft bool) {
	if soft {
		m.mode = 0
		m.bank = 0
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Yoko) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x5400 && addr < 0x6000 {
			m.exRegs[addr&0x03] = v
		} else if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0x8C17 {
	case 0x8000:
		m.bank = v
	case 0x8400:
		m.mode = v
	case 0x8800:
		m.irqCounter = m.irqCounter&0xFF00 | uint16(v)
		m.irqLine = false
	case 0x8801:
		m.irqCounter = m.irqCounter&0x00FF | uint16(v)<<8
		m.irqEnabled = true
	case 0x8C00:
		m.regs[0] = v
	case 0x8C01:
		m.regs[1] = v
	case 0x8C02:
		m.regs[2] = v
	case 0x8C10:
		m.regs[3] = v
	case 0x8C11:
		m.regs[4] = v
	case 0x8C16:
		m.regs[5] = v
	case 0x8C17:
		m.regs[6] = v
	}
}

func (m *Yoko) prgBank(slot int) int {
	if m.mode&0x10 != 0 {
		outer := int(m.bank&0x08) << 1
		switch slot {
		case 0:
			return outer | int(m.regs[0]&0x0F)
		case 1:
			return outer | int(m.regs[1]&0x0F)
		case 2:
			return outer | int(m.regs[2]&0x0F)
		default:
			return outer | 0x0F
		}
	}
	if m.mode&0x08 != 0 {
		return (int(m.bank&0xFE) << 1) + slot
	}
	switch slot {
	case 0, 1:
		return int(m.bank)<<1 + slot
	default:
		return -2 + (slot - 2)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Yoko) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prgBank(int((addr-0x8000)>>13)), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x5400 && addr < 0x6000 {
		return m.exRegs[addr&0x03]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Yoko) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.regs[3+(addr>>11&3)]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Yoko) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.regs[3+(addr>>11&3)]), 0x800, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Yoko) Mirroring() cartridge.Mirroring { return hvMirror(m.mode&0x01 == 0) }

// Tick advances the board by one cycle.
func (m *Yoko) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter--
	if m.irqCounter == 0 {
		m.irqEnabled = false
		m.irqCounter = 0xFFFF
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Yoko) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Yoko) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:7], m.regs[:])
	copy(s.Regs[7:11], m.exRegs[:])
	s.Regs[11] = m.mode
	s.Regs[12] = m.bank
	s.Regs[13] = byte(m.irqCounter)
	s.Regs[14] = byte(m.irqCounter >> 8)
	s.Regs[15] = boolByte(m.irqEnabled)
	s.Regs[16] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Yoko) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.regs[:], s.Regs[0:7])
	copy(m.exRegs[:], s.Regs[7:11])
	m.mode = s.Regs[11]
	m.bank = s.Regs[12]
	m.irqCounter = uint16(s.Regs[13]) | uint16(s.Regs[14])<<8
	m.irqEnabled = s.Regs[15] != 0
	m.irqLine = s.Regs[16] != 0
}

// Mapper487 (487): a NINA-03/ColorDreams-hybrid multicart with 32 KiB PRG
// and 8 KiB CHR windows selected by two registers with 32/64 KiB modes.
type Mapper487 struct {
	base
	regs [2]byte
}

// NewMapper487 constructs the corresponding board.
func NewMapper487(c *cartridge.Cartridge) *Mapper487 {
	return &Mapper487{base: makeBase(c)}
}

// Reset returns the board to its power-on banking.
func (m *Mapper487) Reset(_ bool) { m.regs[0] = 0; m.regs[1] = 0 }

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper487) WritePRG(addr uint16, v byte) {
	if addr < 0x6000 {
		if addr&0x100 != 0 {
			if addr&0x80 != 0 {
				m.regs[1] = v
			} else if m.regs[1]&0x20 == 0 {
				m.regs[0] = v & 0x0F
			}
		}
		return
	}
	if addr < 0x8000 && m.regs[1]&0x20 != 0 {
		m.regs[0] = (v&0x01)<<3 | (v&0x70)>>4
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

func (m *Mapper487) banks() (prg, chr int) {
	prg = int(m.regs[1] & 0x1E)
	chr = int(m.regs[1]&0x1E)<<2 | int(m.regs[0]&0x03)
	if m.regs[1]&0x40 != 0 { // 64 KiB
		prg |= int(m.regs[0]&0x08) >> 3
		chr |= int(m.regs[0] & 0x04)
	} else { // 32 KiB
		prg |= int(m.regs[1] & 0x01)
		chr |= int(m.regs[1]&0x01) << 2
	}
	if m.regs[1]&0x20 != 0 {
		prg += 0x10
		chr += 0x40
	}
	return prg, chr
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper487) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		prg, _ := m.banks()
		return window(m.prg, prg, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper487) ReadCHR(addr uint16) byte {
	_, chr := m.banks()
	return m.chrRead(chr, 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper487) WriteCHR(addr uint16, v byte) {
	_, chr := m.banks()
	m.chrWrite(chr, 0x2000, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper487) Mirroring() cartridge.Mirroring { return hvMirror(m.regs[1]&0x80 == 0) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper487) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.regs[0]
	s.Regs[1] = m.regs[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper487) Restore(s *State) {
	m.restoreRAM(s)
	m.regs[0] = s.Regs[0]
	m.regs[1] = s.Regs[1]
}

// CityFighter (mapper 266): four 8 KiB PRG windows (one mode fixes the
// third), eight 1 KiB CHR windows loaded a nibble at a time, 4-way
// mirroring and a down-counting cycle IRQ. The board's DMC volume write
// to $4011 is a sound side effect and is not forwarded.
type CityFighter struct {
	base
	prgReg     byte
	prgMode    byte
	mir        byte
	chrRegs    [8]byte
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewCityFighter constructs the corresponding board.
func NewCityFighter(c *cartridge.Cartridge) *CityFighter {
	return &CityFighter{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *CityFighter) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0xF00C {
	case 0x9000:
		m.prgReg = v & 0x0C
		m.mir = v & 0x03
	case 0x9004, 0x9008, 0x900C:
		if addr&0x800 == 0 {
			m.prgReg = v & 0x0C
		}
	case 0xC000, 0xC004, 0xC008, 0xC00C:
		m.prgMode = v & 0x01
	case 0xD000:
		m.chrRegs[0] = m.chrRegs[0]&0xF0 | v&0x0F
	case 0xD004:
		m.chrRegs[0] = m.chrRegs[0]&0x0F | v<<4
	case 0xD008:
		m.chrRegs[1] = m.chrRegs[1]&0xF0 | v&0x0F
	case 0xD00C:
		m.chrRegs[1] = m.chrRegs[1]&0x0F | v<<4
	case 0xA000:
		m.chrRegs[2] = m.chrRegs[2]&0xF0 | v&0x0F
	case 0xA004:
		m.chrRegs[2] = m.chrRegs[2]&0x0F | v<<4
	case 0xA008:
		m.chrRegs[3] = m.chrRegs[3]&0xF0 | v&0x0F
	case 0xA00C:
		m.chrRegs[3] = m.chrRegs[3]&0x0F | v<<4
	case 0xB000:
		m.chrRegs[4] = m.chrRegs[4]&0xF0 | v&0x0F
	case 0xB004:
		m.chrRegs[4] = m.chrRegs[4]&0x0F | v<<4
	case 0xB008:
		m.chrRegs[5] = m.chrRegs[5]&0xF0 | v&0x0F
	case 0xB00C:
		m.chrRegs[5] = m.chrRegs[5]&0x0F | v<<4
	case 0xE000:
		m.chrRegs[6] = m.chrRegs[6]&0xF0 | v&0x0F
	case 0xE004:
		m.chrRegs[6] = m.chrRegs[6]&0x0F | v<<4
	case 0xE008:
		m.chrRegs[7] = m.chrRegs[7]&0xF0 | v&0x0F
	case 0xE00C:
		m.chrRegs[7] = m.chrRegs[7]&0x0F | v<<4
	case 0xF000:
		m.irqCounter = m.irqCounter&0x1E0 | uint16(v&0x0F)<<1
	case 0xF004:
		m.irqCounter = m.irqCounter&0x1E | uint16(v&0x0F)<<5
	case 0xF008:
		m.irqEnabled = v&0x02 != 0
		m.irqLine = false
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *CityFighter) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := int((addr - 0x8000) >> 13)
		var bank int
		if slot == 2 && m.prgMode == 0 {
			bank = int(m.prgReg)
		} else {
			bank = int(m.prgReg) + slot
		}
		return window(m.prg, bank, 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *CityFighter) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrRegs[addr>>10&7]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *CityFighter) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrRegs[addr>>10&7]), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *CityFighter) Mirroring() cartridge.Mirroring {
	switch m.mir {
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

// Tick advances the board by one cycle.
func (m *CityFighter) Tick() {
	if m.irqEnabled {
		m.irqCounter--
		if m.irqCounter == 0 {
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *CityFighter) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *CityFighter) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg
	s.Regs[1] = m.prgMode
	s.Regs[2] = m.mir
	copy(s.Regs[3:11], m.chrRegs[:])
	s.Regs[11] = byte(m.irqCounter)
	s.Regs[12] = byte(m.irqCounter >> 8)
	s.Regs[13] = boolByte(m.irqEnabled)
	s.Regs[14] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *CityFighter) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg = s.Regs[0]
	m.prgMode = s.Regs[1]
	m.mir = s.Regs[2]
	copy(m.chrRegs[:], s.Regs[3:11])
	m.irqCounter = uint16(s.Regs[11]) | uint16(s.Regs[12])<<8
	m.irqEnabled = s.Regs[13] != 0
	m.irqLine = s.Regs[14] != 0
}

// OekaKids (mapper 96, Bandai OEKA KIDS): a 32 KiB PRG bank plus a CHR
// scheme where the low 4 KiB window's inner bank is latched from the
// nametable-fetch address (bits 8-9), watched via NotifyVramAddr. Bus
// conflicts apply to the register write.
type OekaKids struct {
	base
	prg0     int
	outerChr int
	innerChr int
	lastAddr uint16
}

// NewOekaKids constructs the corresponding board.
func NewOekaKids(c *cartridge.Cartridge) *OekaKids {
	return &OekaKids{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *OekaKids) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		// Bus conflict: the ROM byte ANDs the written value.
		v &= window(m.prg, m.prg0, 0x8000)[addr&0x7FFF]
		m.prg0 = int(v & 0x03)
		m.outerChr = int(v&0x04) << 2 // -> bit 4 of the 4 KiB CHR bank
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// NotifyVramAddr latches the inner CHR bank when a nametable fetch begins.
func (m *OekaKids) NotifyVramAddr(addr uint16) {
	if m.lastAddr&0x3000 != 0x2000 && addr&0x3000 == 0x2000 {
		m.innerChr = int(addr>>8) & 0x03
	}
	m.lastAddr = addr
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *OekaKids) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prg0, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// chr4K returns the 4 KiB CHR bank for a pattern-table half.
func (m *OekaKids) chr4K(addr uint16) int {
	if addr < 0x1000 {
		return m.outerChr | m.innerChr
	}
	return m.outerChr | 0x03
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *OekaKids) ReadCHR(addr uint16) byte { return m.chrRead(m.chr4K(addr), 0x1000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *OekaKids) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chr4K(addr), 0x1000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *OekaKids) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.outerChr)
	s.Regs[2] = byte(m.innerChr)
	s.Regs[3] = byte(m.lastAddr)
	s.Regs[4] = byte(m.lastAddr >> 8)
}

// Restore loads the board's mapper-specific state from s.
func (m *OekaKids) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.outerChr = int(s.Regs[1])
	m.innerChr = int(s.Regs[2])
	m.lastAddr = uint16(s.Regs[3]) | uint16(s.Regs[4])<<8
}

// Dance2000 (mapper 518): a 16/32 KiB PRG board where, in one mode, the
// low CHR-RAM 4 KiB window follows the current nametable (watched via
// NotifyVramAddr). PRG is banked from a $5000 register; mode from $5200.
type Dance2000 struct {
	base
	prgReg byte
	mode   byte
	lastNt byte
}

// NewDance2000 constructs the corresponding board.
func NewDance2000(c *cartridge.Cartridge) *Dance2000 {
	return &Dance2000{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Dance2000) WritePRG(addr uint16, v byte) {
	switch addr {
	case 0x5000:
		m.prgReg = v
	case 0x5200:
		m.mode = v
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// NotifyVramAddr informs the board of a PPU VRAM address change.
func (m *Dance2000) NotifyVramAddr(addr uint16) {
	if m.mode&0x02 != 0 {
		if addr&0x3000 == 0x2000 {
			nt := byte(addr>>11) & 0x01
			if nt != m.lastNt {
				m.lastNt = nt
			}
		}
	} else if m.lastNt != 0 {
		m.lastNt = 0
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Dance2000) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		if m.mode&0x04 != 0 { // 32 KiB
			bank := int(m.prgReg&0x07)<<1 | int(addr>>14&1)
			return window(m.prg, bank, 0x4000)[addr&0x3FFF]
		}
		// 16 KiB switchable at $8000, bank 0 fixed at $C000.
		if addr < 0xC000 {
			return window(m.prg, int(m.prgReg&0x0F), 0x4000)[addr&0x3FFF]
		}
		return window(m.prg, 0, 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// chr4K: window 0 follows the tracked nametable; window 1 is fixed to 1.
func (m *Dance2000) chr4K(addr uint16) int {
	if addr < 0x1000 {
		return int(m.lastNt)
	}
	return 1
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Dance2000) ReadCHR(addr uint16) byte { return m.chrRead(m.chr4K(addr), 0x1000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Dance2000) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chr4K(addr), 0x1000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Dance2000) Mirroring() cartridge.Mirroring { return hvMirror(m.mode&0x01 == 0) }

// Save writes the board's mapper-specific state into s.
func (m *Dance2000) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgReg
	s.Regs[1] = m.mode
	s.Regs[2] = m.lastNt
}

// Restore loads the board's mapper-specific state from s.
func (m *Dance2000) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg = s.Regs[0]
	m.mode = s.Regs[1]
	m.lastNt = s.Regs[2]
}

// Mapper103 (103, an FDS-to-cart conversion): 16 KiB of work RAM in two
// 8 KiB banks - one at $6000-$7FFF and one mirrored into $B800-$D7FF -
// plus a mode where the $6000 window becomes a switchable PRG-ROM bank.
// The upper 32 KiB is fixed to the last four 8 KiB banks.
type Mapper103 struct {
	base
	wram      [0x4000]byte // 16 KiB banked work RAM
	prgReg    byte
	ramDisabl bool
	mirror    cartridge.Mirroring
}

// NewMapper103 constructs the corresponding board.
func NewMapper103(c *cartridge.Cartridge) *Mapper103 {
	return &Mapper103{base: makeBase(c), mirror: cartridge.Vertical}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper103) WritePRG(addr uint16, v byte) {
	switch addr & 0xF000 {
	case 0x6000, 0x7000:
		// Work RAM is always writeable, even when PRG ROM is mapped here.
		m.wram[addr-0x6000] = v
	case 0x8000:
		m.prgReg = v & 0x0F
	case 0xB000, 0xC000, 0xD000:
		if addr >= 0xB800 && addr < 0xD800 {
			m.wram[0x2000+(addr-0xB800)] = v
		}
	case 0xE000:
		m.mirror = hvMirror(v&0x08 == 0)
	case 0xF000:
		m.ramDisabl = v&0x10 != 0
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper103) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		// $8000-$FFFF is the fixed last 32 KiB, with the $B800-$D7FF hole
		// served from work RAM bank 1 when RAM is enabled.
		if !m.ramDisabl && addr >= 0xB800 && addr < 0xD800 {
			return m.wram[0x2000+(addr-0xB800)]
		}
		n := len(m.prg) / 0x2000
		slot := int((addr - 0x8000) >> 13) // 0..3
		return window(m.prg, n-4+slot, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.ramDisabl {
			return window(m.prg, int(m.prgReg), 0x2000)[addr&0x1FFF]
		}
		return m.wram[addr-0x6000]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper103) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper103) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper103) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Mapper103) Save(s *State) {
	// This board's 16 KiB work RAM lives in its own buffer; store it in
	// the roomy State.PRGRAM (32 KiB) rather than base.prgRAM.
	s.CHRRAM = m.chrRAM
	copy(s.PRGRAM[:0x4000], m.wram[:])
	s.Regs[0] = m.prgReg
	s.Regs[1] = boolByte(m.ramDisabl)
	s.Regs[2] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper103) Restore(s *State) {
	m.chrRAM = s.CHRRAM
	copy(m.wram[:], s.PRGRAM[:0x4000])
	m.prgReg = s.Regs[0]
	m.ramDisabl = s.Regs[1] != 0
	m.mirror = cartridge.Mirroring(s.Regs[2])
}

// Mapper83 (83, "Cony/Yoko" BMC): eight 1 KiB CHR windows (or four 2 KiB
// in one mode), several PRG layouts driven by a mode register and an outer
// bank, 4-way mirroring, a down-counting cycle IRQ, and four read/write
// scratch registers at $5100-$5103. DIP reads return 0.
type Mapper83 struct {
	base
	regs       [11]byte
	exRegs     [4]byte
	is2k       bool
	isNot2k    bool
	mode       byte
	bank       byte
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewMapper83 constructs the corresponding board.
func NewMapper83(c *cartridge.Cartridge) *Mapper83 {
	return &Mapper83{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper83) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x5100 && addr <= 0x5103:
		m.exRegs[addr&0x03] = v
	case addr < 0x8000:
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
	case addr >= 0x8300 && addr <= 0x8302:
		m.mode &= 0xBF
		m.regs[addr-0x8300+8] = v
	case addr >= 0x8310 && addr <= 0x8317:
		m.regs[addr-0x8310] = v
		if addr >= 0x8312 && addr <= 0x8315 {
			m.isNot2k = true
		}
	default:
		switch addr {
		case 0x8000:
			m.is2k = true
			m.bank = v
			m.mode |= 0x40
		case 0xB000, 0xB0FF, 0xB1FF:
			m.bank = v
			m.mode |= 0x40
		case 0x8100:
			m.mode = v | (m.mode & 0x40)
		case 0x8200:
			m.irqCounter = m.irqCounter&0xFF00 | uint16(v)
			m.irqLine = false
		case 0x8201:
			m.irqEnabled = m.mode&0x80 != 0
			m.irqCounter = m.irqCounter&0x00FF | uint16(v)<<8
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper83) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x5100 && addr <= 0x5103:
		return m.exRegs[addr&0x03]
	case addr >= 0x8000:
		slot := int((addr - 0x8000) >> 13)
		var bank int
		if m.mode&0x40 != 0 {
			if slot < 2 {
				bank = (int(m.bank&0x3F) << 1) + slot
			} else {
				bank = ((int(m.bank&0x30) | 0x0F) << 1) + (slot - 2)
			}
		} else {
			switch slot {
			case 0:
				bank = int(m.regs[8])
			case 1:
				bank = int(m.regs[9])
			case 2:
				bank = int(m.regs[10])
			default:
				bank = -1
			}
		}
		return window(m.prg, bank, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

func (m *Mapper83) chrBank(addr uint16) int {
	if m.is2k && !m.isNot2k {
		win := addr >> 11 & 3
		reg := []byte{m.regs[0], m.regs[1], m.regs[6], m.regs[7]}[win]
		return int(reg)<<1 + int(addr>>10&1)
	}
	i := addr >> 10 & 7
	return int(m.regs[i]) | int(m.bank&0x30)<<4
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper83) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper83) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank(addr), 0x400, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper83) Mirroring() cartridge.Mirroring {
	switch m.mode & 0x03 {
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

// Tick advances the board by one cycle.
func (m *Mapper83) Tick() {
	if !m.irqEnabled {
		return
	}
	m.irqCounter--
	if m.irqCounter == 0 {
		m.irqEnabled = false
		m.irqCounter = 0xFFFF
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Mapper83) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Mapper83) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:11], m.regs[:])
	copy(s.Regs[11:15], m.exRegs[:])
	s.Regs[15] = boolByte(m.is2k)
	s.Regs[16] = boolByte(m.isNot2k)
	s.Regs[17] = m.mode
	s.Regs[18] = m.bank
	s.Regs[19] = byte(m.irqCounter)
	s.Regs[20] = byte(m.irqCounter >> 8)
	s.Regs[21] = boolByte(m.irqEnabled)
	s.Regs[22] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper83) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.regs[:], s.Regs[0:11])
	copy(m.exRegs[:], s.Regs[11:15])
	m.is2k = s.Regs[15] != 0
	m.isNot2k = s.Regs[16] != 0
	m.mode = s.Regs[17]
	m.bank = s.Regs[18]
	m.irqCounter = uint16(s.Regs[19]) | uint16(s.Regs[20])<<8
	m.irqEnabled = s.Regs[21] != 0
	m.irqLine = s.Regs[22] != 0
}

// MagicFloor218 (mapper 218): a board with no PRG/CHR ROM banking - PRG is
// fixed and the CHR $0000-$1FFF is wired straight to the console's 2 KiB
// nametable RAM. Which of the two CIRAM pages each 1 KiB CHR window sees is
// fixed at power-on by the solder-pad mirroring. Ported from the reference emulator.
type MagicFloor218 struct {
	base
	mask       uint16
	ciramRead  func(idx uint16) byte
	ciramWrite func(idx uint16, v byte)
}

// NewMagicFloor218 constructs the corresponding board.
func NewMagicFloor218(c *cartridge.Cartridge) *MagicFloor218 {
	m := &MagicFloor218{base: makeBase(c)}
	// The CHR-to-CIRAM page mask is chosen by the fixed mirroring; a
	// four-screen header byte 6 bit 0 picks screen A/B.
	switch c.Mirroring {
	case cartridge.Vertical:
		m.mask = 0x400
	case cartridge.Horizontal:
		m.mask = 0x800
	case cartridge.SingleLow:
		m.mask = 0x1000
	case cartridge.SingleHigh:
		m.mask = 0x2000
	default:
		m.mask = 0x800
	}
	return m
}

// SetCIRAM receives the console VRAM accessors.
func (m *MagicFloor218) SetCIRAM(read func(idx uint16) byte, write func(idx uint16, v byte)) {
	m.ciramRead = read
	m.ciramWrite = write
}

// ciramIndex maps a CHR address to a 2 KiB CIRAM index: the 1 KiB window's
// page (0 or 1) times 0x400 plus the offset within it.
func (m *MagicFloor218) ciramIndex(addr uint16) uint16 {
	page := uint16(0)
	if addr&m.mask != 0 {
		page = 1
	}
	return page*0x400 | (addr & 0x3FF)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MagicFloor218) ReadCHR(addr uint16) byte {
	if m.ciramRead != nil {
		return m.ciramRead(m.ciramIndex(addr))
	}
	return 0
}

// WriteCHR handles a write into the CHR address space.
func (m *MagicFloor218) WriteCHR(addr uint16, v byte) {
	if m.ciramWrite != nil {
		m.ciramWrite(m.ciramIndex(addr), v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MagicFloor218) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MagicFloor218) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *MagicFloor218) Save(s *State) { m.saveRAM(s) }

// Restore loads the board's mapper-specific state from s.
func (m *MagicFloor218) Restore(s *State) { m.restoreRAM(s) }
