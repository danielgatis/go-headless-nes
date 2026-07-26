package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Small Sachen boards ported from the reference emulator.

// Sachen133 (mapper 133): a $4100-region register with a 32 KiB PRG bit
// and a 2-bit CHR bank.
type Sachen133 struct {
	base
	prgBank int
	chrBank int
}

// NewSachen133 wires the board.
func NewSachen133(c *cartridge.Cartridge) *Sachen133 {
	return &Sachen133{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen133) WritePRG(addr uint16, v byte) {
	if addr&0x6100 == 0x4100 {
		m.prgBank = int(v>>2) & 0x01
		m.chrBank = int(v & 0x03)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen133) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen133) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Sachen133) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Sachen133) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sachen133) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
}

// Sachen143 (mapper 143): a protection board that returns the inverted
// low address bits from its $4100-$5FFF register; banking is fixed.
type Sachen143 struct{ base }

// NewSachen143 wires the board.
func NewSachen143(c *cartridge.Cartridge) *Sachen143 {
	return &Sachen143{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen143) WritePRG(_ uint16, _ byte) {}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen143) ReadPRG(addr uint16) byte {
	if addr >= 0x4100 && addr < 0x6000 {
		return byte(^addr&0x3F) | 0x40
	}
	if addr >= 0x8000 {
		// Two fixed 16 KiB PRG banks (0 at $8000, 1 at $C000).
		return m.win(m.prg, int((addr-0x8000)>>14), 0x4000)[addr&0x3FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen143) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Sachen143) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Sachen143) Save(s *State) { m.saveRAM(s) }

// Restore loads the board's mapper-specific state from s.
func (m *Sachen143) Restore(s *State) { m.restoreRAM(s) }

// Sachen145 (mapper 145): a $4100-region register selects an 8 KiB CHR
// bank from bit 7; PRG is fixed 32 KiB.
type Sachen145 struct {
	base
	chrBank int
}

// NewSachen145 wires the board.
func NewSachen145(c *cartridge.Cartridge) *Sachen145 {
	return &Sachen145{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen145) WritePRG(addr uint16, v byte) {
	if addr&0x4100 == 0x4100 {
		m.chrBank = int(v>>7) & 0x01
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen145) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen145) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Sachen145) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Sachen145) Save(s *State) { m.saveRAM(s); s.Regs[0] = byte(m.chrBank) }

// Restore loads the board's mapper-specific state from s.
func (m *Sachen145) Restore(s *State) { m.restoreRAM(s); m.chrBank = int(s.Regs[0]) }

// Sachen148 (mapper 148): a value write selects a 32 KiB PRG bank (bit 3)
// and a 3-bit CHR bank.
type Sachen148 struct {
	base
	prgBank int
	chrBank int
}

// NewSachen148 wires the board.
func NewSachen148(c *cartridge.Cartridge) *Sachen148 {
	return &Sachen148{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen148) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.prgBank = int(v>>3) & 0x01
		m.chrBank = int(v & 0x07)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen148) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen148) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Sachen148) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Sachen148) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgBank)
	s.Regs[1] = byte(m.chrBank)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sachen148) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = int(s.Regs[0])
	m.chrBank = int(s.Regs[1])
}

// Sachen149 (mapper 149): a value write selects an 8 KiB CHR bank from
// bit 7; PRG is fixed 32 KiB.
type Sachen149 struct {
	base
	chrBank int
}

// NewSachen149 wires the board.
func NewSachen149(c *cartridge.Cartridge) *Sachen149 {
	return &Sachen149{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen149) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.chrBank = int(v>>7) & 0x01
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen149) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen149) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Sachen149) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Sachen149) Save(s *State) { m.saveRAM(s); s.Regs[0] = byte(m.chrBank) }

// Restore loads the board's mapper-specific state from s.
func (m *Sachen149) Restore(s *State) { m.restoreRAM(s); m.chrBank = int(s.Regs[0]) }

// Sachen136 (mapper 136): the JV001 TXC chip drives a 6-bit CHR bank.
type Sachen136 struct {
	base
	txc txcChip
}

// NewSachen136 wires the board.
func NewSachen136(c *cartridge.Cartridge) *Sachen136 {
	return &Sachen136{base: makeBase(c), txc: newTxcChip(true)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen136) WritePRG(addr uint16, v byte) { m.txc.write(addr, v&0x3F) }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen136) ReadPRG(addr uint16) byte {
	if addr >= 0x4020 && addr < 0x6000 {
		out := m.openBus()
		if addr&0x103 == 0x100 {
			out = (out & 0xC0) | (m.txc.read() & 0x3F)
		}
		return out
	}
	if addr >= 0x8000 {
		return m.win(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen136) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.txc.output), 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sachen136) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.txc.output), 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Sachen136) Save(s *State) { m.saveRAM(s); m.txc.save(s.Regs[0:7]) }

// Restore loads the board's mapper-specific state from s.
func (m *Sachen136) Restore(s *State) { m.restoreRAM(s); m.txc.restore(s.Regs[0:7]) }

// Sachen147 (mapper 147): the JV001 TXC chip (with a scrambled data bus)
// drives a 32 KiB PRG bit and a 4-bit CHR bank.
type Sachen147 struct {
	base
	txc txcChip
}

// NewSachen147 wires the board.
func NewSachen147(c *cartridge.Cartridge) *Sachen147 {
	return &Sachen147{base: makeBase(c), txc: newTxcChip(true)}
}

func txc147Scramble(v byte) byte { return (v&0xFC)>>2 | (v&0x03)<<6 }

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen147) WritePRG(addr uint16, v byte) {
	m.txc.write(addr, txc147Scramble(v))
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen147) ReadPRG(addr uint16) byte {
	if addr >= 0x4020 && addr < 0x6000 {
		out := m.openBus()
		if addr&0x103 == 0x100 {
			r := m.txc.read()
			out = (r&0x3F)<<2 | (r&0xC0)>>6
		}
		return out
	}
	if addr >= 0x8000 {
		out := m.txc.output
		bank := int(out&0x20)>>4 | int(out&0x01)
		return m.win(m.prg, bank, 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen147) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.txc.output&0x1E)>>1, 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Sachen147) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.txc.output&0x1E)>>1, 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Sachen147) Save(s *State) { m.saveRAM(s); m.txc.save(s.Regs[0:7]) }

// Restore loads the board's mapper-specific state from s.
func (m *Sachen147) Restore(s *State) { m.restoreRAM(s); m.txc.restore(s.Regs[0:7]) }

// Sachen74LS374N (mappers 150, 243): an indexed 8-register board. A
// register select at $4100 and data at $4101 drive a CHR bank, a 32 KiB
// PRG bank and mirroring. Mapper 150 wires the CHR bits differently from
// 243. The DIP-switch OR-$04 quirk is not modelled (DIP defaults to 0).
type Sachen74LS374N struct {
	base
	is150      bool
	currentReg byte
	regs       [8]byte
	mirror     cartridge.Mirroring
}

// NewSachen74LS374N wires the board.
func NewSachen74LS374N(c *cartridge.Cartridge) *Sachen74LS374N {
	m := &Sachen74LS374N{base: makeBase(c), is150: c.MapperID == 150, mirror: c.Mirroring}
	m.update()
	return m
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Sachen74LS374N) WritePRG(addr uint16, v byte) {
	if addr >= 0x4100 && addr < 0x8000 {
		switch addr & 0xC101 {
		case 0x4100:
			m.currentReg = v & 0x07
		case 0x4101:
			m.regs[m.currentReg] = v & 0x07
			m.update()
		}
	}
}

func (m *Sachen74LS374N) update() {
	switch (m.regs[7] >> 1) & 0x03 {
	case 0:
		// Reference: SetNametables(0,0,0,1); approximate with single-low.
		m.mirror = cartridge.SingleLow
	case 1:
		m.mirror = cartridge.Horizontal
	case 2:
		m.mirror = cartridge.Vertical
	case 3:
		m.mirror = cartridge.SingleLow
	}
}

func (m *Sachen74LS374N) chrBank() int {
	if m.is150 {
		return int(m.regs[4]&0x01)<<2 | int(m.regs[6]&0x03)
	}
	return int(m.regs[2]&0x01) | int(m.regs[4]&0x01)<<1 | int(m.regs[6]&0x03)<<2
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Sachen74LS374N) ReadPRG(addr uint16) byte {
	if addr >= 0x4100 && addr < 0x8000 && addr&0xC101 == 0x4101 {
		return m.openBus()&0xF8 | m.regs[m.currentReg]&0x07
	}
	if addr >= 0x8000 {
		return m.win(m.prg, int(m.regs[5]&0x03), 0x8000)[addr&0x7FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Sachen74LS374N) ReadCHR(addr uint16) byte { return m.chrRead(m.chrBank(), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Sachen74LS374N) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrBank(), 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Sachen74LS374N) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Sachen74LS374N) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.currentReg
	copy(s.Regs[1:9], m.regs[:])
	s.Regs[9] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Sachen74LS374N) Restore(s *State) {
	m.restoreRAM(s)
	m.currentReg = s.Regs[0]
	copy(m.regs[:], s.Regs[1:9])
	m.mirror = cartridge.Mirroring(s.Regs[9])
}
