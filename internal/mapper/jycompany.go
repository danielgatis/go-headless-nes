package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// JYCompany (mappers 90, 209, 211): the J.Y. Company ASIC used by their
// pirate originals and ports (Aladdin, Tiny Toon Adventures 6, ...).
// PRG banks in 8/16/32 KiB modes (optionally bit-reversed, optionally
// echoed at $6000), CHR in 1/2/4/8 KiB modes with an outer block
// register, MMC2-style CHR latches (209), a hardware multiplier, and an
// IRQ counter clockable from four sources: CPU cycles, CPU writes, PPU
// A12 rises or PPU reads, all through a 3-bit or 8-bit prescaler.
// Mappers 209/211 can also drive the nametables from CHR ROM and pick
// CIRAM pages per table; 90 has the mirroring register only.
type JYCompany struct {
	base

	variant uint16 // 90, 209 or 211

	prgRegs  [4]byte
	chrLow   [8]byte
	chrHigh  [8]byte
	chrLatch [2]byte

	prgMode   byte
	prgAt6000 bool

	chrMode      byte
	chrBlockMode bool
	chrBlock     byte
	mirrorChr    bool

	mirrorReg    byte
	advancedNt   bool
	disableNtRAM bool
	ntRAMSelect  byte
	ntLow        [4]byte
	ntHigh       [4]byte

	irqEnabled   bool
	irqSource    byte // 0 CPU cycle, 1 PPU A12 rise, 2 PPU read, 3 CPU write
	irqDir       byte
	irqFunkyMode bool
	irqFunkyReg  byte
	irqSmallPre  bool
	irqPrescaler byte
	irqCounter   byte
	irqXor       byte
	irqAsserted  bool
	lastPpuAddr  uint16
	mul1, mul2   byte
	regRAM       byte

	isWriteCycle func() bool
}

// NewJYCompany wires the board for one of its three mapper numbers.
func NewJYCompany(c *cartridge.Cartridge) *JYCompany {
	m := &JYCompany{
		base:         makeBase(c),
		variant:      c.MapperID,
		isWriteCycle: func() bool { return false },
	}
	m.chrLatch[0] = 0
	m.chrLatch[1] = 4
	return m
}

// SetCPUWriteCheck installs the write-cycle probe (console wiring).
func (m *JYCompany) SetCPUWriteCheck(isWrite func() bool) { m.isWriteCycle = isWrite }

// invertPrgBits bit-reverses the low 7 bits of a PRG register, which the
// board does in 8 KiB mode 3.
func invertPrgBits(v byte, invert bool) byte {
	if !invert {
		return v
	}
	return v&0x01<<6 | v&0x02<<4 | v&0x04<<2 | v&0x10>>2 | v&0x20>>4 | v&0x40>>6
}

// prgReg returns PRG register i with the mode's bit inversion applied.
func (m *JYCompany) prgReg(i int) byte {
	return invertPrgBits(m.prgRegs[i], m.prgMode&0x03 == 0x03)
}

// prgBank resolves the 8 KiB PRG bank for a $8000-$FFFF window.
func (m *JYCompany) prgBank(addr uint16) int {
	slot := int(addr>>13) & 3
	switch m.prgMode & 0x03 {
	case 0: // 32 KiB
		base := 0x3C
		if m.prgMode&0x04 != 0 {
			base = int(m.prgReg(3))
		}
		return base + slot
	case 1: // 16 KiB
		if slot < 2 {
			return int(m.prgReg(1))<<1 + slot
		}
		base := 0x3E
		if m.prgMode&0x04 != 0 {
			base = int(m.prgReg(3))
		}
		return base + (slot - 2)
	default: // 8 KiB (mode 3 additionally bit-reverses the registers)
		if slot == 3 {
			if m.prgMode&0x04 != 0 {
				return int(m.prgReg(3))
			}
			return 0x3F
		}
		return int(m.prgReg(slot))
	}
}

// prgBank6000 resolves the ROM bank echoed at $6000-$7FFF.
func (m *JYCompany) prgBank6000() int {
	r := int(m.prgReg(3))
	switch m.prgMode & 0x03 {
	case 0:
		return r*4 + 3
	case 1:
		return r*2 + 1
	default:
		return r
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *JYCompany) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		return window(m.prg, m.prgBank(addr), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.prgAt6000 {
			return window(m.prg, m.prgBank6000(), 0x2000)[addr&0x1FFF]
		}
		return m.openBus()
	case addr >= 0x5000:
		switch addr & 0xF803 {
		case 0x5000:
			return 0 // DIP switches (none wired)
		case 0x5800:
			return byte(uint16(m.mul1) * uint16(m.mul2))
		case 0x5801:
			return byte(uint16(m.mul1) * uint16(m.mul2) >> 8)
		case 0x5803:
			return m.regRAM
		}
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space.
func (m *JYCompany) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		switch addr & 0xF803 {
		case 0x5800:
			m.mul1 = v
		case 0x5801:
			m.mul2 = v
		case 0x5803:
			m.regRAM = v
		}
		return
	}
	switch addr & 0xF007 {
	case 0x8000, 0x8001, 0x8002, 0x8003, 0x8004, 0x8005, 0x8006, 0x8007:
		m.prgRegs[addr&0x03] = v & 0x7F
	case 0x9000, 0x9001, 0x9002, 0x9003, 0x9004, 0x9005, 0x9006, 0x9007:
		m.chrLow[addr&0x07] = v
	case 0xA000, 0xA001, 0xA002, 0xA003, 0xA004, 0xA005, 0xA006, 0xA007:
		m.chrHigh[addr&0x07] = v
	case 0xB000, 0xB001, 0xB002, 0xB003:
		m.ntLow[addr&0x03] = v
	case 0xB004, 0xB005, 0xB006, 0xB007:
		m.ntHigh[addr&0x03] = v
	case 0xC000:
		if v&0x01 != 0 {
			m.irqEnabled = true
		} else {
			m.irqEnabled = false
			m.irqAsserted = false
		}
	case 0xC001:
		m.irqDir = v >> 6 & 0x03
		m.irqFunkyMode = v&0x08 != 0
		m.irqSmallPre = v>>2&0x01 != 0
		m.irqSource = v & 0x03
	case 0xC002:
		m.irqEnabled = false
		m.irqAsserted = false
	case 0xC003:
		m.irqEnabled = true
	case 0xC004:
		m.irqPrescaler = v ^ m.irqXor
	case 0xC005:
		m.irqCounter = v ^ m.irqXor
	case 0xC006:
		m.irqXor = v
	case 0xC007:
		m.irqFunkyReg = v
	case 0xD000:
		m.prgMode = v & 0x07
		m.chrMode = v >> 3 & 0x03
		m.advancedNt = v&0x20 != 0
		m.disableNtRAM = v&0x40 != 0
		m.prgAt6000 = v&0x80 != 0
	case 0xD001:
		m.mirrorReg = v & 0x03
	case 0xD002:
		m.ntRAMSelect = v & 0x80
	case 0xD003:
		m.mirrorChr = v&0x80 != 0
		m.chrBlockMode = v&0x20 == 0
		m.chrBlock = v&0x18>>2 | v&0x01
	}
}

// chrReg composes CHR register index (mirroring and block mode applied).
func (m *JYCompany) chrReg(index int) int {
	if m.chrMode >= 2 && m.mirrorChr && (index == 2 || index == 3) {
		index -= 2
	}
	if m.chrBlockMode {
		var mask, shift byte
		switch m.chrMode {
		case 1:
			mask, shift = 0x3F, 6
		case 2:
			mask, shift = 0x7F, 7
		case 3:
			mask, shift = 0xFF, 8
		default:
			mask, shift = 0x1F, 5
		}
		return int(m.chrLow[index]&mask) | int(m.chrBlock)<<shift
	}
	return int(m.chrLow[index]) | int(m.chrHigh[index])<<8
}

// chrPage resolves the 1 KiB CHR page for a PPU address.
func (m *JYCompany) chrPage(addr uint16) int {
	slot := int(addr>>10) & 7
	switch m.chrMode {
	case 0: // 8 KiB
		return m.chrReg(0)<<3 + slot
	case 1: // 4 KiB, via the MMC2-style latches
		return m.chrReg(int(m.chrLatch[slot>>2]))<<2 + slot&3
	case 2: // 2 KiB
		return m.chrReg(slot&^1)<<1 + slot&1
	default: // 1 KiB
		return m.chrReg(slot)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr; PPU reads
// are one of the IRQ clock sources.
func (m *JYCompany) ReadCHR(addr uint16) byte {
	if m.irqSource == 2 {
		m.tickIRQCounter()
	}
	return m.chrRead(m.chrPage(addr), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *JYCompany) WriteCHR(addr uint16, v byte) {
	m.chrWrite(m.chrPage(addr), 0x400, addr, v)
}

// advancedNtActive reports whether the per-table nametable registers are
// in charge: 211 behaves as though the enable bit were always set, 90 as
// though it never were.
func (m *JYCompany) advancedNtActive() bool {
	return (m.advancedNt || m.variant == 211) && m.variant != 90
}

// NametablePage picks the CIRAM page for each logical table.
func (m *JYCompany) NametablePage(table byte) byte {
	if m.advancedNtActive() {
		return m.ntLow[table&3] & 0x01
	}
	switch m.mirrorReg {
	case 0:
		return table & 1 // vertical
	case 1:
		return table >> 1 & 1 // horizontal
	case 2:
		return 0
	default:
		return 1
	}
}

// ReadNT serves nametable fetches from CHR ROM when the per-table
// registers point there (this affects reads only; writes always land in
// CIRAM). PPU reads clock the IRQ here too, since nametable fetches are
// bus reads like any other.
func (m *JYCompany) ReadNT(addr uint16) (byte, bool) {
	if m.irqSource == 2 {
		m.tickIRQCounter()
	}
	if !m.advancedNtActive() {
		return 0, false
	}
	i := addr >> 10 & 3
	if m.disableNtRAM || m.ntLow[i]&0x80 != m.ntRAMSelect&0x80 {
		page := int(m.ntLow[i]) | int(m.ntHigh[i])<<8
		off := page<<10 | int(addr&0x3FF)
		if m.chr != nil && off < len(m.chr) {
			return m.chr[off], true
		}
		return 0, true
	}
	return 0, false
}

// WriteNT handles a nametable write the board intercepts (never: the
// CHR-ROM mapping affects reads only).
func (m *JYCompany) WriteNT(uint16, byte) bool { return false }

// NotifyVramAddr watches the raw PPU bus for the A12-rise IRQ source and
// the mapper-209 CHR latch addresses.
func (m *JYCompany) NotifyVramAddr(addr uint16) {
	if m.irqSource == 1 && addr&0x1000 != 0 && m.lastPpuAddr&0x1000 == 0 {
		m.tickIRQCounter()
	}
	m.lastPpuAddr = addr

	if m.variant == 209 {
		switch addr & 0x2FF8 {
		case 0x0FD8, 0x0FE8:
			m.chrLatch[addr>>12] = byte(addr>>4) & (byte(addr>>10)&0x04 | 0x02)
		}
	}
}

// Tick clocks the CPU-cycle and CPU-write IRQ sources.
func (m *JYCompany) Tick() {
	if m.irqSource == 0 || (m.irqSource == 3 && m.isWriteCycle()) {
		m.tickIRQCounter()
	}
}

// tickIRQCounter advances the prescaler and, on its wrap, the counter;
// the counter's own wrap asserts the IRQ. Direction 0 counts nothing;
// the "funky" mode register is not modelled (as in the reference).
func (m *JYCompany) tickIRQCounter() {
	clock := false
	mask := byte(0xFF)
	if m.irqSmallPre {
		mask = 0x07
	}
	pre := m.irqPrescaler & mask
	switch m.irqDir {
	case 1:
		pre++
		clock = pre&mask == 0
	case 2:
		pre--
		clock = pre == 0
	}
	m.irqPrescaler = m.irqPrescaler&^mask | pre&mask
	if !clock {
		return
	}
	switch m.irqDir {
	case 1:
		m.irqCounter++
		if m.irqCounter == 0 && m.irqEnabled {
			m.irqAsserted = true
		}
	case 2:
		m.irqCounter--
		if m.irqCounter == 0xFF && m.irqEnabled {
			m.irqAsserted = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *JYCompany) IRQ() bool { return m.irqAsserted }

// Save writes the board's mapper-specific state into s.
func (m *JYCompany) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:4], m.prgRegs[:])
	copy(s.Regs[4:12], m.chrLow[:])
	copy(s.Regs[12:20], m.chrHigh[:])
	copy(s.Regs[20:24], m.ntLow[:])
	copy(s.Regs[24:28], m.ntHigh[:])
	s.Regs[28] = m.chrLatch[0]
	s.Regs[29] = m.chrLatch[1]
	s.Regs[30] = m.prgMode
	s.Regs[31] = boolByte(m.prgAt6000)
	s.Regs[32] = m.chrMode
	s.Regs[33] = boolByte(m.chrBlockMode)
	s.Regs[34] = m.chrBlock
	s.Regs[35] = boolByte(m.mirrorChr)
	s.Regs[36] = m.mirrorReg
	s.Regs[37] = boolByte(m.advancedNt)
	s.Regs[38] = boolByte(m.disableNtRAM)
	s.Regs[39] = m.ntRAMSelect
	s.Regs[40] = boolByte(m.irqEnabled)
	s.Regs[41] = m.irqSource
	s.Regs[42] = m.irqDir
	s.Regs[43] = boolByte(m.irqFunkyMode)
	s.Regs[44] = m.irqFunkyReg
	s.Regs[45] = boolByte(m.irqSmallPre)
	s.Regs[46] = m.irqPrescaler
	s.Regs[47] = m.irqCounter
	s.Regs[48] = m.irqXor
	s.Regs[49] = boolByte(m.irqAsserted)
	s.Regs[50] = byte(m.lastPpuAddr)
	s.Regs[51] = byte(m.lastPpuAddr >> 8)
	s.Regs[52] = m.mul1
	s.Regs[53] = m.mul2
	s.Regs[54] = m.regRAM
}

// Restore loads the board's mapper-specific state from s.
func (m *JYCompany) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgRegs[:], s.Regs[0:4])
	copy(m.chrLow[:], s.Regs[4:12])
	copy(m.chrHigh[:], s.Regs[12:20])
	copy(m.ntLow[:], s.Regs[20:24])
	copy(m.ntHigh[:], s.Regs[24:28])
	m.chrLatch[0] = s.Regs[28]
	m.chrLatch[1] = s.Regs[29]
	m.prgMode = s.Regs[30]
	m.prgAt6000 = s.Regs[31] != 0
	m.chrMode = s.Regs[32]
	m.chrBlockMode = s.Regs[33] != 0
	m.chrBlock = s.Regs[34]
	m.mirrorChr = s.Regs[35] != 0
	m.mirrorReg = s.Regs[36]
	m.advancedNt = s.Regs[37] != 0
	m.disableNtRAM = s.Regs[38] != 0
	m.ntRAMSelect = s.Regs[39]
	m.irqEnabled = s.Regs[40] != 0
	m.irqSource = s.Regs[41]
	m.irqDir = s.Regs[42]
	m.irqFunkyMode = s.Regs[43] != 0
	m.irqFunkyReg = s.Regs[44]
	m.irqSmallPre = s.Regs[45] != 0
	m.irqPrescaler = s.Regs[46]
	m.irqCounter = s.Regs[47]
	m.irqXor = s.Regs[48]
	m.irqAsserted = s.Regs[49] != 0
	m.lastPpuAddr = uint16(s.Regs[50]) | uint16(s.Regs[51])<<8
	m.mul1 = s.Regs[52]
	m.mul2 = s.Regs[53]
	m.regRAM = s.Regs[54]
}
