package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// More multicart/discrete boards ported from the reference emulator. Boards whose only
// mode change happens on the console RESET line (226/230/231/233) keep
// their register behaviour here; the repo has no reset hook, so the
// reset-cycled sub-mode stays at its power-on value.

// ColorDreams144 (Color Dreams mapper 144): the low nibble of the written
// value is a 32 KiB PRG bank, the high nibble an 8 KiB CHR bank. Mapper 144
// forces data bit 0 high (a bus-conflict quirk). Mapper 11 is already
// handled by NewColorDreams; this covers 144.
type ColorDreams144 struct {
	base
	prgBank int
	chrBank int
}

// NewColorDreams144 wires the board.
func NewColorDreams144(c *cartridge.Cartridge) *ColorDreams144 {
	return &ColorDreams144{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *ColorDreams144) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		// 144 ORs in ROM bit 0 (bus conflict): the ROM byte at addr wins.
		v |= window(m.prg, m.prgBank, 0x8000)[addr&0x7FFF] & 0x01
		m.prgBank = int(v & 0x0F)
		m.chrBank = int(v>>4) & 0x0F
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *ColorDreams144) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *ColorDreams144) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *ColorDreams144) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *ColorDreams144) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
}

// Restore loads the board's mapper-specific state from s.
func (m *ColorDreams144) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
}

// Mapper225 (225, 52-in-1 etc.): the write address carries a PRG bank
// (16/32 KiB), an 8 KiB CHR bank and H/V mirroring, with a high bit that
// picks the 512 KiB half.
type Mapper225 struct{ dualPRG16 }

// NewMapper225 wires the board.
func NewMapper225(c *cartridge.Cartridge) *Mapper225 {
	m := &Mapper225{dualPRG16{base: makeBase(c)}}
	m.prg0, m.prg1 = 0, 1
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper225) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	high := int(addr>>8) & 0x40
	prg := int(addr>>6)&0x3F | high
	if addr&0x1000 != 0 {
		m.prg0, m.prg1 = prg, prg
	} else {
		m.prg0, m.prg1 = prg&0xFE, prg&0xFE|1
	}
	m.chrBank = int(addr&0x3F) | high
	m.mirror = hvMirror(addr&0x2000 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper225) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper225) Restore(s *State) { m.restoreDual(s) }

// Mapper226 (226): two registers select a wide PRG bank (16/32 KiB) plus
// mirroring. Also the base for 233.
type Mapper226 struct {
	dualPRG16
	reg [2]byte
}

// NewMapper226 wires the board.
func NewMapper226(c *cartridge.Cartridge) *Mapper226 {
	m := &Mapper226{dualPRG16: dualPRG16{base: makeBase(c)}}
	m.prg0, m.prg1 = 0, 1
	return m
}

func (m *Mapper226) prgPage() int {
	return int(m.reg[0]&0x1F) | int(m.reg[0]&0x80)>>2 | int(m.reg[1]&0x01)<<6
}

func (m *Mapper226) update() {
	p := m.prgPage()
	if m.reg[0]&0x20 != 0 {
		m.prg0, m.prg1 = p, p
	} else {
		m.prg0, m.prg1 = p&0xFE, p&0xFE|1
	}
	if m.reg[0]&0x40 != 0 {
		m.mirror = cartridge.Vertical
	} else {
		m.mirror = cartridge.Horizontal
	}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper226) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0x8001 {
	case 0x8000:
		m.reg[0] = v
	case 0x8001:
		m.reg[1] = v
	}
	m.update()
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper226) Save(s *State) {
	m.saveDual(s)
	s.Regs[7] = m.reg[0]
	s.Regs[8] = m.reg[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper226) Restore(s *State) {
	m.restoreDual(s)
	m.reg[0] = s.Regs[7]
	m.reg[1] = s.Regs[8]
}

// Mapper227 (227): the write address encodes a PRG bank plus several mode
// flags controlling the fixed window and 16/32 KiB split.
type Mapper227 struct{ dualPRG16 }

// NewMapper227 wires the board.
func NewMapper227(c *cartridge.Cartridge) *Mapper227 {
	m := &Mapper227{dualPRG16{base: makeBase(c)}}
	m.decode(0x8000)
	return m
}

func (m *Mapper227) decode(addr uint16) {
	prg := int(addr>>2)&0x1F | int(addr&0x100)>>3
	sFlag := addr&0x01 != 0
	lFlag := (addr>>9)&0x01 != 0
	prgMode := (addr>>7)&0x01 != 0

	switch {
	case prgMode:
		if sFlag {
			m.prg0, m.prg1 = prg&0xFE, prg&0xFE|1
		} else {
			m.prg0, m.prg1 = prg, prg
		}
	case sFlag:
		if lFlag {
			m.prg0, m.prg1 = prg&0x3E, prg|0x07
		} else {
			m.prg0, m.prg1 = prg&0x3E, prg&0x38
		}
	default:
		if lFlag {
			m.prg0, m.prg1 = prg, prg|0x07
		} else {
			m.prg0, m.prg1 = prg, prg&0x38
		}
	}
	m.mirror = hvMirror(addr&0x02 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper227) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper227) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper227) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper227) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper227) Restore(s *State) { m.restoreDual(s) }

// Mapper229 (229, 31-in-1): the low address bits select PRG/CHR banks and
// mirroring.
type Mapper229 struct{ dualPRG16 }

// NewMapper229 wires the board.
func NewMapper229(c *cartridge.Cartridge) *Mapper229 {
	m := &Mapper229{dualPRG16{base: makeBase(c)}}
	m.decode(0x8000)
	return m
}

func (m *Mapper229) decode(addr uint16) {
	m.chrBank = int(addr & 0xFF)
	if addr&0x1E == 0 {
		m.prg0, m.prg1 = 0, 1
	} else {
		p := int(addr & 0x1F)
		m.prg0, m.prg1 = p, p
	}
	m.mirror = hvMirror(addr&0x20 == 0)
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper229) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.decode(addr)
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper229) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper229) Restore(s *State) { m.restoreDual(s) }

// Mapper241 (241): the whole value is a 32 KiB PRG bank; CHR is fixed.
type Mapper241 struct {
	base
	prgBank byte
}

// NewMapper241 wires the board.
func NewMapper241(c *cartridge.Cartridge) *Mapper241 {
	return &Mapper241{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper241) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = v
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper241) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper241) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper241) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper241) Save(s *State) { m.saveRAM(s); s.Regs[0] = m.prgBank }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper241) Restore(s *State) { m.restoreRAM(s); m.prgBank = s.Regs[0] }

// Mapper231 (231, 20-in-1): the write address selects a 16/32 KiB PRG
// window and mirroring; CHR is fixed 8 KiB RAM.
type Mapper231 struct{ dualPRG16 }

// NewMapper231 wires the board.
func NewMapper231(c *cartridge.Cartridge) *Mapper231 {
	m := &Mapper231{dualPRG16{base: makeBase(c)}}
	m.prg0, m.prg1 = 0, 0
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper231) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	prg := int(addr>>5)&0x01 | int(addr&0x1E)
	m.prg0, m.prg1 = prg&0x1E, prg
	m.mirror = hvMirror(addr&0x80 == 0)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper231) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper231) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper231) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper231) Restore(s *State) { m.restoreDual(s) }

// Mapper234 (234, Maxi 15): registers at $FF80-$FFF8 read or written by
// the running code select PRG/CHR in a NINA-03 or CNROM sub-mode. The
// first register locks after its first non-zero write.
type Mapper234 struct {
	base
	reg [2]byte
}

// NewMapper234 wires the board.
func NewMapper234(c *cartridge.Cartridge) *Mapper234 {
	return &Mapper234{base: makeBase(c)}
}

func (m *Mapper234) access(addr uint16, v byte) {
	if addr <= 0xFF9F {
		if m.reg[0]&0x3F == 0 {
			m.reg[0] = v
		}
	} else {
		m.reg[1] = v & 0x71
	}
}

func (m *Mapper234) prgBank() int {
	if m.reg[0]&0x40 != 0 { // NINA-03 mode
		return int(m.reg[0]&0x0E) | int(m.reg[1]&0x01)
	}
	return int(m.reg[0] & 0x0F) // CNROM mode
}

func (m *Mapper234) chrBank() int {
	if m.reg[0]&0x40 != 0 {
		return int(m.reg[0]<<2)&0x38 | int(m.reg[1]>>4)&0x07
	}
	return int(m.reg[0]<<2)&0x3C | int(m.reg[1]>>4)&0x03
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper234) WritePRG(addr uint16, v byte) {
	if addr >= 0xFF80 && addr <= 0xFFF8 {
		m.access(addr, v)
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper234) ReadPRG(addr uint16) byte {
	var v byte
	switch {
	case addr >= 0x8000:
		v = window(m.prg, m.prgBank(), 0x8000)[addr&0x7FFF]
	case addr >= 0x6000:
		v = m.readPRGRAM(addr)
	default:
		return m.openBus()
	}
	// Reads in the register window latch just like writes (Maxi 15 polls
	// the registers by reading them).
	if addr >= 0xFF80 && addr <= 0xFFF8 {
		m.access(addr, v)
	}
	return v
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper234) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank(), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper234) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank(), 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper234) Mirroring() cartridge.Mirroring {
	return hvMirror(m.reg[0]&0x80 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper234) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.reg[0]
	s.Regs[1] = m.reg[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper234) Restore(s *State) {
	m.restoreRAM(s)
	m.reg[0] = s.Regs[0]
	m.reg[1] = s.Regs[1]
}

var lutPrg244 = [4][4]byte{
	{0, 1, 2, 3}, {3, 2, 1, 0}, {0, 2, 1, 3}, {3, 1, 2, 0},
}
var lutChr244 = [8][8]byte{
	{0, 1, 2, 3, 4, 5, 6, 7},
	{0, 2, 1, 3, 4, 6, 5, 7},
	{0, 1, 4, 5, 2, 3, 6, 7},
	{0, 4, 1, 5, 2, 6, 3, 7},
	{0, 4, 2, 6, 1, 5, 3, 7},
	{0, 2, 4, 6, 1, 3, 5, 7},
	{7, 6, 5, 4, 3, 2, 1, 0},
	{7, 6, 5, 4, 3, 2, 1, 0},
}

// Mapper244 (244): a value write selects either a scrambled 32 KiB PRG
// bank or a scrambled 8 KiB CHR bank, via two lookup tables.
type Mapper244 struct {
	base
	prgBank int
	chrBank int
}

// NewMapper244 wires the board.
func NewMapper244(c *cartridge.Cartridge) *Mapper244 {
	return &Mapper244{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper244) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		if v&0x08 != 0 {
			m.chrBank = int(lutChr244[(v>>4)&0x07][v&0x07])
		} else {
			m.prgBank = int(lutPrg244[(v>>4)&0x03][v&0x03])
		}
		return
	}
	if addr >= 0x6000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper244) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper244) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Mapper244) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Mapper244) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper244) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
}
