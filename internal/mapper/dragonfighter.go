package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// DragonFighter (mapper 292): the UNL-DRAGONFIGHTER pirate board, an
// MMC3 with a protection PLD in front of it. CPU reads of $6000-$6FFF
// make the PLD latch a value from console work RAM (zero page $6A or
// $FF, depending on the mode written to $6000), and the latched values
// scramble the CHR banking. Reads of the register area return 0.
type DragonFighter struct {
	MMC3

	exRegs [3]byte

	// peek reads the CPU bus without side effects; the console wires it.
	peek func(addr uint16) byte
}

// NewDragonFighter wires the board.
func NewDragonFighter(c *cartridge.Cartridge) *DragonFighter {
	return &DragonFighter{
		MMC3: *NewMMC3(c),
		peek: func(uint16) byte { return 0 },
	}
}

// SetCPUPeek installs the side-effect-free CPU bus read (console wiring).
func (m *DragonFighter) SetCPUPeek(peek func(addr uint16) byte) { m.peek = peek }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *DragonFighter) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		if addr < 0xA000 {
			// $8000 window: forced to the protection PRG register.
			return m.win(m.prg, int(m.exRegs[0]&0x1F), 0x2000)[addr&0x1FFF]
		}
		return m.MMC3.ReadPRG(addr)
	case addr >= 0x6000 && addr < 0x7000:
		// A read of an even address latches a byte of console RAM into
		// the protection registers and rebanks CHR.
		if addr&0x01 == 0 {
			if m.exRegs[0]&0xE0 == 0xC0 {
				m.exRegs[1] = m.peek(0x6A)
			} else {
				m.exRegs[2] = m.peek(0xFF)
			}
		}
		return 0
	}
	return m.MMC3.ReadPRG(addr)
}

// WritePRG handles a CPU write into the PRG address space.
func (m *DragonFighter) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x7000 {
		if addr&0x01 == 0 {
			m.exRegs[0] = v
		}
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// chrOffset resolves the scrambled CHR map: the low 2 KiB windows mix
// the MMC3 registers with the protection latches, and the whole upper
// 4 KiB comes straight from the second latch.
func (m *DragonFighter) chrOffset(addr uint16) int {
	switch {
	case addr < 0x0800:
		bank := int(m.banks[0]>>1) ^ int(m.exRegs[1])
		return bank<<11 | int(addr&0x7FF)
	case addr < 0x1000:
		bank := int(m.banks[1]>>1) | int(m.exRegs[2]&0x40)<<1
		return bank<<11 | int(addr&0x7FF)
	default:
		return int(m.exRegs[2]&0x3F)<<12 | int(addr&0xFFF)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *DragonFighter) ReadCHR(addr uint16) byte {
	return m.chr[m.chrOffset(addr)%len(m.chr)]
}

// WriteCHR handles a write into the CHR address space (CHR is ROM).
func (m *DragonFighter) WriteCHR(uint16, byte) {}

// Save writes the board's mapper-specific state into s.
func (m *DragonFighter) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:20], m.exRegs[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *DragonFighter) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.exRegs[:], s.Regs[17:20])
}
