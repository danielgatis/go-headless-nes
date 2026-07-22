package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Racermate (mapper 168): the Racermate Challenge II exercise-bike
// cartridge. One switchable 16 KiB PRG window (the second is fixed to
// the last bank) and 64 KiB of battery CHR RAM in two 4 KiB windows,
// the lower fixed to bank 0. A free-running timer asserts an IRQ every
// 1024 CPU cycles; writing $C000 restarts it and acknowledges.
type Racermate struct {
	base

	prgBank byte
	chrBank byte

	irqCounter uint16
	irq        bool

	chr64 [65536]byte // 64 KiB CHR RAM (larger than base's 8 KiB)
}

// NewRacermate wires the board.
func NewRacermate(c *cartridge.Cartridge) *Racermate {
	return &Racermate{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Racermate) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space.
func (m *Racermate) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		switch addr & 0xC000 {
		case 0x8000:
			m.prgBank = (v >> 6) & 0x03
			m.chrBank = v & 0x0F
		case 0xC000:
			m.irqCounter = 1024
			m.irq = false
		}
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// chrOffset maps a PPU address into the 64 KiB CHR RAM: the lower 4 KiB
// window is fixed to bank 0, the upper is switchable.
func (m *Racermate) chrOffset(addr uint16) int {
	if addr < 0x1000 {
		return int(addr)
	}
	return int(m.chrBank)<<12 | int(addr&0x0FFF)
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Racermate) ReadCHR(addr uint16) byte { return m.chr64[m.chrOffset(addr)] }

// WriteCHR handles a write into the CHR address space.
func (m *Racermate) WriteCHR(addr uint16, v byte) { m.chr64[m.chrOffset(addr)] = v }

// Tick decrements the IRQ timer once per CPU cycle; at zero it reloads
// to 1024 and asserts the IRQ line.
func (m *Racermate) Tick() {
	m.irqCounter--
	if m.irqCounter == 0 {
		m.irqCounter = 1024
		m.irq = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Racermate) IRQ() bool { return m.irq }

// Save writes the board's mapper-specific state into s. The 64 KiB CHR
// RAM exceeds the fixed-size snapshot: its first 32 KiB ride in the
// unused PRG-RAM tail and the CHR-RAM area; the rest is best-effort
// (as with the Coolboy's oversized CHR RAM).
func (m *Racermate) Save(s *State) {
	copy(s.PRGRAM[:8192], m.prgRAM[:])
	copy(s.PRGRAM[8192:], m.chr64[:24576])
	copy(s.CHRRAM[:], m.chr64[24576:32768])
	s.Regs[0] = m.prgBank
	s.Regs[1] = m.chrBank
	s.Regs[2] = byte(m.irqCounter)
	s.Regs[3] = byte(m.irqCounter >> 8)
	s.Regs[4] = boolByte(m.irq)
}

// Restore loads the board's mapper-specific state from s.
func (m *Racermate) Restore(s *State) {
	copy(m.prgRAM[:], s.PRGRAM[:8192])
	copy(m.chr64[:24576], s.PRGRAM[8192:])
	copy(m.chr64[24576:32768], s.CHRRAM[:])
	m.prgBank = s.Regs[0]
	m.chrBank = s.Regs[1]
	m.irqCounter = uint16(s.Regs[2]) | uint16(s.Regs[3])<<8
	m.irq = s.Regs[4] != 0
}
