package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// MMC5 (mapper 5), Nintendo's most capable board: four PRG modes mixing
// ROM and RAM, four CHR modes with separate sprite/background bank sets
// for 8x16 sprites, 1 KiB of ExRAM usable as a nametable, extended
// attributes or CPU scratch, fill-mode nametables, a scanline IRQ driven
// by watching the PPU's fetch pattern, a vertical split window, and an
// 8x8-bit multiplier. The board's audio (two pulses + PCM) is mixed
// into the APU output. Up to 32 KiB of work RAM is emulated.
type MMC5 struct {
	base

	wram  [0x8000]byte
	exRAM [0x400]byte

	prgRAMProtect1 byte
	prgRAMProtect2 byte

	fillTile  byte
	fillColor byte

	splitEnabled   bool
	splitRightSide bool
	splitDelimiter byte
	splitScroll    byte
	splitBank      byte
	splitInRegion  bool
	splitTile      uint16
	splitTileNum   int32

	mul1 byte
	mul2 byte

	ntMapping byte
	exRAMMode byte

	exAttrLastNT  uint16
	exAttrCounter int8
	exAttrChrBank byte

	prgMode  byte
	prgBanks [5]byte // $5113-$5117

	chrMode    byte
	chrUpper   byte
	chrBanks   [12]uint16 // $5120-$512B
	lastChrReg uint16

	irqTarget       byte
	irqEnabled      bool
	scanlineCounter byte
	irqPending      bool
	needInFrame     bool
	ppuInFrame      bool
	ppuIdle         byte
	lastPpuReadAddr uint16
	ntReadCounter   byte

	ppuCtrl byte // sniffed $2000 (for the 8x16-sprite CHR set select)

	// PCM channel state.
	pcmReadMode   bool
	pcmIrqEnabled bool
	pcmIrqPending bool
	pcmOutput     byte

	sq1, sq2     mmc5Square
	audioCounter int32

	ciramRead  func(idx uint16) byte
	ciramWrite func(idx uint16, v byte)
}

// NewMMC5 wires an MMC5 board in its documented power-up state.
func NewMMC5(c *cartridge.Cartridge) *MMC5 {
	m := &MMC5{base: makeBase(c)}
	m.prgMode = 3
	m.prgBanks[4] = 0xFF
	// The MMC5A powers on with the DAC at $EF or $FF; without randomized
	// power-on state, $EF.
	m.pcmOutput = 0xEF
	return m
}

// SetCIRAM receives the console VRAM accessors.
func (m *MMC5) SetCIRAM(read func(idx uint16) byte, write func(idx uint16, v byte)) {
	m.ciramRead = read
	m.ciramWrite = write
}

// SniffPPUReg records CPU writes to $2000 (sprite size).
func (m *MMC5) SniffPPUReg(addr uint16, v byte) {
	if addr&0x07 == 0 {
		m.ppuCtrl = v
	}
}

func (m *MMC5) largeSprites() bool { return m.ppuCtrl&0x20 != 0 }

// --- PRG side ---

// prgSlot resolves a CPU address to (register index, 8 KiB page within
// the register's window).
func (m *MMC5) prgSlot(addr uint16) (reg int, page8k int) {
	switch m.prgMode {
	case 0:
		return 4, int(m.prgBanks[4]&0x7C) + int((addr-0x8000)>>13)
	case 1:
		if addr < 0xC000 {
			return 2, int(m.prgBanks[2]&0xFE) + int((addr-0x8000)>>13)
		}
		return 4, int(m.prgBanks[4]&0x7E) + int((addr-0xC000)>>13)
	case 2:
		switch {
		case addr < 0xC000:
			return 2, int(m.prgBanks[2]&0xFE) + int((addr-0x8000)>>13)
		case addr < 0xE000:
			return 3, int(m.prgBanks[3] & 0x7F)
		default:
			return 4, int(m.prgBanks[4] & 0x7F)
		}
	default: // 3
		switch {
		case addr < 0xA000:
			return 1, int(m.prgBanks[1] & 0x7F)
		case addr < 0xC000:
			return 2, int(m.prgBanks[2] & 0x7F)
		case addr < 0xE000:
			return 3, int(m.prgBanks[3] & 0x7F)
		default:
			return 4, int(m.prgBanks[4] & 0x7F)
		}
	}
}

// prgIsRAM reports whether the register currently selects work RAM: the
// top bit selects ROM, and $5117 is hard-wired to ROM.
func (m *MMC5) prgIsRAM(reg int) bool {
	return reg != 4 && m.prgBanks[reg]&0x80 == 0
}

func (m *MMC5) wramWritable() bool {
	return m.prgRAMProtect1 == 0x02 && m.prgRAMProtect2 == 0x01
}

func (m *MMC5) wramWindow(bank int) []byte {
	return m.win(m.wram[:], bank&0x07, 0x2000)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC5) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xFFFA && addr <= 0xFFFB:
		// NMI vector fetches force the in-frame flag off.
		m.ppuInFrame = false
		m.lastPpuReadAddr = 0
		m.scanlineCounter = 0
		m.irqPending = false
		reg, page := m.prgSlot(addr)
		_ = reg
		return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		reg, page := m.prgSlot(addr)
		var v byte
		if m.prgIsRAM(reg) {
			v = m.wramWindow(int(m.prgBanks[reg]))[addr&0x1FFF]
		} else {
			v = m.win(m.prg, page, 0x2000)[addr&0x1FFF]
		}
		if m.pcmReadMode && addr&0xC000 == 0x8000 {
			// PCM read mode: every CPU read from $8000-$BFFF feeds the DAC.
			m.dacWrite(v)
		}
		return v
	case addr >= 0x6000:
		return m.wramWindow(int(m.prgBanks[0]))[addr&0x1FFF]
	case addr >= 0x5C00:
		// ExRAM: readable only in modes 2 and 3.
		if m.exRAMMode >= 2 {
			return m.exRAM[addr&0x3FF]
		}
		return m.openBus()
	case addr >= 0x5000:
		return m.readRegister(addr)
	}
	return m.openBus()
}

// dacWrite feeds the PCM DAC: writing (or read-mode feeding) zero does
// not change the level, it raises the PCM IRQ instead.
func (m *MMC5) dacWrite(v byte) {
	if v == 0 {
		m.pcmIrqPending = true
	} else {
		m.pcmOutput = v
	}
}

func (m *MMC5) readRegister(addr uint16) byte {
	switch addr {
	case 0x5010:
		if m.pcmIrqPending {
			m.pcmIrqPending = false
			return 0x80
		}
		return 0x00
	case 0x5015:
		var v byte
		if m.sq1.LengthVal > 0 {
			v |= 0x01
		}
		if m.sq2.LengthVal > 0 {
			v |= 0x02
		}
		return v
	case 0x5204:
		v := byte(0)
		if m.ppuInFrame {
			v |= 0x40
		}
		if m.irqPending {
			v |= 0x80
		}
		m.irqPending = false
		return v
	case 0x5205:
		return byte(uint16(m.mul1) * uint16(m.mul2))
	case 0x5206:
		return byte(uint16(m.mul1) * uint16(m.mul2) >> 8)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC5) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		reg, _ := m.prgSlot(addr)
		if m.prgIsRAM(reg) && m.wramWritable() {
			m.wramWindow(int(m.prgBanks[reg]))[addr&0x1FFF] = v
		}
	case addr >= 0x6000:
		if m.wramWritable() {
			m.wramWindow(int(m.prgBanks[0]))[addr&0x1FFF] = v
		}
	case addr >= 0x5C00:
		// Modes 0/1: writable only while the PPU renders; otherwise
		// zero is written. Mode 3 is read-only.
		switch {
		case m.exRAMMode <= 1:
			if !m.ppuInFrame {
				v = 0
			}
			m.exRAM[addr&0x3FF] = v
		case m.exRAMMode == 2:
			m.exRAM[addr&0x3FF] = v
		}
	case addr >= 0x5000:
		m.writeRegister(addr, v)
	}
}

func (m *MMC5) writeRegister(addr uint16, v byte) {
	switch {
	case addr >= 0x5113 && addr <= 0x5117:
		m.prgBanks[addr-0x5113] = v
	case addr >= 0x5120 && addr <= 0x512B:
		m.chrBanks[addr-0x5120] = uint16(v) | uint16(m.chrUpper)<<8
		m.lastChrReg = addr
	default:
		switch addr {
		case 0x5000, 0x5001, 0x5002, 0x5003:
			m.sq1.writeReg(addr, v)
		case 0x5004, 0x5005, 0x5006, 0x5007:
			m.sq2.writeReg(addr, v)
		case 0x5015:
			m.sq1.setEnabled(v&0x01 != 0)
			m.sq2.setEnabled(v&0x02 != 0)
		case 0x5010:
			m.pcmReadMode = v&0x01 != 0
			m.pcmIrqEnabled = v&0x80 != 0
		case 0x5011:
			if !m.pcmReadMode {
				m.dacWrite(v)
			}
		case 0x5100:
			m.prgMode = v & 0x03
		case 0x5101:
			m.chrMode = v & 0x03
		case 0x5102:
			m.prgRAMProtect1 = v & 0x03
		case 0x5103:
			m.prgRAMProtect2 = v & 0x03
		case 0x5104:
			m.exRAMMode = v & 0x03
		case 0x5105:
			m.ntMapping = v
		case 0x5106:
			m.fillTile = v
		case 0x5107:
			m.fillColor = v & 0x03
		case 0x5130:
			m.chrUpper = v & 0x03
		case 0x5200:
			m.splitEnabled = v&0x80 != 0
			m.splitRightSide = v&0x40 != 0
			m.splitDelimiter = v & 0x1F
		case 0x5201:
			m.splitScroll = v
		case 0x5202:
			m.splitBank = v
		case 0x5203:
			m.irqTarget = v
		case 0x5204:
			m.irqEnabled = v&0x80 != 0
		case 0x5205:
			m.mul1 = v
		case 0x5206:
			m.mul2 = v
		}
	}
}

// --- IRQ / in-frame detection ---

// mmc5EnvelopeRate is the ~240 Hz envelope/length tick, in CPU cycles.
const mmc5EnvelopeRate = cpuHz / 240

// Tick advances the board by one cycle.
func (m *MMC5) Tick() {
	m.audioCounter--
	m.sq1.run()
	m.sq2.run()
	if m.audioCounter <= 0 {
		m.audioCounter = mmc5EnvelopeRate
		m.sq1.tickLength()
		m.sq1.tickEnvelope()
		m.sq2.tickLength()
		m.sq2.tickEnvelope()
	}
	m.sq1.reloadLength()
	m.sq2.reloadLength()

	if m.ppuIdle > 0 {
		m.ppuIdle--
		if m.ppuIdle == 0 {
			// Three CPU cycles without a PPU read: rendering stopped.
			m.ppuInFrame = false
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *MMC5) IRQ() bool {
	return (m.irqEnabled && m.irqPending) || (m.pcmIrqEnabled && m.pcmIrqPending)
}

// AudioLevel mixes the two squares and the PCM DAC. The MMC5 channels'
// polarity is reversed relative to the APU's.
func (m *MMC5) AudioLevel() float32 {
	return -(expPulseStep*float32(int(m.sq1.Output)+int(m.sq2.Output)) +
		expPCMStep*float32(m.pcmOutput))
}

// detectScanline reproduces the MMC5's in-frame/scanline detector: three
// identical nametable reads mark the start of a scanline.
func (m *MMC5) detectScanline(addr uint16) {
	if m.ntReadCounter >= 2 {
		if !m.ppuInFrame && !m.needInFrame {
			m.needInFrame = true
			m.scanlineCounter = 0
		} else {
			m.scanlineCounter++
			if m.irqTarget == m.scanlineCounter {
				m.irqPending = true
			}
		}
		m.ntReadCounter = 0
	} else if addr >= 0x2000 && addr <= 0x2FFF && m.lastPpuReadAddr == addr {
		m.ntReadCounter++
		if m.ntReadCounter >= 2 {
			m.splitTileNum = 0
		}
	}
	if m.lastPpuReadAddr != addr {
		m.ntReadCounter = 0
	}
	m.ppuIdle = 3
	m.lastPpuReadAddr = addr
}

// --- CHR side ---

// chrUseSetA reports which CHR bank set applies to the current fetch:
// with 8x16 sprites, set A ($5120-$5127) serves sprite fetches and set B
// ($5128-$512B) background fetches.
func (m *MMC5) chrUseSetA() bool {
	if !m.largeSprites() {
		return true
	}
	if m.splitTileNum >= 32 && m.splitTileNum < 48 {
		return true // sprite fetch region of the scanline
	}
	return !m.ppuInFrame && m.lastChrReg <= 0x5127
}

// chrPage1K resolves the pattern page for addr under the current mode.
func (m *MMC5) chrPage1K(addr uint16) int {
	slot := int(addr >> 10)
	a := m.chrUseSetA()
	pick := func(regA, regB int) uint16 {
		if a {
			return m.chrBanks[regA]
		}
		return m.chrBanks[regB]
	}
	switch m.chrMode {
	case 0:
		return int(pick(7, 11))<<3 + slot
	case 1:
		if slot < 4 {
			return int(pick(3, 11))<<2 + slot
		}
		return int(pick(7, 11))<<2 + (slot - 4)
	case 2:
		regsA := [4]int{1, 3, 5, 7}
		regsB := [4]int{9, 11, 9, 11}
		i := slot >> 1
		return int(pick(regsA[i], regsB[i]))<<1 + (slot & 1)
	default: // 3
		regsB := [8]int{8, 9, 10, 11, 8, 9, 10, 11}
		if a {
			return int(m.chrBanks[slot])
		}
		return int(m.chrBanks[regsB[slot]])
	}
}

// chrDirect reads pattern data at an absolute CHR position (split and
// extended-attribute fetches bypass the regular banking).
func (m *MMC5) chrDirect(pos int) byte {
	if m.chr != nil {
		return m.chr[pos%len(m.chr)]
	}
	return m.chrRAM[pos%len(m.chrRAM)]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC5) ReadCHR(addr uint16) byte {
	m.detectScanline(addr)
	if m.exRAMMode <= 1 && m.ppuInFrame {
		if m.splitEnabled && m.splitInRegion {
			scan := m.scanlineCounter
			if m.splitTileNum >= 49 {
				scan++
			}
			scroll := (uint16(scan) + uint16(m.splitScroll)) % 240
			return m.chrDirect(int(m.splitBank)<<12 + int((addr&^0x07|scroll&0x07)&0xFFF))
		}
		if m.exRAMMode == 1 && (m.splitTileNum < 32 || m.splitTileNum >= 48) && m.exAttrCounter > 0 {
			// Extended attributes: the two pattern fetches after the
			// attribute use the ExRAM-selected 4 KiB bank.
			m.exAttrCounter--
			return m.chrDirect(int(m.exAttrChrBank)<<12 + int(addr&0xFFF))
		}
	}
	return m.chrRead(m.chrPage1K(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC5) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage1K(addr), 0x400, addr, v)
}

// --- Nametable side ---

// ReadNT serves every nametable fetch: the four logical tables map to
// CIRAM, ExRAM, the fill nametable or nothing, with the vertical-split
// and extended-attribute machinery observing (and sometimes overriding)
// the data.
func (m *MMC5) ReadNT(addr uint16) (byte, bool) {
	isNtFetch := addr&0x03FF < 0x3C0
	if isNtFetch {
		m.splitTileNum++
		if m.needInFrame && !m.ppuInFrame {
			m.needInFrame = false
			m.ppuInFrame = true
		}
	}
	m.detectScanline(addr)

	if m.exRAMMode <= 1 && m.ppuInFrame {
		if m.splitEnabled {
			if v, ok := m.splitReadNT(addr, isNtFetch); ok {
				return v, true
			}
		}
		if m.exRAMMode == 1 && (m.splitTileNum < 32 || m.splitTileNum >= 48) {
			if isNtFetch {
				m.exAttrLastNT = addr & 0x03FF
				m.exAttrCounter = 3
			} else if m.exAttrCounter > 0 {
				m.exAttrCounter--
				// The attribute fetch: palette from ExRAM, replicated
				// into every 2-bit slot.
				val := m.exRAM[m.exAttrLastNT]
				m.exAttrChrBank = (val & 0x3F) | m.chrUpper<<6
				pal := (val & 0xC0) >> 6
				return pal * 0x55, true
			}
		}
	}
	return m.readNTSource(addr), true
}

// splitReadNT handles the vertical-split window's NT and attribute
// fetches, tracking whether the current column is inside the split.
func (m *MMC5) splitReadNT(_ uint16, isNtFetch bool) (byte, bool) {
	scan := m.scanlineCounter
	if m.splitTileNum >= 49 {
		scan++
	}
	scroll := (uint16(scan) + uint16(m.splitScroll)) % 240
	column := (m.splitTileNum + 2) % 50
	if isNtFetch {
		if column == 0 {
			m.splitInRegion = !m.splitRightSide
		}
		if column == int32(m.splitDelimiter) && m.splitTileNum < 50 {
			m.splitInRegion = !m.splitInRegion
		} else if column > 32 {
			m.splitInRegion = false
		}
		if m.splitInRegion {
			m.splitTile = (scroll&0xF8)<<2 | uint16(column)
			return m.exRAM[m.splitTile&0x3FF], true
		}
	} else if m.splitInRegion {
		shift := (m.splitTile >> 4 & 0x04) | (m.splitTile & 0x02)
		atAddr := 0x3C0 | (m.splitTile&0x380)>>4 | (m.splitTile&0x1F)>>2
		pal := (m.exRAM[atAddr&0x3FF] >> shift) & 0x03
		return pal * 0x55, true
	}
	return 0, false
}

// readNTSource reads the logical table's configured backing store.
func (m *MMC5) readNTSource(addr uint16) byte {
	table := (addr >> 10) & 0x03
	off := addr & 0x03FF
	switch (m.ntMapping >> (table * 2)) & 0x03 {
	case 0:
		if m.ciramRead == nil {
			return 0
		}
		return m.ciramRead(off)
	case 1:
		if m.ciramRead == nil {
			return 0
		}
		return m.ciramRead(0x400 + off)
	case 2:
		if m.exRAMMode <= 1 {
			return m.exRAM[off]
		}
		return 0
	default: // fill mode
		if off < 0x3C0 {
			return m.fillTile
		}
		c := m.fillColor
		return c | c<<2 | c<<4 | c<<6
	}
}

// WriteNT handles a nametable write the board intercepts.
func (m *MMC5) WriteNT(addr uint16, v byte) bool {
	table := (addr >> 10) & 0x03
	off := addr & 0x03FF
	switch (m.ntMapping >> (table * 2)) & 0x03 {
	case 0:
		if m.ciramWrite != nil {
			m.ciramWrite(off, v)
		}
	case 1:
		if m.ciramWrite != nil {
			m.ciramWrite(0x400+off, v)
		}
	case 2:
		if m.exRAMMode <= 1 {
			m.exRAM[off] = v
		}
	}
	return true
}

// --- Snapshot ---

// Save writes the board's mapper-specific state into s.
func (m *MMC5) Save(s *State) {
	copy(s.PRGRAM[:], m.wram[:])
	copy(s.CHRRAM[0:0x400], m.exRAM[:])
	r := s.Regs[:]
	copy(r[0:5], m.prgBanks[:])
	for i, b := range m.chrBanks {
		r[5+i*2] = byte(b)
		r[6+i*2] = byte(b >> 8)
	}
	r[29] = m.prgMode
	r[30] = m.chrMode
	r[31] = m.chrUpper
	r[32] = byte(m.lastChrReg)
	r[33] = byte(m.lastChrReg >> 8)
	r[34] = m.prgRAMProtect1
	r[35] = m.prgRAMProtect2
	r[36] = m.fillTile
	r[37] = m.fillColor
	r[38] = m.ntMapping
	r[39] = m.exRAMMode
	r[40] = m.splitDelimiter
	r[41] = m.splitScroll
	r[42] = m.splitBank
	r[43] = boolByte(m.splitEnabled) | boolByte(m.splitRightSide)<<1 |
		boolByte(m.splitInRegion)<<2 | boolByte(m.irqEnabled)<<3 |
		boolByte(m.irqPending)<<4 | boolByte(m.needInFrame)<<5 |
		boolByte(m.ppuInFrame)<<6
	r[44] = byte(m.splitTile)
	r[45] = byte(m.splitTile >> 8)
	r[46] = byte(m.splitTileNum)
	r[47] = byte(m.splitTileNum >> 8)
	r[48] = byte(m.splitTileNum >> 16)
	r[49] = byte(m.splitTileNum >> 24)
	r[50] = m.mul1
	r[51] = m.mul2
	r[52] = byte(m.exAttrLastNT)
	r[53] = byte(m.exAttrLastNT >> 8)
	r[54] = byte(m.exAttrCounter)
	r[55] = m.exAttrChrBank
	r[56] = m.irqTarget
	r[57] = m.scanlineCounter
	r[58] = m.ppuIdle
	r[59] = byte(m.lastPpuReadAddr)
	r[60] = byte(m.lastPpuReadAddr >> 8)
	r[61] = m.ntReadCounter
	r[62] = m.ppuCtrl
	r[63] = boolByte(m.pcmReadMode) | boolByte(m.pcmIrqEnabled)<<1 | boolByte(m.pcmIrqPending)<<2
	r[64] = m.pcmOutput
	m.sq1.save(r[65:76])
	m.sq2.save(r[76:87])
	r[87] = byte(m.audioCounter)
	r[88] = byte(m.audioCounter >> 8)
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC5) Restore(s *State) {
	copy(m.wram[:], s.PRGRAM[:])
	copy(m.exRAM[:], s.CHRRAM[0:0x400])
	r := s.Regs[:]
	copy(m.prgBanks[:], r[0:5])
	for i := range m.chrBanks {
		m.chrBanks[i] = uint16(r[5+i*2]) | uint16(r[6+i*2])<<8
	}
	m.prgMode = r[29]
	m.chrMode = r[30]
	m.chrUpper = r[31]
	m.lastChrReg = uint16(r[32]) | uint16(r[33])<<8
	m.prgRAMProtect1 = r[34]
	m.prgRAMProtect2 = r[35]
	m.fillTile = r[36]
	m.fillColor = r[37]
	m.ntMapping = r[38]
	m.exRAMMode = r[39]
	m.splitDelimiter = r[40]
	m.splitScroll = r[41]
	m.splitBank = r[42]
	m.splitEnabled = r[43]&1 != 0
	m.splitRightSide = r[43]&2 != 0
	m.splitInRegion = r[43]&4 != 0
	m.irqEnabled = r[43]&8 != 0
	m.irqPending = r[43]&16 != 0
	m.needInFrame = r[43]&32 != 0
	m.ppuInFrame = r[43]&64 != 0
	m.splitTile = uint16(r[44]) | uint16(r[45])<<8
	m.splitTileNum = int32(uint32(r[46]) | uint32(r[47])<<8 | uint32(r[48])<<16 | uint32(r[49])<<24)
	m.mul1 = r[50]
	m.mul2 = r[51]
	m.exAttrLastNT = uint16(r[52]) | uint16(r[53])<<8
	m.exAttrCounter = int8(r[54])
	m.exAttrChrBank = r[55]
	m.irqTarget = r[56]
	m.scanlineCounter = r[57]
	m.ppuIdle = r[58]
	m.lastPpuReadAddr = uint16(r[59]) | uint16(r[60])<<8
	m.ntReadCounter = r[61]
	m.ppuCtrl = r[62]
	m.pcmReadMode = r[63]&1 != 0
	m.pcmIrqEnabled = r[63]&2 != 0
	m.pcmIrqPending = r[63]&4 != 0
	m.pcmOutput = r[64]
	m.sq1.restore(r[65:76])
	m.sq2.restore(r[76:87])
	m.audioCounter = int32(uint16(r[87]) | uint16(r[88])<<8)
}
