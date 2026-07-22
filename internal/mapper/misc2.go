package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Assorted small boards ported from the reference emulator, grouped by vendor.

// Subor166 (mappers 166, 167): four registers XOR together to form a PRG
// bank, with UNROM/NROM sub-modes. Mapper 167 swaps the two 16 KiB halves
// (altMode). CHR is fixed 8 KiB RAM.
type Subor166 struct {
	base
	regs    [4]byte
	altMode bool
	prg0    int
	prg1    int
}

// NewSubor166 wires the board.
func NewSubor166(c *cartridge.Cartridge) *Subor166 {
	m := &Subor166{base: makeBase(c), altMode: c.MapperID == 167}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Subor166) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0xE000 {
	case 0x8000:
		m.regs[0] = v & 0x10
	case 0xA000:
		m.regs[1] = v & 0x1C
	case 0xC000:
		m.regs[2] = v & 0x1F
	case 0xE000:
		m.regs[3] = v & 0x1F
	}
	m.update()
}

func (m *Subor166) update() {
	outer := int((m.regs[0]^m.regs[1])&0x10) << 1
	inner := int(m.regs[2] ^ m.regs[3])
	switch {
	case m.regs[1]&0x08 != 0: // 32 KiB NROM
		bank := (outer | inner) & 0xFE
		if m.altMode {
			m.prg0, m.prg1 = bank+1, bank
		} else {
			m.prg0, m.prg1 = bank, bank+1
		}
	case m.regs[1]&0x04 != 0: // inverted UNROM
		m.prg0, m.prg1 = 0x1F, outer|inner
	default: // UNROM
		m.prg0 = outer | inner
		if m.altMode {
			m.prg1 = 0x20
		} else {
			m.prg1 = 0x07
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Subor166) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, m.prg1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, m.prg0, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Subor166) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Subor166) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Subor166) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.regs[:])
	s.Regs[4] = byte(m.prg0)
	s.Regs[5] = byte(m.prg1)
}

// Restore loads the board's mapper-specific state from s.
func (m *Subor166) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.regs[:], s.Regs[0:4])
	m.prg0 = int(s.Regs[4])
	m.prg1 = int(s.Regs[5])
}

// Henggedianzi177 (mapper 177): a single 32 KiB PRG bank with H/V
// mirroring on bit 5, both from the written value.
type Henggedianzi177 struct {
	base
	prgBank byte
	mirror  cartridge.Mirroring
}

// NewHenggedianzi177 wires the board.
func NewHenggedianzi177(c *cartridge.Cartridge) *Henggedianzi177 {
	return &Henggedianzi177{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Henggedianzi177) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = v
		m.mirror = hvMirror(v&0x20 == 0)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Henggedianzi177) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Henggedianzi177) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Henggedianzi177) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Henggedianzi177) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Henggedianzi177) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Henggedianzi177) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.mirror = cartridge.Mirroring(s.Regs[1])
}

// Henggedianzi179 (mapper 179): a $5000-region write sets a 32 KiB PRG
// bank ((value>>1)); a $8000+ write toggles H/V mirroring.
type Henggedianzi179 struct {
	base
	prgBank byte
	mirror  cartridge.Mirroring
}

// NewHenggedianzi179 wires the board.
func NewHenggedianzi179(c *cartridge.Cartridge) *Henggedianzi179 {
	return &Henggedianzi179{base: makeBase(c), mirror: c.Mirroring}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Henggedianzi179) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.mirror = hvMirror(v&0x01 == 0)
	case addr >= 0x5000 && addr < 0x6000:
		m.prgBank = v >> 1
	case addr >= 0x6000 && addr < 0x8000:
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Henggedianzi179) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Henggedianzi179) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Henggedianzi179) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Henggedianzi179) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Henggedianzi179) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Henggedianzi179) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.mirror = cartridge.Mirroring(s.Regs[1])
}

// DaouInfosys (mapper 156): a Korean board with a switchable 16 KiB PRG
// window (last bank fixed), eight 1 KiB CHR windows loaded as low/high
// byte pairs (16-bit banks), and register-controlled mirroring at $C014.
type DaouInfosys struct {
	base
	prg0    byte
	chrLow  [8]byte
	chrHigh [8]byte
	mirror  cartridge.Mirroring
}

// NewDaouInfosys wires the board.
func NewDaouInfosys(c *cartridge.Cartridge) *DaouInfosys {
	return &DaouInfosys{base: makeBase(c), mirror: cartridge.SingleLow}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *DaouInfosys) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0xC000 && addr <= 0xC00F:
		bank := byte(addr&0x03) + bIf8(addr >= 0xC008, 4, 0)
		if addr&0x04 != 0 {
			m.chrHigh[bank] = v
		} else {
			m.chrLow[bank] = v
		}
	case addr == 0xC010:
		m.prg0 = v
	case addr == 0xC014:
		m.mirror = hvMirror(v&0x01 == 0)
	case addr >= 0x6000 && addr < 0x8000:
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *DaouInfosys) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prg0), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

func (m *DaouInfosys) chrBank(addr uint16) int {
	i := addr >> 10 & 7
	return int(m.chrHigh[i])<<8 | int(m.chrLow[i])
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *DaouInfosys) ReadCHR(addr uint16) byte {
	return m.chrRead(m.chrBank(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *DaouInfosys) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrBank(addr), 0x400, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *DaouInfosys) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *DaouInfosys) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prg0
	copy(s.Regs[1:9], m.chrLow[:])
	copy(s.Regs[9:17], m.chrHigh[:])
	s.Regs[17] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *DaouInfosys) Restore(s *State) {
	m.restoreRAM(s)
	m.prg0 = s.Regs[0]
	copy(m.chrLow[:], s.Regs[1:9])
	copy(m.chrHigh[:], s.Regs[9:17])
	m.mirror = cartridge.Mirroring(s.Regs[17])
}

// bIf8 is bIf for byte results.
func bIf8(cond bool, a, b byte) byte {
	if cond {
		return a
	}
	return b
}
