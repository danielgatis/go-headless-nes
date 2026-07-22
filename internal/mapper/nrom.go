package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// NROM (mapper 0) is the launch-era board with no bank switching:
// 16 or 32 KiB PRG ROM at $8000-$FFFF (16 KiB carts see it mirrored),
// 8 KiB CHR ROM or RAM, and optionally 8 KiB PRG RAM at $6000-$7FFF
// (used by Family BASIC).
type NROM struct {
	base
}

// NewNROM wires an NROM board to the given cartridge.
func NewNROM(c *cartridge.Cartridge) *NROM {
	return &NROM{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *NROM) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		// Address lines above the ROM size are not connected, so a
		// 16 KiB ROM appears twice in the 32 KiB window.
		return m.prg[int(addr-0x8000)%len(m.prg)]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	// Nothing decodes $4020-$5FFF on NROM: open bus.
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *NROM) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
	// Writes to ROM are silently ignored, as on hardware.
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *NROM) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *NROM) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *NROM) Save(s *State) { m.saveRAM(s) }

// Restore loads the board's mapper-specific state from s.
func (m *NROM) Restore(s *State) { m.restoreRAM(s) }
