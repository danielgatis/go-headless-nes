package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// GTROM (mapper 111, "Cheapocabra"): Membler Industries' homebrew flash
// board. One 32 KiB PRG window into a 512 KiB SST39SF040 flash chip the
// game can reprogram in-system (self-saving), 16 KiB of CHR RAM in two
// 8 KiB pages, and 16 KiB of nametable RAM in two 8 KiB pages served by
// the board instead of CIRAM. A single register (write $5000-$5FFF or
// $7000-$7FFF; reading those ranges writes the open-bus value) selects
// all three: PRG bits 0-3, CHR bit 4, nametable bit 5.
//
// Flash programming mutates the board's PRG copy. Like the UNROM512
// port, the 512 KiB array exceeds the fixed-size snapshot, so rewind
// does not undo flash writes (they are saves; keeping them is the
// sensible behavior).
type GTROM struct {
	base

	reg     byte
	prgPage byte
	chrPage byte
	ntPage  byte

	flash  flashSST39SF040
	prgMut []byte // mutable copy of PRG ROM (the flash contents)

	chrRAM16 [16384]byte
	ntRAM    [16384]byte
}

// NewGTROM wires the board.
func NewGTROM(c *cartridge.Cartridge) *GTROM {
	m := &GTROM{base: makeBase(c)}
	m.prgMut = make([]byte, len(c.PRG))
	copy(m.prgMut, c.PRG)
	m.prg = m.prgMut
	return m
}

// updateRegister decodes the single latch.
func (m *GTROM) updateRegister(v byte) {
	m.reg = v
	m.prgPage = v & 0x0F
	m.chrPage = v >> 4 & 1
	m.ntPage = v >> 5 & 1
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *GTROM) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		if v, ok := m.flash.read(addr); ok {
			return v
		}
		return m.win(m.prg, int(m.prgPage), 0x8000)[addr&0x7FFF]
	case addr >= 0x7000, addr >= 0x5000 && addr < 0x6000:
		// Reading the register range latches the floating bus value.
		m.updateRegister(m.openBus())
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space.
func (m *GTROM) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		m.flash.write(m.prgMut, uint32(m.prgPage)<<15|uint32(addr&0x7FFF), v)
	case addr >= 0x7000, addr >= 0x5000 && addr < 0x6000:
		m.updateRegister(v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *GTROM) ReadCHR(addr uint16) byte {
	return m.chrRAM16[int(m.chrPage)<<13|int(addr&0x1FFF)]
}

// WriteCHR handles a write into the CHR address space.
func (m *GTROM) WriteCHR(addr uint16, v byte) {
	m.chrRAM16[int(m.chrPage)<<13|int(addr&0x1FFF)] = v
}

// ntOffset maps a nametable address into the board's NT RAM.
func (m *GTROM) ntOffset(addr uint16) int {
	return int(m.ntPage)<<13 | int(addr&0x0FFF)
}

// ReadNT serves every nametable fetch from the board's own RAM.
func (m *GTROM) ReadNT(addr uint16) (byte, bool) {
	return m.ntRAM[m.ntOffset(addr)], true
}

// WriteNT handles a nametable write the board intercepts.
func (m *GTROM) WriteNT(addr uint16, v byte) bool {
	m.ntRAM[m.ntOffset(addr)] = v
	return true
}

// Save writes the board's mapper-specific state into s. The board has
// no PRG RAM, so its 16 KiB CHR RAM and 16 KiB NT RAM ride in the
// snapshot's PRG-RAM area and round-trip fully.
func (m *GTROM) Save(s *State) {
	copy(s.PRGRAM[:16384], m.chrRAM16[:])
	copy(s.PRGRAM[16384:], m.ntRAM[:])
	s.Regs[0] = m.reg
	s.Regs[1] = byte(m.flash.mode)
	s.Regs[2] = m.flash.cycle
	s.Regs[3] = boolByte(m.flash.softwareID)
}

// Restore loads the board's mapper-specific state from s.
func (m *GTROM) Restore(s *State) {
	copy(m.chrRAM16[:], s.PRGRAM[:16384])
	copy(m.ntRAM[:], s.PRGRAM[16384:])
	m.updateRegister(s.Regs[0])
	m.flash.mode = flashMode(s.Regs[1])
	m.flash.cycle = s.Regs[2]
	m.flash.softwareID = s.Regs[3] != 0
}

// --- SST39SF040 flash chip ---

// flashMode is the SST39SF040 command state.
type flashMode byte

const (
	flashIdle flashMode = iota
	flashWrite
	flashErase
)

// flashSST39SF040 models the JEDEC command interface of the 512 KiB
// flash chip GTROM boards carry: $5555=$AA, $2AAA=$55 unlock sequences
// followed by byte-program, sector/chip erase or software-ID commands.
type flashSST39SF040 struct {
	mode       flashMode
	cycle      byte
	softwareID bool
}

// read serves a flash read in software-ID mode; ok is false when the
// chip is in normal array-read mode.
func (f *flashSST39SF040) read(addr uint16) (byte, bool) {
	if !f.softwareID {
		return 0, false
	}
	switch addr & 0x1FF {
	case 0x00:
		return 0xBF, true // manufacturer: SST
	case 0x01:
		return 0xB7, true // device: SST39SF040
	default:
		return 0xFF, true
	}
}

func (f *flashSST39SF040) reset() {
	f.mode = flashIdle
	f.cycle = 0
}

// write advances the command state machine; data is the full flash
// array the program/erase commands mutate.
func (f *flashSST39SF040) write(data []byte, addr uint32, v byte) {
	cmd := addr & 0x7FFF
	switch f.mode {
	case flashIdle:
		switch {
		case f.cycle == 0 && cmd == 0x5555 && v == 0xAA:
			f.cycle++
		case f.cycle == 0 && v == 0xF0:
			f.reset()
			f.softwareID = false
		case f.cycle == 1 && cmd == 0x2AAA && v == 0x55:
			f.cycle++
		case f.cycle == 2 && cmd == 0x5555:
			f.cycle++
			switch v {
			case 0x80:
				f.mode = flashErase
			case 0x90:
				f.reset()
				f.softwareID = true
			case 0xA0:
				f.mode = flashWrite
			case 0xF0:
				f.reset()
				f.softwareID = false
			}
		default:
			f.cycle = 0
		}
	case flashWrite:
		// Byte program: flash can only clear bits.
		if int(addr) < len(data) {
			data[addr] &= v
		}
		f.reset()
	case flashErase:
		switch {
		case f.cycle == 3 && cmd == 0x5555 && v == 0xAA:
			f.cycle++
		case f.cycle == 4 && cmd == 0x2AAA && v == 0x55:
			f.cycle++
		case f.cycle == 5:
			if cmd == 0x5555 && v == 0x10 {
				// Chip erase.
				for i := range data {
					data[i] = 0xFF
				}
			} else if v == 0x30 {
				// 4 KiB sector erase.
				off := int(addr & 0x7F000)
				if off+0x1000 <= len(data) {
					for i := off; i < off+0x1000; i++ {
						data[i] = 0xFF
					}
				}
			}
			f.reset()
		default:
			f.reset()
		}
	}
}
