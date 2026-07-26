package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// BandaiKaraoke (mapper 188): Karaoke Studio. The cartridge has a slot
// for expansion song packs: the internal ROM is the first 512 KiB and an
// inserted pack appears as banks 8+. One switchable 16 KiB PRG window
// (register bit 4 picks internal vs expansion ROM), the second window
// fixed to the last internal bank. $6000-$7FFF reads the microphone; we
// model it unconnected (buttons idle, mic silent), leaving the top bits
// to open bus. The register writes see bus conflicts.
type BandaiKaraoke struct {
	base

	prgBank  byte
	expandOn bool // selected bank is in the (possibly absent) expansion pack
	mirror   cartridge.Mirroring
}

// NewBandaiKaraoke wires the board.
func NewBandaiKaraoke(c *cartridge.Cartridge) *BandaiKaraoke {
	return &BandaiKaraoke{base: makeBase(c), mirror: c.Mirroring}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *BandaiKaraoke) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		// Fixed to the last bank of the internal ROM (bank 7 of 512 KiB;
		// window wraps for smaller dumps).
		return m.win(m.prg, 7, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		bank := int(m.prgBank)
		if m.expandOn {
			if len(m.prg) < 0x40000+0x4000 {
				// No expansion pack inserted: the window floats.
				return m.openBus()
			}
			bank |= 0x08
		}
		return m.win(m.prg, bank, 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		// Microphone port: bit 0-1 buttons (active low on hardware reads
		// as 0 here), bit 2 mic level; upper bits float.
		return m.openBus() & 0xF8
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space.
func (m *BandaiKaraoke) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	v &= m.ReadPRG(addr) // bus conflict
	m.prgBank = v & 0x07
	m.expandOn = v&0x10 == 0
	if v&0x20 != 0 {
		m.mirror = cartridge.Horizontal
	} else {
		m.mirror = cartridge.Vertical
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *BandaiKaraoke) ReadCHR(addr uint16) byte { return m.chrRead(0, 8192, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *BandaiKaraoke) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 8192, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *BandaiKaraoke) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *BandaiKaraoke) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = boolByte(m.expandOn)
	s.Regs[2] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *BandaiKaraoke) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.expandOn = s.Regs[1] != 0
	m.mirror = cartridge.Mirroring(s.Regs[2])
}
