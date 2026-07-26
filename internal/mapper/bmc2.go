package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// A large batch of simple address/value-latched multicart boards ported
// from the reference emulator. Most reuse the shared dualPRG16 (two 16 KiB PRG windows + one
// 8 KiB CHR window + H/V mirroring) helper.

// Mapper214 (214): value bits pick a 16 KiB PRG bank (mirrored) and an
// 8 KiB CHR bank.
type Mapper214 struct{ dualPRG16 }

// NewMapper214 constructs the corresponding board.
func NewMapper214(c *cartridge.Cartridge) *Mapper214 {
	m := &Mapper214{dualPRG16{base: makeBase(c), mirror: c.Mirroring}}
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper214) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		p := int(addr>>2) & 0x03
		m.prg0, m.prg1 = p, p
		m.chrBank = int(addr & 0x03)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper214) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper214) Restore(s *State) { m.restoreDual(s) }

// Bmc190in1 (300): value bits pick a 16 KiB PRG bank (mirrored), an 8 KiB
// CHR bank and H/V mirroring.
type Bmc190in1 struct{ dualPRG16 }

// NewBmc190in1 constructs the corresponding board.
func NewBmc190in1(c *cartridge.Cartridge) *Bmc190in1 {
	return &Bmc190in1{dualPRG16{base: makeBase(c), mirror: c.Mirroring}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc190in1) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		p := int(v>>2) & 0x07
		m.prg0, m.prg1 = p, p
		m.chrBank = p
		m.mirror = hvMirror(v&0x01 == 0)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Bmc190in1) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Bmc190in1) Restore(s *State) { m.restoreDual(s) }

// BmcNtd03 (290): the write address encodes a 16/32 KiB PRG bank, a CHR
// bank and mirroring.
type BmcNtd03 struct{ dualPRG16 }

// NewBmcNtd03 constructs the corresponding board.
func NewBmcNtd03(c *cartridge.Cartridge) *BmcNtd03 {
	m := &BmcNtd03{dualPRG16{base: makeBase(c), mirror: c.Mirroring}}
	m.decode(0x8000)
	return m
}

func (m *BmcNtd03) decode(addr uint16) {
	prg := int(addr>>10) & 0x1E
	chr := int(addr&0x0300)>>5 | int(addr&0x07)
	if addr&0x80 != 0 {
		p := prg | int(addr>>6)&1
		m.prg0, m.prg1 = p, p
	} else {
		m.prg0, m.prg1 = prg&0xFE, prg&0xFE|1
	}
	m.chrBank = chr
	m.mirror = hvMirror(addr&0x400 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *BmcNtd03) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *BmcNtd03) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *BmcNtd03) Restore(s *State) { m.restoreDual(s) }

// Bmc810544CA1 (261): the write address encodes a 16/32 KiB PRG bank, a
// CHR bank and mirroring.
type Bmc810544CA1 struct{ dualPRG16 }

// NewBmc810544CA1 constructs the corresponding board.
func NewBmc810544CA1(c *cartridge.Cartridge) *Bmc810544CA1 {
	m := &Bmc810544CA1{dualPRG16{base: makeBase(c), mirror: c.Mirroring}}
	m.decode(0x8000)
	return m
}

func (m *Bmc810544CA1) decode(addr uint16) {
	bank := int(addr>>6) & 0xFFFE
	if addr&0x40 != 0 {
		m.prg0, m.prg1 = bank, bank|1
	} else {
		p := bank | int(addr>>5)&0x01
		m.prg0, m.prg1 = p, p
	}
	m.chrBank = int(addr & 0x0F)
	m.mirror = hvMirror(addr&0x10 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc810544CA1) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Bmc810544CA1) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Bmc810544CA1) Restore(s *State) { m.restoreDual(s) }

// BmcG146 (349): the write address encodes several PRG layouts and
// mirroring; CHR is fixed 8 KiB RAM.
type BmcG146 struct{ dualPRG16 }

// NewBmcG146 constructs the corresponding board.
func NewBmcG146(c *cartridge.Cartridge) *BmcG146 {
	m := &BmcG146{dualPRG16{base: makeBase(c), mirror: c.Mirroring}}
	m.decode(0x8000)
	return m
}

func (m *BmcG146) decode(addr uint16) {
	switch {
	case addr&0x800 != 0:
		m.prg0 = int(addr&0x1F) | int(addr&(addr&0x40)>>6)
		m.prg1 = int(addr&0x18) | 0x07
	case addr&0x40 != 0:
		p := int(addr & 0x1F)
		m.prg0, m.prg1 = p, p
	default:
		p := int(addr & 0x1E)
		m.prg0, m.prg1 = p, p|1
	}
	m.mirror = hvMirror(addr&0x80 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *BmcG146) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *BmcG146) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *BmcG146) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *BmcG146) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *BmcG146) Restore(s *State) { m.restoreDual(s) }

// Bmc11160 (299): a 32 KiB PRG bank plus CHR, from one value.
type Bmc11160 struct {
	base
	prgBank int
	chrBank int
	mirror  cartridge.Mirroring
}

// NewBmc11160 constructs the corresponding board.
func NewBmc11160(c *cartridge.Cartridge) *Bmc11160 {
	return &Bmc11160{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc11160) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		bank := int(v>>4) & 0x07
		m.prgBank = bank
		m.chrBank = bank<<2 | int(v&0x03)
		if v&0x80 != 0 {
			m.mirror = cartridge.Vertical
		} else {
			m.mirror = cartridge.Horizontal
		}
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc11160) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc11160) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc11160) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc11160) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc11160) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
	s.Regs[2] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc11160) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
	m.mirror = cartridge.Mirroring(s.Regs[2])
}

// BmcK3046 (336): value bits form an inner/outer PRG split; last bank of
// the block fixed at $C000. CHR is 8 KiB RAM.
type BmcK3046 struct {
	base
	prg0 int
	prg1 int
}

// NewBmcK3046 constructs the corresponding board.
func NewBmcK3046(c *cartridge.Cartridge) *BmcK3046 {
	return &BmcK3046{base: makeBase(c), prg0: 0, prg1: 7}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *BmcK3046) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		inner := int(v & 0x07)
		outer := int(v & 0x38)
		m.prg0 = outer | inner
		m.prg1 = outer | 7
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *BmcK3046) ReadPRG(addr uint16) byte {
	if addr >= 0xC000 {
		return m.win(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x8000 {
		return m.win(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *BmcK3046) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *BmcK3046) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *BmcK3046) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.prg1)
}

// Restore loads the board's mapper-specific state from s.
func (m *BmcK3046) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0])
	m.prg1 = int(s.Regs[1])
}

// Mapper214 and friends handle their own $6000 window; Mapper246 banks a
// PRG-ROM window there.

// Mapper246 (246): four 8 KiB PRG windows ($6000-$FFFF, last fixed) and
// four 2 KiB CHR windows, all register-selected from $6000-$67FF.
type Mapper246 struct {
	base
	prgReg [4]byte
	chrReg [4]byte
}

// NewMapper246 constructs the corresponding board.
func NewMapper246(c *cartridge.Cartridge) *Mapper246 {
	m := &Mapper246{base: makeBase(c)}
	m.prgReg[3] = 0xFF
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper246) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr <= 0x67FF {
		if addr&0x07 <= 0x03 {
			m.prgReg[addr&0x03] = v
		} else {
			m.chrReg[addr&0x03] = v
		}
		return
	}
	if addr >= 0x6800 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper246) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		slot := (addr - 0x8000) >> 13
		return m.win(m.prg, int(m.prgReg[slot]), 0x2000)[addr&0x1FFF]
	}
	if addr >= 0x6800 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper246) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrReg[addr>>11&3]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper246) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrReg[addr>>11&3]), 0x800, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper246) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.prgReg[:])
	copy(s.Regs[4:8], m.chrReg[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper246) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgReg[:], s.Regs[0:4])
	copy(m.chrReg[:], s.Regs[4:8])
}

// Mapper120 (120): a $41FF register banks a PRG-ROM window at $6000; the
// upper 32 KiB is fixed to banks 8-11.
type Mapper120 struct {
	base
	prgReg byte
}

// NewMapper120 constructs the corresponding board.
func NewMapper120(c *cartridge.Cartridge) *Mapper120 {
	return &Mapper120{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper120) WritePRG(addr uint16, v byte) {
	if addr == 0x41FF {
		m.prgReg = v & 0x07
	}
	// $6000-$7FFF is PRG ROM on this board, so writes there are ignored.
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper120) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		slot := int((addr - 0x8000) >> 13)
		return m.win(m.prg, 8+slot, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.win(m.prg, int(m.prgReg), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper120) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper120) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper120) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.prgReg }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper120) Restore(s *State) { m.restoreRAM(s); m.prgReg = s.Regs[0] }

// Mapper212 (212): address bits pick a 16/32 KiB PRG bank, an 8 KiB CHR
// bank and mirroring; a read in $6000-$7FFF returns bit 7 set (a menu
// detection quirk).
type Mapper212 struct{ dualPRG16 }

// NewMapper212 constructs the corresponding board.
func NewMapper212(c *cartridge.Cartridge) *Mapper212 {
	m := &Mapper212{dualPRG16{base: makeBase(c), mirror: c.Mirroring}}
	m.decode(0x8000)
	return m
}

func (m *Mapper212) decode(addr uint16) {
	if addr&0x4000 != 0 {
		b := int(addr & 0x06)
		m.prg0, m.prg1 = b, b|1
	} else {
		p := int(addr & 0x07)
		m.prg0, m.prg1 = p, p
	}
	m.chrBank = int(addr & 0x07)
	m.mirror = hvMirror(addr&0x08 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper212) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper212) ReadPRG(addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 && addr&0xE010 == 0x6000 {
		return m.openBus() | 0x80
	}
	return m.dualPRG16.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper212) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper212) Restore(s *State) { m.restoreDual(s) }

// Bmc63 (mapper 63, "NTDEC 22-in-1"): eight 8 KiB PRG windows computed
// from the write address; some address bits force the $8000-$BFFF window
// to open bus.
type Bmc63 struct {
	base
	prgBanks [4]int
	openBus  bool
	mirror   cartridge.Mirroring
}

// NewBmc63 constructs the corresponding board.
func NewBmc63(c *cartridge.Cartridge) *Bmc63 {
	m := &Bmc63{base: makeBase(c)}
	m.decode(0x8000)
	return m
}

func (m *Bmc63) bit(cond bool, a int) int {
	if cond {
		return a
	}
	return 0
}

func (m *Bmc63) decode(addr uint16) {
	m.openBus = addr&0x0300 == 0x0300
	base := int(addr>>1) & 0x1FC
	a2 := addr & 0x02
	m.prgBanks[0] = base | m.bit(a2 == 0, int(addr>>1)&0x02)
	m.prgBanks[1] = base | 0x01 | m.bit(a2 == 0, int(addr>>1)&0x02)
	m.prgBanks[2] = base | 0x02 | m.bit(a2 == 0, int(addr>>1)&0x02)
	if addr&0x800 != 0 {
		hi := int(addr) & 0x07C
		if addr&0x06 != 0 {
			m.prgBanks[3] = hi | 0x03
		} else {
			m.prgBanks[3] = hi | 0x01
		}
	} else {
		lo := int(addr>>1) & 0x01FC
		if a2 != 0 {
			m.prgBanks[3] = lo | 0x03
		} else {
			m.prgBanks[3] = lo | 0x01 | (int(addr>>1) & 0x02)
		}
	}
	m.mirror = hvMirror(addr&0x01 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc63) WritePRG(addr uint16, _ byte) {
	if addr >= 0x8000 {
		m.decode(addr)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc63) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.openBusVal
	}
	slot := int((addr - 0x8000) >> 13)
	if slot == 0 && m.openBus {
		return m.openBusVal
	}
	return m.win(m.prg, m.prgBanks[slot], 0x2000)[addr&0x1FFF]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc63) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Bmc63) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc63) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc63) Save(s *State) {
	m.saveRAM(s)
	for i := 0; i < 4; i++ {
		s.Regs[i*2] = byte(m.prgBanks[i])
		s.Regs[i*2+1] = byte(m.prgBanks[i] >> 8)
	}
	s.Regs[8] = boolByte(m.openBus)
	s.Regs[9] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc63) Restore(s *State) {
	m.restoreRAM(s)
	for i := 0; i < 4; i++ {
		m.prgBanks[i] = int(s.Regs[i*2]) | int(s.Regs[i*2+1])<<8
	}
	m.openBus = s.Regs[8] != 0
	m.mirror = cartridge.Mirroring(s.Regs[9])
}

// Bmc70in1 (mapper 236, "Game 800-in-1"): an outer/inner PRG register pair
// with NROM/UNROM sub-modes and a CHR bank; the DIP-modified read is not
// modelled.
type Bmc70in1 struct {
	base
	bankMode  byte
	outerBank byte
	prgReg    byte
	chrReg    byte
	useOuter  bool
	mirror    cartridge.Mirroring
}

// NewBmc70in1 constructs the corresponding board.
func NewBmc70in1(c *cartridge.Cartridge) *Bmc70in1 {
	m := &Bmc70in1{base: makeBase(c), mirror: c.Mirroring}
	m.useOuter = m.chr == nil
	return m
}

// Reset returns the board to its power-on banking.
func (m *Bmc70in1) Reset(_ bool) { m.bankMode = 0; m.outerBank = 0 }

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bmc70in1) WritePRG(addr uint16, _ byte) {
	if addr < 0x8000 {
		return
	}
	if addr&0x4000 != 0 {
		m.bankMode = byte(addr) & 0x30
		m.prgReg = byte(addr) & 0x07
	} else {
		m.mirror = hvMirror(addr&0x20 == 0)
		if m.useOuter {
			m.outerBank = byte(addr&0x03) << 3
		} else {
			m.chrReg = byte(addr) & 0x07
		}
	}
}

func (m *Bmc70in1) prgBank(slot int) int {
	p := int(m.outerBank | m.prgReg)
	switch m.bankMode {
	case 0x00, 0x10:
		if slot == 0 {
			return p
		}
		return int(m.outerBank) | 7
	case 0x20:
		return (p & 0xFE) + slot
	default: // 0x30
		return p
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bmc70in1) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank(int((addr-0x8000)>>14)), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bmc70in1) ReadCHR(addr uint16) byte {
	if m.useOuter {
		return m.chrRead(0, 0x2000, addr)
	}
	return m.chrRead(int(m.chrReg), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Bmc70in1) WriteCHR(addr uint16, v byte) {
	if m.useOuter {
		m.chrWrite(0, 0x2000, addr, v)
	} else {
		m.chrWrite(int(m.chrReg), 0x2000, addr, v)
	}
}

// Mirroring reports the board's current nametable mirroring.
func (m *Bmc70in1) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Bmc70in1) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bankMode
	s.Regs[1] = m.outerBank
	s.Regs[2] = m.prgReg
	s.Regs[3] = m.chrReg
	s.Regs[4] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bmc70in1) Restore(s *State) {
	m.restoreRAM(s)
	m.bankMode = s.Regs[0]
	m.outerBank = s.Regs[1]
	m.prgReg = s.Regs[2]
	m.chrReg = s.Regs[3]
	m.mirror = cartridge.Mirroring(s.Regs[4])
}
