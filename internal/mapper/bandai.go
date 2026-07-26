package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// I²C serial EEPROM modes shared by the X24C01/X24C02 models.
const (
	eeIdle byte = iota
	eeAddress
	eeRead
	eeWrite
	eeSendAck
	eeWaitAck
	eeChipAddress
)

// eeprom24 models the Bandai boards' X24C01 (128 bytes, is01) and
// X24C02 (256 bytes) serial EEPROMs bit by bit on the SCL/SDA lines.
type eeprom24 struct {
	is01 bool

	mode     byte
	nextMode byte
	chipAddr byte
	addr     byte
	data     byte
	counter  byte
	output   byte
	prevScl  byte
	prevSda  byte
	rom      [256]byte
}

func (e *eeprom24) read() byte        { return e.output }
func (e *eeprom24) writeScl(scl byte) { e.write(scl, e.prevSda) }
func (e *eeprom24) writeSda(sda byte) { e.write(e.prevScl, sda) }

// writeBit01/readBit01: the X24C01 shifts LSB first; the X24C02 MSB first.
func (e *eeprom24) writeBit(dest *byte, v byte) {
	if e.counter < 8 {
		if e.is01 {
			mask := byte(1) << e.counter
			*dest = (*dest &^ mask) | v<<e.counter
		} else {
			mask := byte(1) << (7 - e.counter)
			*dest = (*dest &^ mask) | v<<(7-e.counter)
		}
		e.counter++
	}
}

func (e *eeprom24) readBit() {
	if e.counter < 8 {
		var bit byte
		if e.is01 {
			bit = (e.data >> e.counter) & 1
		} else {
			bit = (e.data >> (7 - e.counter)) & 1
		}
		e.output = bit
		e.counter++
	}
}

func (e *eeprom24) write(scl, sda byte) {
	switch {
	case e.prevScl != 0 && scl != 0 && sda < e.prevSda:
		// START: SDA falls while SCL is stable high.
		if e.is01 {
			e.mode = eeAddress
			e.addr = 0
		} else {
			e.mode = eeChipAddress
		}
		e.counter = 0
		e.output = 1
	case e.prevScl != 0 && scl != 0 && sda > e.prevSda:
		// STOP: SDA rises while SCL is stable high.
		e.mode = eeIdle
		e.output = 1
	case scl > e.prevScl: // clock rise
		switch e.mode {
		case eeChipAddress:
			e.writeBit(&e.chipAddr, sda)
		case eeAddress:
			if e.is01 {
				if e.counter < 7 {
					e.writeBit(&e.addr, sda)
				} else if e.counter == 7 {
					e.counter = 8
					if sda != 0 {
						e.nextMode = eeRead
						e.data = e.rom[e.addr&0x7F]
					} else {
						e.nextMode = eeWrite
					}
				}
			} else {
				e.writeBit(&e.addr, sda)
			}
		case eeRead:
			e.readBit()
		case eeWrite:
			e.writeBit(&e.data, sda)
		case eeSendAck:
			e.output = 0
		case eeWaitAck:
			if e.is01 {
				if sda != 0 {
					e.nextMode = eeIdle
				}
			} else if sda == 0 {
				e.nextMode = eeRead
				e.data = e.rom[e.addr]
			}
		}
	case scl < e.prevScl: // clock fall
		switch e.mode {
		case eeChipAddress:
			if e.counter == 8 {
				if e.chipAddr&0xA0 == 0xA0 {
					e.mode = eeSendAck
					e.counter = 0
					e.output = 1
					if e.chipAddr&0x01 != 0 {
						e.nextMode = eeRead
						e.data = e.rom[e.addr]
					} else {
						e.nextMode = eeAddress
					}
				} else {
					e.mode = eeIdle
					e.counter = 0
					e.output = 1
				}
			}
		case eeAddress:
			if e.counter == 8 {
				e.mode = eeSendAck
				if !e.is01 {
					e.nextMode = eeWrite
				}
				e.counter = 0
				e.output = 1
			}
		case eeRead:
			if e.counter == 8 {
				e.mode = eeWaitAck
				if e.is01 {
					e.addr = (e.addr + 1) & 0x7F
				} else {
					e.addr++
				}
			}
		case eeWrite:
			if e.counter == 8 {
				e.mode = eeSendAck
				if e.is01 {
					e.nextMode = eeIdle
					e.rom[e.addr&0x7F] = e.data
					e.addr = (e.addr + 1) & 0x7F
				} else {
					e.nextMode = eeWrite
					e.rom[e.addr] = e.data
					e.addr++
				}
			}
		case eeSendAck:
			e.mode = e.nextMode
			e.counter = 0
			e.output = 1
		case eeWaitAck:
			if !e.is01 {
				e.mode = e.nextMode
				e.counter = 0
				e.output = 1
			}
		}
	}
	e.prevScl = scl
	e.prevSda = sda
}

// save packs the EEPROM control state into 9 bytes (the data array is
// stored separately).
func (e *eeprom24) save(r []byte) {
	r[0] = e.mode
	r[1] = e.nextMode
	r[2] = e.chipAddr
	r[3] = e.addr
	r[4] = e.data
	r[5] = e.counter
	r[6] = e.output
	r[7] = e.prevScl
	r[8] = e.prevSda
}

func (e *eeprom24) restore(r []byte) {
	e.mode = r[0]
	e.nextMode = r[1]
	e.chipAddr = r[2]
	e.addr = r[3]
	e.data = r[4]
	e.counter = r[5]
	e.output = r[6]
	e.prevScl = r[7]
	e.prevSda = r[8]
}

// BandaiFCG covers the Bandai FCG-1/2 and LZ93D50 boards (mappers 16,
// 153, 157 and 159): 16 KiB PRG banking, 1 KiB CHR banking, a 16-bit
// CPU-cycle IRQ down-counter (with the FCG-1/2 direct-counter variant),
// register mirroring and serial EEPROM saves. Mapper 153 (Famicom Jump
// II) turns the CHR registers into a PRG outer-bank select and has
// battery RAM at $6000. The Datach barcode reader (157) is not
// emulated; its games run without barcode input.
type BandaiFCG struct {
	base

	id  uint16
	sub byte

	chrRegs       [8]byte
	prgPage       byte
	prgBankSelect byte
	wramEnabled   bool

	irqEnabled bool
	irqCounter uint16
	irqReload  uint16
	irqLine    bool

	std   *eeprom24
	extra *eeprom24
}

// NewBandaiFCG wires the board family.
func NewBandaiFCG(c *cartridge.Cartridge) *BandaiFCG {
	m := &BandaiFCG{base: makeBase(c), id: c.MapperID, sub: c.Submapper}
	switch {
	case c.MapperID == 157:
		// All Datach games share an internal 256-byte EEPROM; some carry
		// an extra 128-byte one on the game cartridge.
		m.std = &eeprom24{}
		m.extra = &eeprom24{is01: true}
	case c.MapperID == 159:
		m.std = &eeprom24{is01: true}
	case c.MapperID == 16 && (c.Submapper == 0 || c.Submapper == 5):
		m.std = &eeprom24{}
	}
	return m
}

// usesOuterBank reports the mapper-153 (or oversized-PRG) mode where the
// CHR registers hold PRG A20.
func (m *BandaiFCG) usesOuterBank() bool {
	return m.id == 153 || len(m.prg)/0x4000 >= 0x20
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *BandaiFCG) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, int(0x0F|m.prgBankSelect), 0x4000)[addr&0x3FFF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.prgPage|m.prgBankSelect), 0x4000)[addr&0x3FFF]
	case addr >= 0x6000:
		if m.id == 153 {
			if m.wramEnabled {
				return m.readPRGRAM(addr)
			}
			return m.openBus()
		}
		// EEPROM/barcode data bit on D4, the rest open bus.
		var out byte
		switch {
		case m.std != nil && m.extra != nil:
			out = (m.std.read() & m.extra.read()) << 4
		case m.std != nil:
			out = m.std.read() << 4
		}
		return out | (m.openBus() & 0xE7)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *BandaiFCG) WritePRG(addr uint16, v byte) {
	if addr < 0x6000 {
		return
	}
	// Register mirroring: the FCG-1/2 (16.4) responds at $6000-$7FFF
	// only, the LZ93D50 revisions at $8000-$FFFF only; plain iNES
	// mapper 16 answers everywhere. Mapper 153 has real RAM at $6000.
	if addr < 0x8000 {
		if m.id == 153 {
			if m.wramEnabled {
				m.writePRGRAM(addr, v)
			}
			return
		}
		if m.id != 16 || m.sub == 5 {
			return
		}
	} else if m.id == 16 && m.sub == 4 {
		return
	}
	m.writeRegister(addr, v)
}

func (m *BandaiFCG) writeRegister(addr uint16, v byte) {
	switch addr & 0x0F {
	case 0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7:
		m.chrRegs[addr&0x07] = v
		if m.usesOuterBank() {
			m.prgBankSelect = 0
			for i := range m.chrRegs {
				m.prgBankSelect |= (m.chrRegs[i] & 0x01) << 4
			}
		}
		if m.extra != nil && m.id == 157 && addr&0x0F <= 3 {
			m.extra.writeScl((v >> 3) & 0x01)
		}
	case 0x8:
		m.prgPage = v & 0x0F
	case 0x9:
		switch v & 0x03 {
		case 0:
			m.mirroring = cartridge.Vertical
		case 1:
			m.mirroring = cartridge.Horizontal
		case 2:
			m.mirroring = cartridge.SingleLow
		case 3:
			m.mirroring = cartridge.SingleHigh
		}
	case 0xA:
		m.irqEnabled = v&0x01 != 0
		if m.id != 16 || m.sub != 4 {
			// The LZ93D50 copies the reload latch into the counter here.
			m.irqCounter = m.irqReload
		}
		m.irqLine = false
	case 0xB:
		if m.id != 16 || m.sub != 4 {
			m.irqReload = (m.irqReload & 0xFF00) | uint16(v)
		} else {
			m.irqCounter = (m.irqCounter & 0xFF00) | uint16(v)
		}
	case 0xC:
		if m.id != 16 || m.sub != 4 {
			m.irqReload = (m.irqReload & 0x00FF) | uint16(v)<<8
		} else {
			m.irqCounter = (m.irqCounter & 0x00FF) | uint16(v)<<8
		}
	case 0xD:
		if m.id == 153 {
			m.wramEnabled = v&0x20 != 0
		} else {
			scl := (v & 0x20) >> 5
			sda := (v & 0x40) >> 6
			if m.std != nil {
				m.std.write(scl, sda)
			}
			if m.extra != nil {
				m.extra.writeSda(sda)
			}
		}
	}
}

// Tick advances the board by one cycle.
func (m *BandaiFCG) Tick() {
	if m.irqEnabled {
		// The counter is checked before decrementing.
		if m.irqCounter == 0 {
			m.irqLine = true
		}
		m.irqCounter--
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *BandaiFCG) IRQ() bool { return m.irqLine }

// bankedCHR reports whether the CHR registers bank CHR ROM (they double
// as PRG outer-bank bits on mapper 153 and are unused on the Datach).
func (m *BandaiFCG) bankedCHR() bool {
	return m.chr != nil && m.id != 157 && !m.usesOuterBank()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *BandaiFCG) ReadCHR(addr uint16) byte {
	if m.bankedCHR() {
		return m.chrRead(int(m.chrRegs[addr>>10]), 0x400, addr)
	}
	return m.chrRead(0, 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *BandaiFCG) WriteCHR(addr uint16, v byte) {
	if m.bankedCHR() {
		m.chrWrite(int(m.chrRegs[addr>>10]), 0x400, addr, v)
		return
	}
	m.chrWrite(0, 0x2000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *BandaiFCG) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:8], m.chrRegs[:])
	s.Regs[8] = m.prgPage
	s.Regs[9] = m.prgBankSelect
	s.Regs[10] = boolByte(m.wramEnabled) | boolByte(m.irqEnabled)<<1 | boolByte(m.irqLine)<<2
	s.Regs[11] = byte(m.irqCounter)
	s.Regs[12] = byte(m.irqCounter >> 8)
	s.Regs[13] = byte(m.irqReload)
	s.Regs[14] = byte(m.irqReload >> 8)
	s.Regs[15] = byte(m.mirroring)
	// EEPROM contents live in the CHRRAM snapshot area: the EEPROM
	// boards (16/157/159) all carry CHR ROM, so that area is unused.
	if m.std != nil {
		m.std.save(s.Regs[16:25])
		copy(s.CHRRAM[0:256], m.std.rom[:])
	}
	if m.extra != nil {
		m.extra.save(s.Regs[25:34])
		copy(s.CHRRAM[256:384], m.extra.rom[:128])
	}
}

// Restore loads the board's mapper-specific state from s.
func (m *BandaiFCG) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.chrRegs[:], s.Regs[0:8])
	m.prgPage = s.Regs[8]
	m.prgBankSelect = s.Regs[9]
	m.wramEnabled = s.Regs[10]&1 != 0
	m.irqEnabled = s.Regs[10]&2 != 0
	m.irqLine = s.Regs[10]&4 != 0
	m.irqCounter = uint16(s.Regs[11]) | uint16(s.Regs[12])<<8
	m.irqReload = uint16(s.Regs[13]) | uint16(s.Regs[14])<<8
	m.mirroring = cartridge.Mirroring(s.Regs[15])
	if m.std != nil {
		m.std.restore(s.Regs[16:25])
		copy(m.std.rom[:], s.CHRRAM[0:256])
	}
	if m.extra != nil {
		m.extra.restore(s.Regs[25:34])
		copy(m.extra.rom[:128], s.CHRRAM[256:384])
	}
}
