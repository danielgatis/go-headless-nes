package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// VRC6 (mappers 24 and 26): a 16 KiB + 8 KiB PRG window pair, eight
// 1 KiB CHR banks with three grouping modes, register-controlled
// mirroring including per-table CIRAM selection and CHR-ROM nametables,
// and the shared VRC IRQ. Mapper 26 swaps the two register-select
// address lines. The board's audio (two pulses + saw) is mixed into the
// APU output.
type VRC6 struct {
	base

	swapLines bool // mapper 26 (VRC6b)
	irq       vrcIRQ

	pulse1    vrc6Pulse
	pulse2    vrc6Pulse
	saw       vrc6Saw
	haltAudio bool

	bankingMode byte
	chrRegs     [8]byte
	prg16       byte // 16 KiB bank at $8000 (in 8 KiB units, low bit forced 0)
	prg8        byte // 8 KiB bank at $C000

	// Derived nametable state, recomputed from the registers.
	ntFromCHR bool
	ntPage    [4]byte // CIRAM page per table (when !ntFromCHR)
	ntCHR     [4]byte // CHR 1 KiB page per table (when ntFromCHR)
	chrPages  [8]byte // resolved CHR page per 1 KiB slot
}

// NewVRC6 wires a Konami VRC6 board.
func NewVRC6(c *cartridge.Cartridge) *VRC6 {
	m := &VRC6{base: makeBase(c), swapLines: c.MapperID == 26}
	m.updateBanking()
	return m
}

func (m *VRC6) ramEnabled() bool { return m.bankingMode&0x80 != 0 }

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *VRC6) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.win(m.prg, int(m.prg8), 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prg16)|int((addr>>13)&1), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if !m.ramEnabled() {
			return m.openBus()
		}
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *VRC6) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		if addr >= 0x6000 && m.ramEnabled() {
			m.writePRGRAM(addr, v)
		}
		return
	}

	if m.swapLines {
		addr = (addr & 0xFFFC) | ((addr & 0x01) << 1) | ((addr & 0x02) >> 1)
	}

	switch addr & 0xF003 {
	case 0x8000, 0x8001, 0x8002, 0x8003:
		m.prg16 = (v & 0x0F) << 1
	case 0x9000, 0x9001, 0x9002:
		m.pulse1.writeReg(addr, v)
	case 0x9003:
		m.haltAudio = v&0x01 != 0
		var shift byte
		switch {
		case v&0x04 != 0:
			shift = 8
		case v&0x02 != 0:
			shift = 4
		}
		m.pulse1.FreqShift = shift
		m.pulse2.FreqShift = shift
		m.saw.FreqShift = shift
	case 0xA000, 0xA001, 0xA002:
		m.pulse2.writeReg(addr, v)
	case 0xB000, 0xB001, 0xB002:
		m.saw.writeReg(addr, v)
	case 0xB003:
		m.bankingMode = v
		m.updateBanking()
	case 0xC000, 0xC001, 0xC002, 0xC003:
		m.prg8 = v & 0x1F
	case 0xD000, 0xD001, 0xD002, 0xD003:
		m.chrRegs[addr&0x03] = v
		m.updateBanking()
	case 0xE000, 0xE001, 0xE002, 0xE003:
		m.chrRegs[4+(addr&0x03)] = v
		m.updateBanking()
	case 0xF000:
		m.irq.reload = v
	case 0xF001:
		m.irq.setControl(v)
	case 0xF002:
		m.irq.ack()
	}
}

// updateBanking recomputes the CHR slot pages and the nametable routing
// from the banking-mode register.
func (m *VRC6) updateBanking() {
	mask, orMask := byte(0xFF), byte(0)
	if m.bankingMode&0x20 != 0 {
		mask, orMask = 0xFE, 1
	}

	r := &m.chrRegs
	switch m.bankingMode & 0x03 {
	case 0:
		m.chrPages = *r
	case 1:
		m.chrPages = [8]byte{
			r[0] & mask, r[0]&mask | orMask,
			r[1] & mask, r[1]&mask | orMask,
			r[2] & mask, r[2]&mask | orMask,
			r[3] & mask, r[3]&mask | orMask,
		}
	default: // 2 and 3
		m.chrPages = [8]byte{
			r[0], r[1], r[2], r[3],
			r[4] & mask, r[4]&mask | orMask,
			r[5] & mask, r[5]&mask | orMask,
		}
	}

	m.ntFromCHR = m.bankingMode&0x10 != 0
	if m.ntFromCHR {
		switch m.bankingMode & 0x2F {
		case 0x20, 0x27:
			m.ntCHR = [4]byte{r[6] & 0xFE, r[6]&0xFE | 1, r[7] & 0xFE, r[7]&0xFE | 1}
		case 0x23, 0x24:
			m.ntCHR = [4]byte{r[6] & 0xFE, r[7] & 0xFE, r[6]&0xFE | 1, r[7]&0xFE | 1}
		case 0x28, 0x2F:
			m.ntCHR = [4]byte{r[6] & 0xFE, r[6] & 0xFE, r[7] & 0xFE, r[7] & 0xFE}
		case 0x2B, 0x2C:
			m.ntCHR = [4]byte{r[6]&0xFE | 1, r[7]&0xFE | 1, r[6]&0xFE | 1, r[7]&0xFE | 1}
		default:
			switch m.bankingMode & 0x07 {
			case 0, 6, 7:
				m.ntCHR = [4]byte{r[6], r[6], r[7], r[7]}
			case 1, 5:
				m.ntCHR = [4]byte{r[4], r[5], r[6], r[7]}
			default: // 2, 3, 4
				m.ntCHR = [4]byte{r[6], r[7], r[6], r[7]}
			}
		}
		return
	}

	switch m.bankingMode & 0x2F {
	case 0x20, 0x27:
		m.ntPage = [4]byte{0, 1, 0, 1} // vertical
	case 0x23, 0x24:
		m.ntPage = [4]byte{0, 0, 1, 1} // horizontal
	case 0x28, 0x2F:
		m.ntPage = [4]byte{0, 0, 0, 0} // screen A
	case 0x2B, 0x2C:
		m.ntPage = [4]byte{1, 1, 1, 1} // screen B
	default:
		switch m.bankingMode & 0x07 {
		case 0, 6, 7:
			m.ntPage = [4]byte{r[6] & 1, r[6] & 1, r[7] & 1, r[7] & 1}
		case 1, 5:
			m.ntPage = [4]byte{r[4] & 1, r[5] & 1, r[6] & 1, r[7] & 1}
		default: // 2, 3, 4
			m.ntPage = [4]byte{r[6] & 1, r[7] & 1, r[6] & 1, r[7] & 1}
		}
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *VRC6) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrPages[addr>>10]), 0x400, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *VRC6) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrPages[addr>>10]), 0x400, addr, v)
}

// NametablePage implements per-table CIRAM selection.
func (m *VRC6) NametablePage(table byte) byte { return m.ntPage[table&3] }

// ReadNT / WriteNT serve nametables from CHR when the banking mode maps
// them there.
func (m *VRC6) ReadNT(addr uint16) (byte, bool) {
	if !m.ntFromCHR {
		return 0, false
	}
	table := (addr >> 10) & 3
	return m.chrRead(int(m.ntCHR[table]), 0x400, addr), true
}

// WriteNT handles a nametable write the board intercepts.
func (m *VRC6) WriteNT(addr uint16, v byte) bool {
	if !m.ntFromCHR {
		return false
	}
	table := (addr >> 10) & 3
	m.chrWrite(int(m.ntCHR[table]), 0x400, addr, v)
	return true
}

// Tick advances the board by one cycle.
func (m *VRC6) Tick() {
	m.irq.tick()
	if !m.haltAudio {
		m.pulse1.clock()
		m.pulse2.clock()
		m.saw.clock()
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *VRC6) IRQ() bool { return m.irq.line }

// AudioLevel mixes the two pulses and the sawtooth.
func (m *VRC6) AudioLevel() float32 {
	return expPulseStep * float32(int(m.pulse1.volume())+int(m.pulse2.volume())+int(m.saw.volume()))
}

// Save writes the board's mapper-specific state into s.
func (m *VRC6) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.bankingMode
	copy(s.Regs[1:9], m.chrRegs[:])
	s.Regs[9] = m.prg16
	s.Regs[10] = m.prg8
	m.irq.save(s.Regs[11:18])
	s.Regs[18] = boolByte(m.haltAudio)
	m.pulse1.save(s.Regs[19:27])
	m.pulse2.save(s.Regs[27:35])
	m.saw.save(s.Regs[35:43])
}

// Restore loads the board's mapper-specific state from s.
func (m *VRC6) Restore(s *State) {
	m.restoreRAM(s)
	m.bankingMode = s.Regs[0]
	copy(m.chrRegs[:], s.Regs[1:9])
	m.prg16 = s.Regs[9]
	m.prg8 = s.Regs[10]
	m.irq.restore(s.Regs[11:18])
	m.haltAudio = s.Regs[18] != 0
	m.pulse1.restore(s.Regs[19:27])
	m.pulse2.restore(s.Regs[27:35])
	m.saw.restore(s.Regs[35:43])
	m.updateBanking()
}
