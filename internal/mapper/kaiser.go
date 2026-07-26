package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// Kaiser boards, ported from the reference emulator.

// Kaiser202 (mappers 56, 142): three switchable 8 KiB PRG windows plus a
// fixed last bank, a CPU-cycle IRQ counter loaded a nibble at a time, and
// an optional PRG-ROM window at $6000. Mapper 56 adds its own CHR banking
// and mirroring; 142 leaves CHR fixed.
type Kaiser202 struct {
	base

	is56 bool

	irqReload  uint16
	irqCounter uint16
	irqControl byte
	irqLine    bool

	selectedReg byte
	prgRegs     [4]byte
	useROM      bool
	chrBanks    [8]byte
	mirror      cartridge.Mirroring
}

// NewKaiser202 wires the board.
func NewKaiser202(c *cartridge.Cartridge) *Kaiser202 {
	return &Kaiser202{base: makeBase(c), is56: c.MapperID == 56, mirror: c.Mirroring}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser202) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, -1, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		slot := (addr - 0x8000) >> 13
		return m.win(m.prg, int(m.prgRegs[slot]), 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		if m.useROM {
			return m.win(m.prg, int(m.prgRegs[3]), 0x2000)[addr&0x1FFF]
		}
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser202) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		if !m.useROM {
			m.writePRGRAM(addr, v)
		}
		return
	}
	if addr < 0x8000 {
		return
	}
	switch addr & 0xF000 {
	case 0x8000:
		m.irqReload = m.irqReload&0xFFF0 | uint16(v&0x0F)
	case 0x9000:
		m.irqReload = m.irqReload&0xFF0F | uint16(v&0x0F)<<4
	case 0xA000:
		m.irqReload = m.irqReload&0xF0FF | uint16(v&0x0F)<<8
	case 0xB000:
		m.irqReload = m.irqReload&0x0FFF | uint16(v&0x0F)<<12
	case 0xC000:
		m.irqControl = v
		if m.irqControl&0x02 != 0 {
			m.irqCounter = m.irqReload
		}
		m.irqLine = false
	case 0xD000:
		m.irqLine = false
	case 0xE000:
		m.selectedReg = (v & 0x07) - 1
	case 0xF000:
		switch m.selectedReg {
		case 0, 1, 2, 3:
			m.prgRegs[m.selectedReg] = m.prgRegs[m.selectedReg]&0x10 | v&0x0F
		case 4:
			m.useROM = v&0x04 != 0
		}
		if m.is56 {
			switch addr & 0xFC00 {
			case 0xF000:
				bank := addr & 0x03
				m.prgRegs[bank] = v&0x10 | m.prgRegs[bank]&0x0F
			case 0xF800:
				m.mirror = hvMirror(v&0x01 != 0)
			case 0xFC00:
				m.chrBanks[addr&0x07] = v
			}
		}
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser202) ReadCHR(addr uint16) byte {
	if m.is56 {
		return m.chrRead(int(m.chrBanks[addr>>10&7]), 0x400, addr)
	}
	return m.chrRead(0, 0x2000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser202) WriteCHR(addr uint16, v byte) {
	if m.is56 {
		m.chrWrite(int(m.chrBanks[addr>>10&7]), 0x400, addr, v)
		return
	}
	m.chrWrite(0, 0x2000, addr, v)
}

// Tick clocks the IRQ counter each CPU cycle when enabled; a wrap through
// 0xFFFF reloads and asserts the line.
func (m *Kaiser202) Tick() {
	if m.irqControl&0x02 == 0 {
		return
	}
	m.irqCounter++
	if m.irqCounter == 0xFFFF {
		m.irqCounter = m.irqReload
		m.irqControl &^= 0x02
		m.irqLine = true
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Kaiser202) IRQ() bool { return m.irqLine }

// Mirroring reports the board's current nametable mirroring.
func (m *Kaiser202) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser202) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.irqReload)
	s.Regs[1] = byte(m.irqReload >> 8)
	s.Regs[2] = byte(m.irqCounter)
	s.Regs[3] = byte(m.irqCounter >> 8)
	s.Regs[4] = m.irqControl
	s.Regs[5] = boolByte(m.irqLine)
	s.Regs[6] = m.selectedReg
	copy(s.Regs[7:11], m.prgRegs[:])
	s.Regs[11] = boolByte(m.useROM)
	copy(s.Regs[12:20], m.chrBanks[:])
	s.Regs[20] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser202) Restore(s *State) {
	m.restoreRAM(s)
	m.irqReload = uint16(s.Regs[0]) | uint16(s.Regs[1])<<8
	m.irqCounter = uint16(s.Regs[2]) | uint16(s.Regs[3])<<8
	m.irqControl = s.Regs[4]
	m.irqLine = s.Regs[5] != 0
	m.selectedReg = s.Regs[6]
	copy(m.prgRegs[:], s.Regs[7:11])
	m.useROM = s.Regs[11] != 0
	copy(m.chrBanks[:], s.Regs[12:20])
	m.mirror = cartridge.Mirroring(s.Regs[20])
}

// Kaiser7058 (mapper 171): two 4 KiB CHR windows selected by writes to
// $F000/$F080; PRG is a fixed 32 KiB.
type Kaiser7058 struct {
	base
	chrLo byte
	chrHi byte
}

// NewKaiser7058 wires the board.
func NewKaiser7058(c *cartridge.Cartridge) *Kaiser7058 {
	return &Kaiser7058{base: makeBase(c)}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7058) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, 0, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7058) WritePRG(addr uint16, v byte) {
	if addr >= 0xF000 {
		switch addr & 0xF080 {
		case 0xF000:
			m.chrLo = v
		case 0xF080:
			m.chrHi = v
		}
		return
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7058) ReadCHR(addr uint16) byte {
	if addr < 0x1000 {
		return m.chrRead(int(m.chrLo), 0x1000, addr)
	}
	return m.chrRead(int(m.chrHi), 0x1000, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7058) WriteCHR(addr uint16, v byte) {
	if addr < 0x1000 {
		m.chrWrite(int(m.chrLo), 0x1000, addr, v)
		return
	}
	m.chrWrite(int(m.chrHi), 0x1000, addr, v)
}

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7058) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.chrLo
	s.Regs[1] = m.chrHi
}

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7058) Restore(s *State) {
	m.restoreRAM(s)
	m.chrLo = s.Regs[0]
	m.chrHi = s.Regs[1]
}

// Kaiser7022 (mapper 175): a single register drives both PRG halves and
// the CHR bank; it latches on any read of $FFFC (a reset-vector copy
// protection trick), and mirroring toggles on a write to $8000.
type Kaiser7022 struct {
	base
	reg    byte
	mirror cartridge.Mirroring
}

// NewKaiser7022 wires the board.
func NewKaiser7022(c *cartridge.Cartridge) *Kaiser7022 {
	return &Kaiser7022{base: makeBase(c), mirror: c.Mirroring}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7022) ReadPRG(addr uint16) byte {
	if addr == 0xFFFC {
		// Reading the reset vector latches the bank register into both
		// 16 KiB PRG windows and the CHR bank.
		v := m.win(m.prg, int(m.reg), 0x4000)[addr&0x3FFF]
		return v
	}
	if addr >= 0x8000 {
		return m.win(m.prg, int(m.reg), 0x4000)[addr&0x3FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7022) WritePRG(addr uint16, v byte) {
	switch {
	case addr == 0x8000:
		m.mirror = hvMirror(v&0x04 == 0)
	case addr&0xF000 == 0xA000:
		m.reg = v & 0x0F
	case addr >= 0x6000 && addr < 0x8000:
		m.writePRGRAM(addr, v)
	}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7022) ReadCHR(addr uint16) byte { return m.chrRead(int(m.reg), 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7022) WriteCHR(addr uint16, v byte) { m.chrWrite(int(m.reg), 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Kaiser7022) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7022) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.reg
	s.Regs[1] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7022) Restore(s *State) {
	m.restoreRAM(s)
	m.reg = s.Regs[0]
	m.mirror = cartridge.Mirroring(s.Regs[1])
}

// Kaiser7012 (mapper 346): a fixed 32 KiB PRG board with a single bank
// register toggled by two magic addresses.
type Kaiser7012 struct {
	base
	prgBank int
}

// NewKaiser7012 wires the board.
func NewKaiser7012(c *cartridge.Cartridge) *Kaiser7012 {
	return &Kaiser7012{base: makeBase(c), prgBank: 1}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7012) WritePRG(addr uint16, v byte) {
	switch addr {
	case 0xE0A0:
		m.prgBank = 0
	case 0xEE36:
		m.prgBank = 1
	}
	if addr >= 0x6000 && addr < 0x8000 {
		m.writePRGRAM(addr, v)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7012) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 {
		return m.win(m.prg, m.prgBank, 0x8000)[addr&0x7FFF]
	}
	if addr >= 0x6000 {
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7012) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7012) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7012) Save(s *State) { m.saveRAM(s); s.Regs[0] = byte(m.prgBank) }

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7012) Restore(s *State) { m.restoreRAM(s); m.prgBank = int(s.Regs[0]) }

// Kaiser7031 (mapper 305): four 2 KiB PRG-ROM windows banked into
// $6000-$7FFF; $8000-$FFFF is fixed to the top 32 KiB in reverse order.
type Kaiser7031 struct {
	base
	regs [4]byte
}

// NewKaiser7031 wires the board.
func NewKaiser7031(c *cartridge.Cartridge) *Kaiser7031 {
	return &Kaiser7031{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7031) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.regs[(addr>>11)&0x03] = v
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7031) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		// 16 fixed 2 KiB windows in reverse (slot i -> bank 15-i).
		slot := int((addr - 0x8000) >> 11)
		return m.win(m.prg, 15-slot, 0x800)[addr&0x7FF]
	case addr >= 0x6000:
		win := int((addr - 0x6000) >> 11)
		return m.win(m.prg, int(m.regs[win]), 0x800)[addr&0x7FF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7031) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7031) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Kaiser7031) Mirroring() cartridge.Mirroring { return cartridge.Vertical }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7031) Save(s *State) { m.saveRAM(s); copy(s.Regs[0:4], m.regs[:]) }

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7031) Restore(s *State) { m.restoreRAM(s); copy(m.regs[:], s.Regs[0:4]) }

// Kaiser7016 (mapper 306): a $6000-$7FFF PRG-ROM window whose bank is set
// by a small address-decoded state machine; $8000+ is fixed to the top
// 32 KiB.
type Kaiser7016 struct {
	base
	prgReg int
}

// NewKaiser7016 wires the board.
func NewKaiser7016(c *cartridge.Cartridge) *Kaiser7016 {
	return &Kaiser7016{base: makeBase(c), prgReg: 8}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7016) WritePRG(addr uint16, _ byte) {
	if addr < 0x8000 {
		return
	}
	mode := addr&0x30 == 0x30
	switch addr & 0xD943 {
	case 0xD943:
		if mode {
			m.prgReg = 0x0B
		} else {
			m.prgReg = int(addr>>2) & 0x0F
		}
	case 0xD903:
		if mode {
			m.prgReg = 0x08 | int(addr>>2)&0x03
		} else {
			m.prgReg = 0x0B
		}
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7016) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0x8000:
		// Fixed top 32 KiB (banks 0x0C-0x0F in 8 KiB units).
		slot := int((addr - 0x8000) >> 13)
		return m.win(m.prg, 0x0C+slot, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.win(m.prg, m.prgReg, 0x2000)[addr&0x1FFF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7016) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7016) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7016) Save(s *State) { m.saveRAM(s); s.Regs[0] = byte(m.prgReg) }

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7016) Restore(s *State) { m.restoreRAM(s); m.prgReg = int(s.Regs[0]) }

// Kaiser7057 (mapper 302): eight 2 KiB PRG-ROM windows at $6000-$7FFF
// (four register-selected) plus four fixed 8 KiB top banks; PRG banks are
// loaded a nibble at a time.
type Kaiser7057 struct {
	base
	regs   [8]byte
	mirror cartridge.Mirroring
}

// NewKaiser7057 constructs the corresponding board.
func NewKaiser7057(c *cartridge.Cartridge) *Kaiser7057 {
	return &Kaiser7057{base: makeBase(c), mirror: cartridge.Vertical}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7057) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		return
	}
	low := addr&0x01 == 0
	upd := func(i int) {
		if low {
			m.regs[i] = m.regs[i]&0xF0 | v&0x0F
		} else {
			m.regs[i] = m.regs[i]&0x0F | v<<4
		}
	}
	switch addr & 0xF002 {
	case 0x8000, 0x8002, 0x9000, 0x9002:
		m.mirror = hvMirror(v&0x01 != 0)
	case 0xB000:
		upd(0)
	case 0xB002:
		upd(1)
	case 0xC000:
		upd(2)
	case 0xC002:
		upd(3)
	case 0xD000:
		upd(4)
	case 0xD002:
		upd(5)
	case 0xE000:
		upd(6)
	case 0xE002:
		upd(7)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7057) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, 0x3C+int((addr-0xE000)>>11), 0x800)[addr&0x7FF]
	case addr >= 0xC000:
		return m.win(m.prg, 0x38+int((addr-0xC000)>>11), 0x800)[addr&0x7FF]
	case addr >= 0xA000:
		return m.win(m.prg, 0x34+int((addr-0xA000)>>11), 0x800)[addr&0x7FF]
	case addr >= 0x8000:
		return m.win(m.prg, int(m.regs[(addr-0x8000)>>11&3]), 0x800)[addr&0x7FF]
	case addr >= 0x6000:
		return m.win(m.prg, int(m.regs[4+((addr-0x6000)>>11&3)]), 0x800)[addr&0x7FF]
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7057) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7057) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Kaiser7057) Mirroring() cartridge.Mirroring { return m.mirror }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7057) Save(s *State) {
	m.saveRAM(s)
	copy(s.Regs[0:8], m.regs[:])
	s.Regs[8] = byte(m.mirror)
}

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7057) Restore(s *State) {
	m.restoreRAM(s)
	copy(m.regs[:], s.Regs[0:8])
	m.mirror = cartridge.Mirroring(s.Regs[8])
}

// Kaiser7017 (mapper 303): a $4A00-region register banks a 16 KiB PRG
// window ($8000, $A000 fixed to bank 2); a down-counting cycle IRQ armed
// via $4020/$4021; mirroring at $4025.
type Kaiser7017 struct {
	base
	prgReg     int
	mirror     cartridge.Mirroring
	irqCounter uint16
	irqEnabled bool
	irqLine    bool
}

// NewKaiser7017 constructs the corresponding board.
func NewKaiser7017(c *cartridge.Cartridge) *Kaiser7017 {
	return &Kaiser7017{base: makeBase(c), mirror: cartridge.Vertical}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7017) WritePRG(addr uint16, v byte) {
	switch {
	case addr&0xFF00 == 0x4A00:
		m.prgReg = int(addr>>2)&0x03 | int(addr>>4)&0x04
	case addr == 0x4020:
		m.irqCounter = m.irqCounter&0xFF00 | uint16(v)
		m.irqLine = false
	case addr == 0x4021:
		m.irqCounter = m.irqCounter&0x00FF | uint16(v)<<8
		m.irqEnabled = true
		m.irqLine = false
	case addr == 0x4025:
		m.mirror = hvMirror(v>>3&0x01 == 0)
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7017) ReadPRG(addr uint16) byte {
	switch {
	case addr >= 0xC000:
		return m.win(m.prg, -1, 0x4000)[addr&0x3FFF]
	case addr >= 0xA000:
		return m.win(m.prg, 2, 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, m.prgReg, 0x2000)[addr&0x1FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7017) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7017) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// Mirroring reports the board's current nametable mirroring.
func (m *Kaiser7017) Mirroring() cartridge.Mirroring { return m.mirror }

// Tick advances the board by one cycle.
func (m *Kaiser7017) Tick() {
	if m.irqEnabled && m.irqCounter != 0 {
		m.irqCounter--
		if m.irqCounter == 0 {
			m.irqEnabled = false
			m.irqLine = true
		}
	}
}

// IRQ reports whether the board is asserting the IRQ line.
func (m *Kaiser7017) IRQ() bool { return m.irqLine }

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7017) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = byte(m.prgReg)
	s.Regs[1] = byte(m.mirror)
	s.Regs[2] = byte(m.irqCounter)
	s.Regs[3] = byte(m.irqCounter >> 8)
	s.Regs[4] = boolByte(m.irqEnabled)
	s.Regs[5] = boolByte(m.irqLine)
}

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7017) Restore(s *State) {
	m.restoreRAM(s)
	m.prgReg = int(s.Regs[0])
	m.mirror = cartridge.Mirroring(s.Regs[1])
	m.irqCounter = uint16(s.Regs[2]) | uint16(s.Regs[3])<<8
	m.irqEnabled = s.Regs[4] != 0
	m.irqLine = s.Regs[5] != 0
}

// Kaiser7037 (mapper 307): an index/data register file drives two
// switchable 16 KiB-worth PRG windows, a WRAM window and fixed banks, plus
// per-table nametable page selection (SetNametables). CHR is fixed 8 KiB.
type Kaiser7037 struct {
	base
	currentReg byte
	regs       [8]byte
}

// NewKaiser7037 constructs the corresponding board.
func NewKaiser7037(c *cartridge.Cartridge) *Kaiser7037 {
	return &Kaiser7037{base: makeBase(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *Kaiser7037) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x7000 {
		m.writePRGRAM(addr, v)
		return
	}
	switch addr & 0xE001 {
	case 0x8000:
		m.currentReg = v & 0x07
	case 0x8001:
		m.regs[m.currentReg] = v
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *Kaiser7037) ReadPRG(addr uint16) byte {
	// Layout (from the reference): $6000-$6FFF WRAM, $7000-$7FFF fixed bank 15,
	// $8000-$9FFF two 8 KiB banks from reg6, $A000-$BFFF fixed to bank -4,
	// $C000-$DFFF two 8 KiB banks from reg7, $E000-$FFFF fixed to bank -2.
	switch {
	case addr >= 0xE000:
		return m.win(m.prg, -2+int((addr-0xE000)>>13), 0x2000)[addr&0x1FFF]
	case addr >= 0xC000:
		return m.win(m.prg, (int(m.regs[7])<<1)+int((addr-0xC000)>>12), 0x1000)[addr&0x0FFF]
	case addr >= 0xA000:
		return m.win(m.prg, -4+int((addr-0xA000)>>13), 0x2000)[addr&0x1FFF]
	case addr >= 0x8000:
		return m.win(m.prg, (int(m.regs[6])<<1)+int((addr-0x8000)>>12), 0x1000)[addr&0x0FFF]
	case addr >= 0x7000:
		n := len(m.prg) / 0x1000
		return m.win(m.prg, n-1, 0x1000)[addr&0x0FFF]
	case addr >= 0x6000:
		return m.readPRGRAM(addr)
	}
	return m.openBus()
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *Kaiser7037) ReadCHR(addr uint16) byte { return m.chrRead(0, 0x2000, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *Kaiser7037) WriteCHR(addr uint16, v byte) { m.chrWrite(0, 0x2000, addr, v) }

// NametablePage selects the CIRAM page per table from regs 2-5.
func (m *Kaiser7037) NametablePage(table byte) byte {
	switch table & 3 {
	case 0:
		return m.regs[2] & 1
	case 1:
		return m.regs[4] & 1
	case 2:
		return m.regs[3] & 1
	default:
		return m.regs[5] & 1
	}
}

// Save writes the board's mapper-specific state into s.
func (m *Kaiser7037) Save(s *State) {
	m.saveRAM(s)
	s.Regs[0] = m.currentReg
	copy(s.Regs[1:9], m.regs[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *Kaiser7037) Restore(s *State) {
	m.restoreRAM(s)
	m.currentReg = s.Regs[0]
	copy(m.regs[:], s.Regs[1:9])
}
