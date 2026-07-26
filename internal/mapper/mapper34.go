package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Mapper34 covers two unrelated boards that share the number:
// BNROM (32 KiB PRG banking, CHR RAM, latch at $8000+) and NINA-001
// (registers in PRG RAM space at $7FFD-$7FFF, two 4 KiB CHR windows).
// A cartridge with CHR ROM must be NINA-001; BNROM boards carry CHR RAM.
type Mapper34 struct {
	base
	nina bool

	prgBank byte
	chr     [2]byte
}

// NewMapper34 picks the board variant from the NES 2.0 submapper when
// present (1 = NINA-001, 2 = BNROM), falling back to the cartridge
// contents: only NINA-001 boards carry CHR ROM.
func NewMapper34(c *cartridge.Cartridge) *Mapper34 {
	nina := len(c.CHR) > 0
	switch c.Submapper {
	case 1:
		nina = true
	case 2:
		nina = false
	}
	return &Mapper34{base: makeBase(c), nina: nina}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Mapper34) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prgBank), 0x8000)[addr&0x7FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Mapper34) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		if !m.nina {
			m.prgBank = v & m.ReadPRG(addr) & 3 // BNROM: bus conflict
		}
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
		if m.nina {
			switch addr {
			case 0x7FFD:
				m.prgBank = v & 1
			case 0x7FFE:
				m.chr[0] = v & 0x0F
			case 0x7FFF:
				m.chr[1] = v & 0x0F
			}
		}
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Mapper34) ReadCHR(addr uint16) byte {
	if m.nina {
		return m.chrRead(int(m.chr[addr>>12]), 4096, addr)
	}
	return m.chrRead(0, 8192, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Mapper34) WriteCHR(addr uint16, v byte) {
	if m.nina {
		m.chrWrite(int(m.chr[addr>>12]), 4096, addr, v)
		return
	}
	m.chrWrite(0, 8192, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Mapper34) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chr[0]
	s.Regs[2] = m.chr[1]
}

// Restore loads the board's mapper-specific state from s.
func (m *Mapper34) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.chr[0] = s.Regs[1]
	m.chr[1] = s.Regs[2]
}
