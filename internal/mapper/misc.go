package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// CpROM (mapper 13, Videomation): fixed 32 KiB PRG, 16 KiB of CHR RAM
// split into a fixed and a switchable 4 KiB window.
type CpROM struct {
	base

	chrBank byte
	chrRAM2 [8192]byte // second half of the 16 KiB CHR RAM
}

// NewCpROM wires a CPROM board.
func NewCpROM(c *cartridge.Cartridge) *CpROM {
	m := &CpROM{base: makeBase(c)}
	m.mirroring = cartridge.Vertical
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *CpROM) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *CpROM) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.chrBank = v & 0x03
	}
}

// chrSlot resolves a PPU address to one of the four 4 KiB CHR RAM pages.
func (m *CpROM) chrSlot(addr uint16) []byte {
	page := 0
	if addr >= 0x1000 {
		page = int(m.chrBank)
	}
	if page < 2 {
		return m.chrRAM[page*0x1000:]
	}
	return m.chrRAM2[(page-2)*0x1000:]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *CpROM) ReadCHR(addr uint16) byte { return m.chrSlot(addr)[addr&0xFFF] }

// WriteCHR handles a write into the CHR address space.
func (m *CpROM) WriteCHR(addr uint16, v byte) { m.chrSlot(addr)[addr&0xFFF] = v }

// Save writes the board's mapper-specific state into s.
func (m *CpROM) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.chrBank
	copy(s.PRGRAM[:8192], m.chrRAM2[:]) // PRG RAM is unused on this board; reuse the slot
}

// Restore loads the board's mapper-specific state from s.
func (m *CpROM) Restore(s *State) {
	m.restoreRAM(s)
	m.chrBank = s.Regs[0]
	copy(m.chrRAM2[:], s.PRGRAM[:8192])
}

// Mapper15 (Contra 100-in-1 multicart): 8 KiB PRG pages driven by one
// register with four layout modes, and CHR RAM write protection in
// modes 0 and 3.
type Mapper15 struct {
	base

	mode  byte
	value byte
}

// NewMapper15 wires the board.
func NewMapper15(c *cartridge.Cartridge) *Mapper15 {
	return &Mapper15{base: makeBase(c)}
}

func (m *Mapper15) prgBank(addr uint16) int {
	slot := int((addr - 0x8000) >> 13)
	subBank := int(m.value >> 7)
	bank := int(m.value&0x7F) << 1
	switch m.mode {
	case 0:
		return (bank + slot) ^ subBank
	case 1, 3:
		b := bank | subBank
		if slot < 2 {
			return b + slot
		}
		if m.mode == 1 {
			b = (b | 0x0E) | subBank
		}
		return b + (slot - 2)
	default: // 2
		return bank | subBank
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper15) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper15) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.mode = byte(addr & 0x03)
		m.value = v
		if v&0x40 != 0 {
			m.mirroring = cartridge.Horizontal
		} else {
			m.mirroring = cartridge.Vertical
		}
	}
}

func (m *Mapper15) chrWritable() bool { return m.mode == 1 || m.mode == 2 }

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper15) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper15) WriteCHR(addr uint16, v byte) {
	if m.chrWritable() {
		m.chrWrite(0, 0x2000, addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper15) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.mode
	s.Regs[1] = m.value
	s.Regs[2] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper15) Restore(s *State) {
	m.restoreRAM(s)
	m.mode = s.Regs[0]
	m.value = s.Regs[1]
	m.mirroring = cartridge.Mirroring(s.Regs[2])
}

// PCI556 (mapper 38, Bit Corp. crime Busters): 32 KiB PRG and 8 KiB CHR
// banks from one register at $7000-$7FFF.
type PCI556 struct {
	base

	reg byte
}

// NewPCI556 wires the board.
func NewPCI556(c *cartridge.Cartridge) *PCI556 {
	return &PCI556{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *PCI556) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.reg&0x03), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *PCI556) WritePRG(addr uint16, v byte) {
	if addr >= 0x7000 && addr < 0x8000 {
		m.reg = v
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *PCI556) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.reg>>2)&0x03, 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *PCI556) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.reg>>2)&0x03, 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *PCI556) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *PCI556) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// Bandai74161 (mappers 70 and 152): one 16 KiB PRG bank, one 8 KiB CHR
// bank and (on 152, or once a game uses it) one-screen mirroring.
type Bandai74161 struct {
	base

	mirrorControl bool
	prgBank       byte
	chrBank       byte
}

// NewBandai74161 wires the board; mapper 152 enables mirroring control
// from the start.
func NewBandai74161(c *cartridge.Cartridge) *Bandai74161 {
	m := &Bandai74161{base: makeBase(c), mirrorControl: c.MapperID == 152}
	// Kamen Rider Club has a bad header; the board is wired vertical.
	m.mirroring = cartridge.Vertical
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Bandai74161) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Bandai74161) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	if v&0x80 != 0 {
		// A game that sets the bit is assumed to use mirroring switching.
		m.mirrorControl = true
	}
	if m.mirrorControl {
		if v&0x80 != 0 {
			m.mirroring = cartridge.SingleHigh
		} else {
			m.mirroring = cartridge.SingleLow
		}
	}
	m.prgBank = (v >> 4) & 0x07
	m.chrBank = v & 0x0F
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Bandai74161) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Bandai74161) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Bandai74161) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
	s.Regs[2] = boolByte(m.mirrorControl)
	s.Regs[3] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *Bandai74161) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.mirrorControl = s.Regs[2] != 0
	m.mirroring = cartridge.Mirroring(s.Regs[3])
}

// IremLROG017 (mapper 77, Napoleon Senki): 32 KiB PRG bank and a 2 KiB
// CHR ROM window, with 6 KiB of CHR RAM filling the rest of the pattern
// space and four-screen VRAM. Has bus conflicts.
type IremLROG017 struct {
	base

	reg byte
}

// NewIremLROG017 wires the board.
func NewIremLROG017(c *cartridge.Cartridge) *IremLROG017 {
	m := &IremLROG017{base: makeBase(c)}
	m.mirroring = cartridge.FourScreen
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *IremLROG017) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.reg&0x0F), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *IremLROG017) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		v &= m.ReadPRG(addr) // bus conflict
		m.reg = v
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *IremLROG017) ReadCHR(addr uint16) byte {
	if addr < 0x800 {
		return window(m.chr, int(m.reg>>4)&0x0F, 0x800)[addr&0x7FF]
	}
	return m.chrRAM[addr-0x800]
}

// WriteCHR handles a write into the CHR address space.
func (m *IremLROG017) WriteCHR(addr uint16, v byte) {
	if addr >= 0x800 {
		m.chrRAM[addr-0x800] = v
	}
}

// Save writes the board's mapper-specific state into s.
func (m *IremLROG017) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *IremLROG017) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// JalecoJF13 (mapper 86): 32 KiB PRG and 8 KiB CHR banks from a $6000
// register; the $7000 sample-playback register is ignored.
type JalecoJF13 struct {
	base

	prgBank byte
	chrBank byte
}

// NewJalecoJF13 wires the board.
func NewJalecoJF13(c *cartridge.Cartridge) *JalecoJF13 {
	return &JalecoJF13{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *JalecoJF13) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *JalecoJF13) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x7000 {
		m.prgBank = (v & 0x30) >> 4
		m.chrBank = (v & 0x03) | ((v >> 4) & 0x04)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *JalecoJF13) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *JalecoJF13) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *JalecoJF13) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
}

// Restore loads the board's mapper-specific state from s.
func (m *JalecoJF13) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
}

// Jaleco101 (mapper 101): an 8 KiB CHR bank register at $6000-$7FFF
// (the JF-10's mapper-87 register with its bits in the natural order).
type Jaleco101 struct {
	base

	chrBank byte
}

// NewJaleco101 wires the board.
func NewJaleco101(c *cartridge.Cartridge) *Jaleco101 {
	return &Jaleco101{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Jaleco101) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Jaleco101) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.chrBank = v
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Jaleco101) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBank), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Jaleco101) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBank), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Jaleco101) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.chrBank }

// Restore loads the board's mapper-specific state from s.
func (m *Jaleco101) Restore(s *State) { m.restoreRAM(s); m.chrBank = s.Regs[0] }

// Mapper107 (Magic Dragon): 32 KiB PRG and 8 KiB CHR from one register.
type Mapper107 struct {
	base

	reg byte
}

// NewMapper107 wires the board.
func NewMapper107(c *cartridge.Cartridge) *Mapper107 {
	return &Mapper107{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper107) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.reg)>>1, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper107) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.reg = v
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper107) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.reg), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper107) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.reg), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper107) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.reg }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper107) Restore(s *State) { m.restoreRAM(s); m.reg = s.Regs[0] }

// Mapper112 (NTDEC): MMC3-style index/data banking with the registers
// on $8000/$A000, an outer-CHR-bit register and mirroring at $E000.
type Mapper112 struct {
	base

	current  byte
	outerCHR byte
	regs     [8]byte
}

// NewMapper112 wires the board.
func NewMapper112(c *cartridge.Cartridge) *Mapper112 {
	m := &Mapper112{base: makeBase(c)}
	m.mirroring = cartridge.Vertical
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper112) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		bank := -2
		if addr >= 0xE000 {
			bank = -1
		}
		return window(m.prg, bank, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.regs[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper112) WritePRG(addr uint16, v byte) {
	if addr < 0x6000 {
		return
	}
	if addr < 0x8000 {
		m.writePRGRAM(addr, v)
		return
	}
	switch addr & 0xE001 {
	case 0x8000:
		m.current = v & 0x07
	case 0xA000:
		m.regs[m.current] = v
	case 0xC000:
		m.outerCHR = v
	case 0xE000:
		if v&0x01 != 0 {
			m.mirroring = cartridge.Horizontal
		} else {
			m.mirroring = cartridge.Vertical
		}
	}
}

func (m *Mapper112) chrBank(addr uint16) (int, int) {
	slot := addr >> 10
	if slot < 4 {
		// Two 2 KiB windows from regs 2-3 (bank in 1 KiB units, even).
		return int(m.regs[2+(slot>>1)]&^1) >> 1, 0x800
	}
	// The outer register holds CHR bit 8 for each 1 KiB slot (bits 4-7).
	high := 0
	if m.outerCHR&(0x10<<(slot-4)) != 0 {
		high = 0x100
	}
	return int(m.regs[slot]) | high, 0x400
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper112) ReadCHR(addr uint16) byte {
	bank, size := m.chrBank(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper112) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrBank(addr)
	m.chrWrite(bank, size, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper112) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.current
	s.Regs[1] = m.outerCHR
	copy(s.Regs[2:10], m.regs[:])
	s.Regs[10] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper112) Restore(s *State) {
	m.restoreRAM(s)
	m.current = s.Regs[0]
	m.outerCHR = s.Regs[1]
	copy(m.regs[:], s.Regs[2:10])
	m.mirroring = cartridge.Mirroring(s.Regs[10])
}

// CNROMProtect (mapper 185): a CNROM whose CHR enable works as copy
// protection; disabled CHR reads float with D0 pulled up.
type CNROMProtect struct {
	base

	sub        byte
	chrEnabled bool
}

// NewCNROMProtect wires the board.
func NewCNROMProtect(c *cartridge.Cartridge) *CNROMProtect {
	return &CNROMProtect{base: makeBase(c), sub: c.Submapper, chrEnabled: true}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *CNROMProtect) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *CNROMProtect) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	v &= m.ReadPRG(addr) // bus conflict
	switch m.sub {
	case 4, 5, 6, 7:
		m.chrEnabled = v&0x03 == m.sub-4
	default:
		m.chrEnabled = v&0x0F != 0 && v != 0x13
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *CNROMProtect) ReadCHR(addr uint16) byte {
	if !m.chrEnabled {
		// The floating bus reads back with D0 pulled up.
		return m.chrRead(0, 0x2000, addr) | 0x01
	}
	return m.chrRead(0, 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *CNROMProtect) WriteCHR(addr uint16, v byte) {
	if m.chrEnabled {
		m.chrWrite(0, 0x2000, addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *CNROMProtect) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = boolByte(m.chrEnabled)
}

// Restore loads the board's mapper-specific state from s.
func (m *CNROMProtect) Restore(s *State) {
	m.restoreRAM(s)
	m.chrEnabled = s.Regs[0] != 0
}

// UNROM512 (mapper 30, homebrew): up to 512 KiB of PRG in 16 KiB banks,
// 32 KiB of CHR RAM in 8 KiB banks and optional one-screen/mapper-
// controlled mirroring. The self-flashing (battery) variant's flash
// commands are not emulated; the board behaves as its non-battery
// revision.
type UNROM512 struct {
	base

	sub       byte
	mirrorBit bool
	prgBank   byte
	chrBank   byte
	chrRAM32  [4][8192]byte
}

// NewUNROM512 wires the board.
func NewUNROM512(c *cartridge.Cartridge) *UNROM512 {
	m := &UNROM512{base: makeBase(c), sub: c.Submapper}
	if c.Submapper == 3 {
		m.mirrorBit = true
		m.mirroring = cartridge.Vertical
	} else if c.Mirroring == cartridge.SingleLow || c.Mirroring == cartridge.SingleHigh {
		m.mirroring = cartridge.SingleLow
		m.mirrorBit = true
	}
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *UNROM512) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *UNROM512) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	if m.sub == 0 || m.sub == 2 {
		v &= m.ReadPRG(addr) // bus conflict on the non-battery revisions
	}
	m.prgBank = v & 0x1F
	m.chrBank = (v >> 5) & 0x03
	if m.mirrorBit {
		switch {
		case m.sub == 3 && v&0x80 != 0:
			m.mirroring = cartridge.Vertical
		case m.sub == 3:
			m.mirroring = cartridge.Horizontal
		case v&0x80 != 0:
			m.mirroring = cartridge.SingleHigh
		default:
			m.mirroring = cartridge.SingleLow
		}
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *UNROM512) ReadCHR(addr uint16) byte {
	return m.chrRAM32[m.chrBank][addr&0x1FFF]
}

// WriteCHR handles a write into the CHR address space.
func (m *UNROM512) WriteCHR(addr uint16, v byte) {
	m.chrRAM32[m.chrBank][addr&0x1FFF] = v
}

// Save writes the board's mapper-specific state into s.
func (m *UNROM512) Save(s *State) {
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
	s.Regs[2] = byte(m.mirroring)
	// Only the active CHR bank fits the snapshot; the others rarely
	// change mid-rewind-window, but copy the active one at least.
	s.CHRRAM = m.chrRAM32[m.chrBank]
}

// Restore loads the board's mapper-specific state from s.
func (m *UNROM512) Restore(s *State) {
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.mirroring = cartridge.Mirroring(s.Regs[2])
	m.chrRAM32[m.chrBank] = s.CHRRAM
}
