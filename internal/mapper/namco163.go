package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Namco variants covered by the 163 board family.
const (
	namco163 byte = iota
	namco175
	namco340
	namcoUnknown
)

// Namco163 (mappers 19 and 210) banks PRG in 8 KiB and CHR in 1 KiB
// with twelve slots — the last four drive the nametables, which any
// slot can also source from console VRAM (register values $E0-$FF).
// It has a 15-bit CPU-cycle IRQ up-counter, per-2-KiB save RAM write
// protection and 128 bytes of internal sound RAM behind an auto-
// incrementing data port, with the wavetable channels mixed into the
// APU output.
// Mapper 210 boards (175/340) drop the IRQ/sound and use plain
// mirroring; without a submapper the variant is auto-detected from the
// register traffic.
type Namco163 struct {
	base

	variant    byte
	autoDetect bool
	notN340    bool

	prgBanks [3]byte
	chrRegs  [12]byte // 8 pattern slots + 4 nametable slots

	lowChrNT  bool // low pattern slots: $E0+ values stay CHR ROM
	highChrNT bool

	writeProtect byte
	irqCounter   uint16
	irqLine      bool

	soundAddr byte // sound RAM address + auto-increment flag (bit 7)
	soundRAM  [128]byte
	audio     n163Audio

	ciramRead  func(idx uint16) byte
	ciramWrite func(idx uint16, v byte)
}

// NewNamco163 wires the board family.
func NewNamco163(c *cartridge.Cartridge) *Namco163 {
	m := &Namco163{base: makeBase(c)}
	switch c.MapperID {
	case 19:
		m.variant = namco163
	case 210:
		switch c.Submapper {
		case 1:
			m.variant = namco175
		case 2:
			m.variant = namco340
		default:
			m.variant = namcoUnknown
			m.autoDetect = true
		}
	}
	// Initialize the nametable slots to console VRAM so the board comes
	// up with ordinary mirroring behavior.
	for i := 8; i < 12; i++ {
		m.chrRegs[i] = 0xE0 | byte(i&1)
	}
	return m
}

// SetCIRAM receives the console VRAM accessors.
func (m *Namco163) SetCIRAM(read func(idx uint16) byte, write func(idx uint16, v byte)) {
	m.ciramRead = read
	m.ciramWrite = write
}

func (m *Namco163) setVariant(v byte) {
	if m.autoDetect && (!m.notN340 || v != namco340) {
		m.variant = v
	}
}

// ramWritable reports whether the $6000-$7FFF slice holding addr accepts
// writes under the current protection register.
func (m *Namco163) ramWritable(addr uint16) bool {
	switch m.variant {
	case namco163:
		if m.writeProtect&0x40 == 0 {
			return false
		}
		section := (addr >> 11) & 0x03
		return m.writeProtect&(1<<section) == 0
	case namco175:
		return m.writeProtect&0x01 != 0
	default:
		return false
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Namco163) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return window(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBanks[(addr-0x8000)>>13]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.variant == namco340 {
			return m.openBus()
		}
		return m.readPRGRAM(addr)
	case addr >= 0x5800:
		return byte(m.irqCounter >> 8)
	case addr >= 0x5000:
		return byte(m.irqCounter)
	case addr >= 0x4800:
		// Sound RAM data port with optional auto-increment.
		v := m.soundRAM[m.soundAddr&0x7F]
		if m.soundAddr&0x80 != 0 {
			m.soundAddr = (m.soundAddr & 0x80) | ((m.soundAddr + 1) & 0x7F)
		}
		return v
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Namco163) WritePRG(addr uint16, v byte) {
	switch addr & 0xF800 {
	case 0x4800:
		m.setVariant(namco163)
		m.soundRAM[m.soundAddr&0x7F] = v
		if m.soundAddr&0x80 != 0 {
			m.soundAddr = (m.soundAddr & 0x80) | ((m.soundAddr + 1) & 0x7F)
		}
	case 0x5000:
		m.setVariant(namco163)
		m.irqCounter = (m.irqCounter & 0xFF00) | uint16(v)
		m.irqLine = false
	case 0x5800:
		m.setVariant(namco163)
		m.irqCounter = (m.irqCounter & 0x00FF) | uint16(v)<<8
		m.irqLine = false
	case 0x6000, 0x6800, 0x7000, 0x7800:
		m.notN340 = true
		if m.variant == namco340 {
			m.setVariant(namcoUnknown)
		}
		if m.ramWritable(addr) {
			m.writePRGRAM(addr, v)
		}
	case 0x8000, 0x8800, 0x9000, 0x9800:
		m.chrRegs[(addr-0x8000)>>11] = v
	case 0xA000, 0xA800, 0xB000, 0xB800:
		m.chrRegs[4+(addr-0xA000)>>11] = v
	case 0xC000, 0xC800, 0xD000, 0xD800:
		if addr >= 0xC800 {
			m.setVariant(namco163)
		} else if m.variant != namco163 {
			m.setVariant(namco175)
		}
		if m.variant == namco175 {
			m.writeProtect = v
		} else {
			m.chrRegs[8+(addr-0xC000)>>11] = v
		}
	case 0xE000:
		if v&0x80 != 0 {
			m.setVariant(namco340)
		} else if v&0x40 != 0 && m.variant != namco163 {
			m.setVariant(namco340)
		}
		m.prgBanks[0] = v & 0x3F
		if m.variant == namco340 {
			switch (v & 0xC0) >> 6 {
			case 0:
				m.mirroring = cartridge.SingleLow
			case 1:
				m.mirroring = cartridge.Vertical
			case 2:
				m.mirroring = cartridge.SingleHigh
			case 3:
				m.mirroring = cartridge.Horizontal
			}
		}
		if m.variant == namco163 {
			m.audio.Disabled = v&0x40 != 0
		}
	case 0xE800:
		m.prgBanks[1] = v & 0x3F
		if m.variant == namco163 {
			m.lowChrNT = v&0x40 != 0
			m.highChrNT = v&0x80 != 0
		}
	case 0xF000:
		m.prgBanks[2] = v & 0x3F
	case 0xF800:
		m.setVariant(namco163)
		if m.variant == namco163 {
			m.writeProtect = v
			m.soundAddr = v
		}
	}
}

// Tick advances the board by one cycle.
func (m *Namco163) Tick() {
	if m.variant == namco163 {
		m.audio.clock(&m.soundRAM)
	}
	if m.irqCounter&0x8000 != 0 && m.irqCounter&0x7FFF != 0x7FFF {
		m.irqCounter++
		if m.irqCounter&0x7FFF == 0x7FFF {
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Namco163) IRQ() bool { return m.irqLine }

// AudioLevel mixes the wavetable channels.
func (m *Namco163) AudioLevel() float32 {
	if m.variant != namco163 {
		return 0
	}
	return expN163Step * float32(m.audio.output(&m.soundRAM))
}

// chrFromCIRAM reports whether the pattern slot's register points at
// console VRAM.
func (m *Namco163) chrFromCIRAM(slot int) bool {
	if m.variant != namco163 || m.chrRegs[slot] < 0xE0 {
		return false
	}
	if slot < 4 {
		return !m.lowChrNT
	}
	return !m.highChrNT
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Namco163) ReadCHR(addr uint16) byte {
	slot := int(addr >> 10)
	if m.chrFromCIRAM(slot) && m.ciramRead != nil {
		return m.ciramRead(uint16(m.chrRegs[slot]&0x01)*0x400 + (addr & 0x3FF))
	}
	return m.chrRead(int(m.chrRegs[slot]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Namco163) WriteCHR(addr uint16, v byte) {
	slot := int(addr >> 10)
	if m.chrFromCIRAM(slot) && m.ciramWrite != nil {
		m.ciramWrite(uint16(m.chrRegs[slot]&0x01)*0x400+(addr&0x3FF), v)
		return
	}
	m.chrWrite(int(m.chrRegs[slot]), 0x400, addr, v)
}

// ReadNT serves nametable fetches from CHR ROM when the slot register
// points there; CIRAM values fall back to NametablePage.
func (m *Namco163) ReadNT(addr uint16) (byte, bool) {
	if m.variant != namco163 {
		return 0, false
	}
	reg := m.chrRegs[8+(addr>>10)&3]
	if reg >= 0xE0 {
		return 0, false
	}
	return m.chrRead(int(reg), 0x400, addr), true
}

// WriteNT handles a nametable write the board intercepts.
func (m *Namco163) WriteNT(addr uint16, v byte) bool {
	if m.variant != namco163 {
		return false
	}
	reg := m.chrRegs[8+(addr>>10)&3]
	if reg >= 0xE0 {
		return false
	}
	m.chrWrite(int(reg), 0x400, addr, v)
	return true
}

// NametablePage picks the CIRAM page per table (163) or falls back to
// the plain mirroring modes (175/340).
func (m *Namco163) NametablePage(table byte) byte {
	if m.variant == namco163 {
		return m.chrRegs[8+table&3] & 0x01
	}
	switch m.mirroring {
	case cartridge.Horizontal:
		return table >> 1
	case cartridge.Vertical:
		return table & 1
	case cartridge.SingleHigh:
		return 1
	default:
		return 0
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Namco163) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:3], m.prgBanks[:])
	copy(s.Regs[3:15], m.chrRegs[:])
	s.Regs[15] = m.writeProtect
	s.Regs[16] = byte(m.irqCounter)
	s.Regs[17] = byte(m.irqCounter >> 8)
	s.Regs[18] = boolByte(m.irqLine) | boolByte(m.lowChrNT)<<1 | boolByte(m.highChrNT)<<2 |
		boolByte(m.autoDetect)<<3 | boolByte(m.notN340)<<4
	s.Regs[19] = m.variant
	s.Regs[20] = m.soundAddr
	s.Regs[21] = byte(m.mirroring)
	// The 163 carries CHR ROM; the CHRRAM snapshot area holds sound RAM
	// and the wavetable engine state.
	copy(s.CHRRAM[0:128], m.soundRAM[:])
	for i, o := range m.audio.ChannelOut {
		s.CHRRAM[128+i*2] = byte(o)
		s.CHRRAM[129+i*2] = byte(o >> 8)
	}
	s.CHRRAM[144] = m.audio.UpdateCount
	s.CHRRAM[145] = byte(m.audio.CurrentCh)
	s.CHRRAM[146] = boolByte(m.audio.Disabled)
}

// Restore loads the board's mapper-specific state from s.
func (m *Namco163) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:3])
	copy(m.chrRegs[:], s.Regs[3:15])
	m.writeProtect = s.Regs[15]
	m.irqCounter = uint16(s.Regs[16]) | uint16(s.Regs[17])<<8
	m.irqLine = s.Regs[18]&1 != 0
	m.lowChrNT = s.Regs[18]&2 != 0
	m.highChrNT = s.Regs[18]&4 != 0
	m.autoDetect = s.Regs[18]&8 != 0
	m.notN340 = s.Regs[18]&16 != 0
	m.variant = s.Regs[19]
	m.soundAddr = s.Regs[20]
	m.mirroring = cartridge.Mirroring(s.Regs[21])
	copy(m.soundRAM[:], s.CHRRAM[0:128])
	for i := range m.audio.ChannelOut {
		m.audio.ChannelOut[i] = int16(uint16(s.CHRRAM[128+i*2]) | uint16(s.CHRRAM[129+i*2])<<8)
	}
	m.audio.UpdateCount = s.CHRRAM[144]
	m.audio.CurrentCh = int8(s.CHRRAM[145])
	m.audio.Disabled = s.CHRRAM[146] != 0
}
