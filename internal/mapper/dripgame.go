package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// DripGame (mapper 284): quietust's homebrew "Drip" board. One
// switchable 16 KiB PRG window, four 2 KiB CHR windows, 8 KiB work RAM
// with a write-enable bit, a one-shot CPU-cycle IRQ timer, two FIFO
// sample channels mixed into the APU output, and an extended-attribute
// mode: 2 KiB of expansion VRAM at $C000 supplies a palette index per
// tile (instead of per 4x4 tile group) during attribute fetches.
type DripGame struct {
	base

	prgBank  byte
	chrBanks [4]byte
	mirror   cartridge.Mirroring

	irqLow      byte
	irqCounter  uint16
	irqEnabled  bool
	irqAsserted bool

	extAttrEnabled bool
	wramWritable   bool
	lastNtFetch    uint16
	extAttr        [2][1024]byte

	ch [2]dripChannel
}

// NewDripGame wires the board.
func NewDripGame(c *cartridge.Cartridge) *DripGame {
	m := &DripGame{base: makeBase(c), mirror: c.Mirroring}
	m.ch[0].empty = true
	m.ch[1].empty = true
	return m
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *DripGame) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return window(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return window(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	case addr >= 0x4800:
		switch addr & 0x5800 {
		case 0x4800:
			return 0x64 // status/DIP port, switch open
		case 0x5000:
			return m.ch[0].status()
		default: // 0x5800
			return m.ch[1].status()
		}
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space.
func (m *DripGame) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0xC000:
		// Attribute expansion memory, mirrored through $C000-$FFFF.
		m.extAttr[addr>>10&1][addr&0x3FF] = v
	case addr >= 0x8000:
		m.writeRegister(addr, v)
	case addr >= 0x6000:
		if m.wramWritable {
			m.writePRGRAM(addr, v)
		}
	}
}

// writeRegister decodes the $8000-$BFFF register file.
func (m *DripGame) writeRegister(addr uint16, v byte) {
	switch addr & 0x800F {
	case 0x8000, 0x8001, 0x8002, 0x8003:
		m.ch[0].writeReg(addr, v)
	case 0x8004, 0x8005, 0x8006, 0x8007:
		m.ch[1].writeReg(addr, v)
	case 0x8008:
		m.irqLow = v
	case 0x8009:
		// The low byte is buffered until the high write arms the timer;
		// writing here also acknowledges a pending IRQ.
		m.irqCounter = uint16(v&0x7F)<<8 | uint16(m.irqLow)
		m.irqEnabled = v&0x80 != 0
		m.irqAsserted = false
	case 0x800A:
		switch v & 0x03 {
		case 0:
			m.mirror = cartridge.Vertical
		case 1:
			m.mirror = cartridge.Horizontal
		case 2:
			m.mirror = cartridge.SingleLow
		case 3:
			m.mirror = cartridge.SingleHigh
		}
		m.extAttrEnabled = v&0x04 != 0
		m.wramWritable = v&0x08 != 0
	case 0x800B:
		m.prgBank = v & 0x0F
	case 0x800C, 0x800D, 0x800E, 0x800F:
		m.chrBanks[addr&0x03] = v & 0x0F
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *DripGame) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>11&3]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *DripGame) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>11&3]), 0x800, addr, v)
}

// Mirroring reports the board's current nametable mirroring.
func (m *DripGame) Mirroring() cartridge.Mirroring { return m.mirror }

// ReadNT intercepts attribute fetches when extended attributes are on:
// it remembers the last nametable fetch and answers the following
// attribute fetch with that tile's own palette, replicated into all
// four 2-bit slots so the PPU's shift picks the right one.
func (m *DripGame) ReadNT(addr uint16) (byte, bool) {
	if !m.extAttrEnabled {
		return 0, false
	}
	if addr&0x3FF < 0x3C0 {
		m.lastNtFetch = addr & 0x3FF
		return 0, false
	}
	bank := 0
	switch m.mirror {
	case cartridge.SingleLow:
		bank = 0
	case cartridge.SingleHigh:
		bank = 1
	case cartridge.Horizontal:
		if addr&0x800 != 0 {
			bank = 1
		}
	default: // vertical
		if addr&0x400 != 0 {
			bank = 1
		}
	}
	v := m.extAttr[bank][m.lastNtFetch] & 0x03
	return v<<6 | v<<4 | v<<2 | v, true
}

// WriteNT handles a nametable write the board intercepts (never).
func (m *DripGame) WriteNT(uint16, byte) bool { return false }

// Tick runs the one-shot IRQ timer and clocks both sample channels.
func (m *DripGame) Tick() {
	if m.irqEnabled && m.irqCounter > 0 {
		m.irqCounter--
		if m.irqCounter == 0 {
			// The timer stops at zero and pulls the IRQ line low.
			m.irqEnabled = false
			m.irqAsserted = true
		}
	}
	m.ch[0].clock()
	m.ch[1].clock()
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *DripGame) IRQ() bool { return m.irqAsserted }

// AudioLevel mixes the two sample channels.
func (m *DripGame) AudioLevel() float32 {
	return float32(int(m.ch[0].out)+int(m.ch[1].out)) * expDripStep
}

// Save writes the board's mapper-specific state into s. The extended
// attribute VRAM and FIFO buffers ride in the unused PRG-RAM tail.
func (m *DripGame) Save(s *State) {
	m.saveRAM(s)
	copy(s.PRGRAM[8192:9216], m.extAttr[0][:])
	copy(s.PRGRAM[9216:10240], m.extAttr[1][:])
	copy(s.PRGRAM[10240:10496], m.ch[0].buf[:])
	copy(s.PRGRAM[10496:10752], m.ch[1].buf[:])
	s.Regs[0] = m.prgBank
	copy(s.Regs[1:5], m.chrBanks[:])
	s.Regs[5] = byte(m.mirror)
	s.Regs[6] = m.irqLow
	s.Regs[7] = byte(m.irqCounter)
	s.Regs[8] = byte(m.irqCounter >> 8)
	s.Regs[9] = boolByte(m.irqEnabled)
	s.Regs[10] = boolByte(m.irqAsserted)
	s.Regs[11] = boolByte(m.extAttrEnabled)
	s.Regs[12] = boolByte(m.wramWritable)
	s.Regs[13] = byte(m.lastNtFetch)
	s.Regs[14] = byte(m.lastNtFetch >> 8)
	m.ch[0].save(s.Regs[15:24])
	m.ch[1].save(s.Regs[24:33])
}

// Restore loads the board's mapper-specific state from s.
func (m *DripGame) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.extAttr[0][:], s.PRGRAM[8192:9216])
	copy(m.extAttr[1][:], s.PRGRAM[9216:10240])
	copy(m.ch[0].buf[:], s.PRGRAM[10240:10496])
	copy(m.ch[1].buf[:], s.PRGRAM[10496:10752])
	m.prgBank = s.Regs[0]
	copy(m.chrBanks[:], s.Regs[1:5])
	m.mirror = cartridge.Mirroring(s.Regs[5])
	m.irqLow = s.Regs[6]
	m.irqCounter = uint16(s.Regs[7]) | uint16(s.Regs[8])<<8
	m.irqEnabled = s.Regs[9] != 0
	m.irqAsserted = s.Regs[10] != 0
	m.extAttrEnabled = s.Regs[11] != 0
	m.wramWritable = s.Regs[12] != 0
	m.lastNtFetch = uint16(s.Regs[13]) | uint16(s.Regs[14])<<8
	m.ch[0].restore(s.Regs[15:24])
	m.ch[1].restore(s.Regs[24:33])
}

// dripChannel is one FIFO sample channel: the CPU pushes 8-bit samples,
// the channel pops one every freq cycles and holds it on its DAC.
type dripChannel struct {
	buf      [256]byte
	readPos  byte
	writePos byte
	full     bool
	empty    bool

	freq   uint16
	timer  uint16
	volume byte
	out    int16
}

func (c *dripChannel) status() byte {
	v := byte(0)
	if c.full {
		v |= 0x80
	}
	if c.empty {
		v |= 0x40
	}
	return v
}

func (c *dripChannel) setOutput(sample byte) {
	c.out = (int16(sample) - 0x80) * int16(c.volume)
}

func (c *dripChannel) clock() {
	if c.empty {
		return
	}
	c.timer--
	if c.timer != 0 {
		return
	}
	c.timer = c.freq
	if c.readPos == c.writePos {
		c.full = false
	}
	c.readPos++
	c.setOutput(c.buf[c.readPos])
	if c.readPos == c.writePos {
		c.empty = true
	}
}

func (c *dripChannel) writeReg(addr uint16, v byte) {
	switch addr & 0x03 {
	case 0:
		// Clear FIFO: silence the channel and reload the timer.
		c.buf = [256]byte{}
		c.readPos = 0
		c.writePos = 0
		c.full = false
		c.empty = true
		c.out = 0
		c.timer = c.freq
	case 1:
		// Push a sample; pushing to an empty channel starts playback.
		if c.readPos == c.writePos {
			c.empty = false
			c.setOutput(v)
			c.timer = c.freq
		}
		c.buf[c.writePos] = v
		c.writePos++
		if c.readPos == c.writePos {
			c.full = true
		}
	case 2:
		c.freq = c.freq&0x0F00 | uint16(v)
	case 3:
		// Period high bits take effect with the next sample; volume
		// changes apply immediately.
		c.freq = c.freq&0x00FF | uint16(v&0x0F)<<8
		c.volume = v >> 4
		if !c.empty {
			c.setOutput(c.buf[c.readPos])
		}
	}
}

func (c *dripChannel) save(regs []byte) {
	regs[0] = c.readPos
	regs[1] = c.writePos
	regs[2] = boolByte(c.full)
	regs[3] = boolByte(c.empty)
	regs[4] = byte(c.freq)
	regs[5] = byte(c.freq >> 8)
	regs[6] = byte(c.timer)
	regs[7] = byte(c.timer >> 8)
	regs[8] = c.volume
}

func (c *dripChannel) restore(regs []byte) {
	c.readPos = regs[0]
	c.writePos = regs[1]
	c.full = regs[2] != 0
	c.empty = regs[3] != 0
	c.freq = uint16(regs[4]) | uint16(regs[5])<<8
	c.timer = uint16(regs[6]) | uint16(regs[7])<<8
	c.volume = regs[8]
	if !c.empty {
		c.setOutput(c.buf[c.readPos])
	} else {
		c.out = 0
	}
}
