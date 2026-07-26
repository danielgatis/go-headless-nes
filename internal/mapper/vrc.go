package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// vrcIRQ is the IRQ counter shared by Konami's VRC3/4/6/7 boards: an
// 8-bit up-counter clocked either every CPU cycle (cycle mode) or once
// per scanline worth of CPU cycles via a 341/3 prescaler, reloading and
// raising the IRQ line on overflow.
type vrcIRQ struct {
	reload          byte
	counter         byte
	prescaler       int16
	enabled         bool
	enabledAfterAck bool
	cycleMode       bool
	line            bool
}

func (q *vrcIRQ) tick() {
	if !q.enabled {
		return
	}
	q.prescaler -= 3
	if q.cycleMode || q.prescaler <= 0 {
		if q.counter == 0xFF {
			q.counter = q.reload
			q.line = true
		} else {
			q.counter++
		}
		q.prescaler += 341
	}
}

func (q *vrcIRQ) setReloadLow(v byte)  { q.reload = (q.reload & 0xF0) | (v & 0x0F) }
func (q *vrcIRQ) setReloadHigh(v byte) { q.reload = (q.reload & 0x0F) | ((v & 0x0F) << 4) }

func (q *vrcIRQ) setControl(v byte) {
	q.enabledAfterAck = v&0x01 != 0
	q.enabled = v&0x02 != 0
	q.cycleMode = v&0x04 != 0
	if q.enabled {
		q.counter = q.reload
		q.prescaler = 341
	}
	q.line = false
}

func (q *vrcIRQ) ack() {
	q.enabled = q.enabledAfterAck
	q.line = false
}

// save/restore pack the IRQ unit into 7 bytes of a State.Regs slice.
func (q *vrcIRQ) save(r []byte) {
	r[0] = q.reload
	r[1] = q.counter
	r[2] = byte(q.prescaler)
	r[3] = byte(q.prescaler >> 8)
	r[4] = boolByte(q.enabled)
	r[5] = boolByte(q.enabledAfterAck)
	r[6] = boolByte(q.cycleMode) | boolByte(q.line)<<1
}

func (q *vrcIRQ) restore(r []byte) {
	q.reload = r[0]
	q.counter = r[1]
	q.prescaler = int16(uint16(r[2]) | uint16(r[3])<<8)
	q.enabled = r[4] != 0
	q.enabledAfterAck = r[5] != 0
	q.cycleMode = r[6]&1 != 0
	q.line = r[6]&2 != 0
}

// VRC1 (mapper 75): three switchable 8 KiB PRG banks plus a fixed last
// bank, two 4 KiB CHR banks with a 5th bit held in the $9000 register,
// and register-controlled mirroring (ignored on four-screen boards).
type VRC1 struct {
	base

	prgBanks   [3]byte
	chrBanks   [2]byte
	fourScreen bool
}

// NewVRC1 wires a Konami VRC1 board.
func NewVRC1(c *cartridge.Cartridge) *VRC1 {
	return &VRC1{base: makeBase(c), fourScreen: c.Mirroring == cartridge.FourScreen}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *VRC1) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		bank := m.prgBanks[(addr-0x8000)>>13]
		return m.win(m.prg, int(bank), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *VRC1) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		switch addr & 0xF000 {
		case 0x8000:
			m.prgBanks[0] = v
		case 0x9000:
			if !m.fourScreen {
				if v&0x01 != 0 {
					m.mirroring = cartridge.Horizontal
				} else {
					m.mirroring = cartridge.Vertical
				}
			}
			m.chrBanks[0] = (m.chrBanks[0] & 0x0F) | ((v & 0x02) << 3)
			m.chrBanks[1] = (m.chrBanks[1] & 0x0F) | ((v & 0x04) << 2)
		case 0xA000:
			m.prgBanks[1] = v
		case 0xC000:
			m.prgBanks[2] = v
		case 0xE000:
			m.chrBanks[0] = (m.chrBanks[0] & 0x10) | (v & 0x0F)
		case 0xF000:
			m.chrBanks[1] = (m.chrBanks[1] & 0x10) | (v & 0x0F)
		}
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *VRC1) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.chrBanks[addr>>12]), 0x1000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *VRC1) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.chrBanks[addr>>12]), 0x1000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *VRC1) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:3], m.prgBanks[:])
	copy(s.Regs[3:5], m.chrBanks[:])
	s.Regs[5] = byte(m.mirroring)
}

// Restore loads the board's mapper-specific state from s.
func (m *VRC1) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.prgBanks[:], s.Regs[0:3])
	copy(m.chrBanks[:], s.Regs[3:5])
	m.mirroring = cartridge.Mirroring(s.Regs[5])
}

// VRC3 (mapper 73, Salamander): one switchable 16 KiB PRG bank, CHR RAM,
// and a 16-bit CPU-cycle IRQ counter with an 8-bit "small counter" mode.
type VRC3 struct {
	base

	prgBank byte

	irqEnableOnAck bool
	irqEnabled     bool
	smallCounter   bool
	irqReload      uint16
	irqCounter     uint16
	irqLine        bool
}

// NewVRC3 wires a Konami VRC3 board.
func NewVRC3(c *cartridge.Cartridge) *VRC3 {
	return &VRC3{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *VRC3) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prgBank), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *VRC3) WritePRG(addr uint16, v byte) {
	switch {
	case addr >= 0x8000:
		switch addr & 0xF000 {
		case 0x8000:
			m.irqReload = (m.irqReload & 0xFFF0) | uint16(v&0x0F)
		case 0x9000:
			m.irqReload = (m.irqReload & 0xFF0F) | uint16(v&0x0F)<<4
		case 0xA000:
			m.irqReload = (m.irqReload & 0xF0FF) | uint16(v&0x0F)<<8
		case 0xB000:
			m.irqReload = (m.irqReload & 0x0FFF) | uint16(v&0x0F)<<12
		case 0xC000:
			m.irqEnabled = v&0x02 != 0
			if m.irqEnabled {
				m.irqCounter = m.irqReload
			}
			m.smallCounter = v&0x04 != 0
			m.irqEnableOnAck = v&0x01 != 0
			m.irqLine = false
		case 0xD000:
			m.irqLine = false
			m.irqEnabled = m.irqEnableOnAck
		case 0xF000:
			m.prgBank = v & 0x07
		}
	case addr >= 0x6000:
		m.writePRGRAM(addr, v)
	}
}

// Tick clocks the IRQ counter every CPU cycle: in small-counter mode
// only the low byte counts and reloads on overflow; otherwise the full
// 16-bit counter does.
func (m *VRC3) Tick() {
	if !m.irqEnabled {
		return
	}
	if m.smallCounter {
		small := byte(m.irqCounter) + 1
		if small == 0 {
			small = byte(m.irqReload)
			m.irqLine = true
		}
		m.irqCounter = (m.irqCounter & 0xFF00) | uint16(small)
	} else {
		m.irqCounter++
		if m.irqCounter == 0 {
			m.irqCounter = m.irqReload
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *VRC3) IRQ() bool { return m.irqLine }

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *VRC3) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *VRC3) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *VRC3) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.prgBank
	s.Regs[1] = byte(m.irqReload)
	s.Regs[2] = byte(m.irqReload >> 8)
	s.Regs[3] = byte(m.irqCounter)
	s.Regs[4] = byte(m.irqCounter >> 8)
	s.Regs[5] = boolByte(m.irqEnabled) | boolByte(m.irqEnableOnAck)<<1 |
		boolByte(m.smallCounter)<<2 | boolByte(m.irqLine)<<3
}

// Restore loads the board's mapper-specific state from s.
func (m *VRC3) Restore(s *State) {
	m.restoreRAM(s)
	m.prgBank = s.Regs[0]
	m.irqReload = uint16(s.Regs[1]) | uint16(s.Regs[2])<<8
	m.irqCounter = uint16(s.Regs[3]) | uint16(s.Regs[4])<<8
	m.irqEnabled = s.Regs[5]&1 != 0
	m.irqEnableOnAck = s.Regs[5]&2 != 0
	m.smallCounter = s.Regs[5]&4 != 0
	m.irqLine = s.Regs[5]&8 != 0
}
