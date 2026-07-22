package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Simple discrete/multicart boards ported from the reference emulator. These latch their
// bank state from the write *address* (the data byte is usually ignored),
// switch one or two PRG windows and a single CHR window, and toggle H/V
// mirroring. Each stores its own mirroring since the base has no setter.

// prg16 reads a 16 KiB-windowed PRG board's two banks.
type dualPRG16 struct {
	base
	prg0, prg1 int
	chrBank    int
	mirror     cartridge.Mirroring
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *dualPRG16) ReadPRG(addr uint16) byte {
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
func (m *dualPRG16) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *dualPRG16) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *dualPRG16) Mirroring() cartridge.Mirroring { return m.mirror }

func (m *dualPRG16) saveDual(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prg0)
	s.Regs[1] = byte(m.prg1)
	s.Regs[2] = byte(m.chrBank)
	s.Regs[3] = byte(m.mirror)
	s.Regs[4] = byte(m.prg0 >> 8)
	s.Regs[5] = byte(m.prg1 >> 8)
	s.Regs[6] = byte(m.chrBank >> 8)
}

func (m *dualPRG16) restoreDual(s *State) {
	m.restoreRAM(s)
	m.prg0 = int(s.Regs[0]) | int(s.Regs[4])<<8
	m.prg1 = int(s.Regs[1]) | int(s.Regs[5])<<8
	m.chrBank = int(s.Regs[2]) | int(s.Regs[6])<<8
	m.mirror = cartridge.Mirroring(s.Regs[3])
}

func hvMirror(vertical bool) cartridge.Mirroring {
	if vertical {
		return cartridge.Vertical
	}
	return cartridge.Horizontal
}

// Mapper57 (57): two address-latched registers at $8000/$8800 select a
// CHR bank (with a high bit) and a PRG bank in 16/32 KiB modes.
type Mapper57 struct {
	dualPRG16
	reg [2]byte
}

// NewMapper57 wires the board.
func NewMapper57(c *cartridge.Cartridge) *Mapper57 {
	m := &Mapper57{dualPRG16: dualPRG16{base: makeBase(c)}}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper57) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	switch addr & 0x8800 {
	case 0x8000:
		m.reg[0] = v
	case 0x8800:
		m.reg[1] = v
	}
	m.update()
}

func (m *Mapper57) update() {
	m.mirror = hvMirror(m.reg[1]&0x08 == 0)
	m.chrBank = int((m.reg[0]&0x40)>>3) | int((m.reg[0]|m.reg[1])&0x07)
	if m.reg[1]&0x10 != 0 {
		base := int(m.reg[1]>>5) & 0x06
		m.prg0, m.prg1 = base, base+1
	} else {
		p := int(m.reg[1]>>5) & 0x07
		m.prg0, m.prg1 = p, p
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper57) Save(s *State) {
	m.saveDual(s)
	s.Regs[7] = m.reg[0]
	s.Regs[8] = m.reg[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper57) Restore(s *State) {
	m.restoreDual(s)
	m.reg[0] = s.Regs[7]
	m.reg[1] = s.Regs[8]
}

// Mapper58 (58): address bits select PRG (16/32 KiB) and CHR banks.
type Mapper58 struct{ dualPRG16 }

// NewMapper58 wires the board.
func NewMapper58(c *cartridge.Cartridge) *Mapper58 {
	m := &Mapper58{dualPRG16{base: makeBase(c)}}
	m.prg0, m.prg1 = 0, 1
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper58) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	prg := int(addr & 0x07)
	if addr&0x40 != 0 {
		m.prg0, m.prg1 = prg, prg
	} else {
		m.prg0, m.prg1 = prg&0x06, prg&0x06|1
	}
	m.chrBank = int(addr>>3) & 0x07
	m.mirror = hvMirror(addr&0x80 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper58) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper58) Restore(s *State) { m.restoreDual(s) }

// Mapper61 (61): address bits form a PRG page, in 16/32 KiB modes.
type Mapper61 struct{ dualPRG16 }

// NewMapper61 wires the board.
func NewMapper61(c *cartridge.Cartridge) *Mapper61 {
	m := &Mapper61{dualPRG16{base: makeBase(c)}}
	m.prg0, m.prg1 = 0, 1
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper61) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	prg := int((addr&0x0F)<<1) | int(addr>>5)&0x01
	if addr&0x10 != 0 {
		m.prg0, m.prg1 = prg, prg
	} else {
		m.prg0, m.prg1 = prg&0xFE, prg&0xFE|1
	}
	m.mirror = hvMirror(addr&0x80 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper61) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper61) Restore(s *State) { m.restoreDual(s) }

// Mapper62 (62): address bits form the PRG page and CHR high bits; the
// data byte supplies the CHR low bits.
type Mapper62 struct{ dualPRG16 }

// NewMapper62 wires the board.
func NewMapper62(c *cartridge.Cartridge) *Mapper62 {
	m := &Mapper62{dualPRG16{base: makeBase(c)}}
	m.prg0, m.prg1 = 0, 1
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper62) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	prg := int((addr&0x3F00)>>8) | int(addr&0x40)
	m.chrBank = int((addr&0x1F)<<2) | int(v&0x03)
	if addr&0x20 != 0 {
		m.prg0, m.prg1 = prg, prg
	} else {
		m.prg0, m.prg1 = prg&0xFE, prg&0xFE|1
	}
	m.mirror = hvMirror(addr&0x80 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper62) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper62) Restore(s *State) { m.restoreDual(s) }

// Mapper200 (200): low address bits mirror one bank into all windows.
type Mapper200 struct{ dualPRG16 }

// NewMapper200 wires the board.
func NewMapper200(c *cartridge.Cartridge) *Mapper200 {
	return &Mapper200{dualPRG16{base: makeBase(c)}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper200) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	bank := int(addr & 0x07)
	m.prg0, m.prg1, m.chrBank = bank, bank, bank
	m.mirror = hvMirror(addr&0x08 != 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper200) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper200) Restore(s *State) { m.restoreDual(s) }

// Mapper202 (202): address bits pick a bank; bit pattern $09 enables a
// two-bank PRG mode.
type Mapper202 struct{ dualPRG16 }

// NewMapper202 wires the board.
func NewMapper202(c *cartridge.Cartridge) *Mapper202 {
	return &Mapper202{dualPRG16{base: makeBase(c)}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper202) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	bank := int(addr>>1) & 0x07
	m.chrBank = bank
	if addr&0x09 == 0x09 {
		m.prg0, m.prg1 = bank, bank+1
	} else {
		m.prg0, m.prg1 = bank, bank
	}
	m.mirror = hvMirror(addr&0x01 == 0)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper202) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper202) Restore(s *State) { m.restoreDual(s) }

// Mapper203 (203): the data byte selects a 16 KiB PRG bank (mirrored) and
// an 8 KiB CHR bank.
type Mapper203 struct{ dualPRG16 }

// NewMapper203 wires the board.
func NewMapper203(c *cartridge.Cartridge) *Mapper203 {
	return &Mapper203{dualPRG16{base: makeBase(c), mirror: c.Mirroring}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper203) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	m.prg0, m.prg1 = int(v>>2), int(v>>2)
	m.chrBank = int(v & 0x03)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper203) Save(s *State) { m.saveDual(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Mapper203) Restore(s *State) { m.restoreDual(s) }

// NovelDiamond (54, 201): low address bits select PRG (32 KiB) and CHR.
type NovelDiamond struct {
	base
	prgBank int
	chrBank int
}

// NewNovelDiamond wires the board.
func NewNovelDiamond(c *cartridge.Cartridge) *NovelDiamond {
	return &NovelDiamond{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *NovelDiamond) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return window(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *NovelDiamond) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	m.prgBank = int(addr & 0x03)
	m.chrBank = int(addr & 0x07)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *NovelDiamond) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *NovelDiamond) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *NovelDiamond) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
}

// Restore loads the board's mapper-specific state from s.
func (m *NovelDiamond) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
}

// Mapper213 (213): a NovelDiamond-like board wired to different address
// bits (PRG in 32 KiB, CHR in 8 KiB).
type Mapper213 struct{ NovelDiamond }

// NewMapper213 wires the board.
func NewMapper213(c *cartridge.Cartridge) *Mapper213 {
	return &Mapper213{NovelDiamond{base: makeBase(c)}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper213) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	m.chrBank = int(addr>>3) & 0x07
	m.prgBank = int(addr>>1) & 0x03
}

// Mapper240 (240): a $4020-$5FFF register with PRG (32 KiB) in the high
// nibble and CHR (8 KiB) in the low nibble.
type Mapper240 struct{ NovelDiamond }

// NewMapper240 wires the board.
func NewMapper240(c *cartridge.Cartridge) *Mapper240 {
	return &Mapper240{NovelDiamond{base: makeBase(c)}}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper240) WritePRG(addr uint16, v byte) {
	if addr >= 0x4020 && addr < 0x6000 {
		m.prgBank = int(v>>4) & 0x0F
		m.chrBank = int(v & 0x0F)
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// Mapper242 (242): a $8000+ register with PRG (32 KiB) in bits 3-6 and
// H/V mirroring on bit 1.
type Mapper242 struct {
	NovelDiamond
	mirror cartridge.Mirroring
}

// NewMapper242 wires the board.
func NewMapper242(c *cartridge.Cartridge) *Mapper242 {
	return &Mapper242{NovelDiamond: NovelDiamond{base: makeBase(c)}, mirror: cartridge.Vertical}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper242) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 {
			m.writePRGRAM(addr, v)
		}
		return
	}
	m.mirror = hvMirror(addr&0x02 == 0)
	m.prgBank = int(addr>>3) & 0x0F
}

// Mirroring reports the board's current nametable mirroring.
func (m *Mapper242) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Mapper242) Save(s *State) {
	m.NovelDiamond.Save(s)
	s.Regs[2] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper242) Restore(s *State) {
	m.NovelDiamond.Restore(s)
	m.mirror = cartridge.Mirroring(s.Regs[2])
}
