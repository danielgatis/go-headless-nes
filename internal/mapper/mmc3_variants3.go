package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// More MMC3-derived boards, ported from the reference emulator. Each embeds
// MMC3 and overrides the minimum wiring that differs. Extra state packs
// into s.Regs from index 17 up.

// MMC312 (mapper 12, Gouder clones): a register at $4020-$5FFF adds a
// CHR outer bank (bit 8) applied independently to the two 4 KiB halves
// (bit 0 -> $0000 half, bit 4 -> $1000 half).
type MMC312 struct {
	MMC3

	chrSelection byte
}

// NewMMC312 wires the board.
func NewMMC312(c *cartridge.Cartridge) *MMC312 {
	return &MMC312{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC312) WritePRG(addr uint16, v byte) {
	if addr >= 0x4020 && addr <= 0x5FFF {
		m.chrSelection = v
		return
	}
	m.MMC3.WritePRG(addr, v)
}

func (m *MMC312) chrPage(addr uint16) int {
	// The half is chosen after the MMC3 A12 swap, matching the reference's slot
	// numbering (slots 0-3 are the $0000 half, 4-7 the $1000 half).
	sel := addr >> 12 & 1
	if m.bankSelect&0x80 != 0 {
		sel ^= 1
	}
	page := m.chrPage1K(addr)
	if sel == 0 && m.chrSelection&0x01 != 0 {
		page |= 0x100
	} else if sel == 1 && m.chrSelection&0x10 != 0 {
		page |= 0x100
	}
	return page
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC312) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC312) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *MMC312) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.chrSelection }

// Restore loads the board's mapper-specific state from s.
func (m *MMC312) Restore(s *State) { m.MMC3.Restore(s); m.chrSelection = s.Regs[17] }

// MMC3182 (mapper 182, "Hosenkan"): the four MMC3 register pairs are
// wired to different addresses and the bank-select's target field is
// permuted. We translate each incoming write to the equivalent stock
// MMC3 register write.
type MMC3182 struct {
	MMC3
}

// NewMMC3182 wires the board.
func NewMMC3182(c *cartridge.Cartridge) *MMC3182 {
	return &MMC3182{MMC3: *NewMMC3(c)}
}

// reg182 permutes the low 3 bits of a bank-select write.
var reg182 = [8]byte{0, 3, 1, 5, 6, 7, 2, 4}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3182) WritePRG(addr uint16, v byte) {
	if addr < 0x8000 {
		m.MMC3.WritePRG(addr, v)
		return
	}
	switch addr & 0xE001 {
	case 0x8001:
		m.MMC3.WritePRG(0xA000, v)
	case 0xA000:
		m.MMC3.WritePRG(0x8000, v&0xF8|reg182[v&0x07])
	case 0xC000:
		m.MMC3.WritePRG(0x8001, v)
	case 0xC001:
		m.MMC3.WritePRG(0xC000, v)
		m.MMC3.WritePRG(0xC001, v)
	case 0xE000:
		m.MMC3.WritePRG(0xE000, v)
	case 0xE001:
		m.MMC3.WritePRG(0xE001, v)
	}
}

// MMC3197 (mapper 197): a CHR wiring where R0 banks a 4 KiB window and
// R2/R3 bank two 2 KiB windows (CHR mode 0), or R2 banks 4 KiB and R0
// mirrors into both 2 KiB windows (CHR mode 1).
type MMC3197 struct {
	MMC3
}

// NewMMC3197 wires the board.
func NewMMC3197(c *cartridge.Cartridge) *MMC3197 {
	return &MMC3197{MMC3: *NewMMC3(c)}
}

func (m *MMC3197) chrBankSize(addr uint16) (bank, size int) {
	// 197 fully overrides the CHR layout, so there is no A12 swap: slot
	// 0-3 ($0000-$0FFF) is one 4 KiB window, slots 4-5 and 6-7 are two
	// 2 KiB windows. mode0 (chr bit clear): 4 KiB from R0, high halves
	// from R2/R3. mode1: 4 KiB from R2, both high halves from R0.
	mode1 := m.bankSelect&0x80 != 0
	if addr < 0x1000 {
		r := m.banks[0]
		if mode1 {
			r = m.banks[2]
		}
		return int(r) >> 1, 0x1000 // (reg<<1) 1 KiB units / 4
	}
	r := m.banks[2+(addr>>11&1)] // R2 for $1000-$17FF, R3 for $1800-$1FFF
	if mode1 {
		r = m.banks[0]
	}
	return int(r), 0x800 // (reg<<1) / 2
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3197) ReadCHR(addr uint16) byte {
	bank, size := m.chrBankSize(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3197) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrBankSize(addr)
	m.chrWrite(bank, size, addr, v)
}

// MMC3205 (mapper 205, JC-016 multicart): a $6000-$7FFF register selects
// one of four 128 KiB blocks, masking and offsetting both PRG and CHR.
type MMC3205 struct {
	MMC3

	block byte
}

// NewMMC3205 wires the board.
func NewMMC3205(c *cartridge.Cartridge) *MMC3205 {
	return &MMC3205{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3205) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x8000 {
		m.block = v & 0x03
		return
	}
	m.MMC3.WritePRG(addr, v)
}

func (m *MMC3205) prgSelect(addr uint16) int {
	p := m.prgBank(addr)
	// Resolve negative (fixed) banks to a concrete index first.
	n := len(m.prg) / 0x2000
	if p < 0 {
		p += n
	}
	if m.block <= 1 {
		p &= 0x1F
	} else {
		p &= 0x0F
	}
	return p | int(m.block)*0x10
}

func (m *MMC3205) chrPage(addr uint16) int {
	p := m.chrPage1K(addr)
	if m.block >= 2 {
		p = p&0x7F | 0x100
	}
	if m.block == 1 || m.block == 3 {
		p |= 0x80
	}
	return p
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3205) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return m.win(m.prg, m.prgSelect(addr), 0x2000)[addr&0x1FFF]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3205) ReadCHR(addr uint16) byte { return m.chrRead(m.chrPage(addr), 0x400, addr) }

// WriteCHR handles a write into the CHR address space.
func (m *MMC3205) WriteCHR(addr uint16, v byte) { m.chrWrite(m.chrPage(addr), 0x400, addr, v) }

// Save writes the board's mapper-specific state into s.
func (m *MMC3205) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.block }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3205) Restore(s *State) { m.MMC3.Restore(s); m.block = s.Regs[17] }

// MMC3250 (mapper 250, Nitra): the MMC3 registers are addressed by the
// low address bits; the value written comes from the address itself. A
// write to $NnnV translates to a stock write of ($NnnV&0xFF) to register
// (addr&0xE000)|((addr&0x0400)>>10).
type MMC3250 struct {
	MMC3
}

// NewMMC3250 wires the board.
func NewMMC3250(c *cartridge.Cartridge) *MMC3250 {
	return &MMC3250{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3250) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		m.MMC3.WritePRG((addr&0xE000)|((addr&0x0400)>>10), byte(addr&0xFF))
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// MMC3196 (mapper 196, Master Fighter II clones): the $8000+ register
// address bits are scrambled, and a write to $6000-$6FFF switches to a
// fixed 32 KiB PRG bank selected by a merged nibble.
type MMC3196 struct {
	MMC3

	prgOverride bool
	prg32       byte
}

// NewMMC3196 wires the board.
func NewMMC3196(c *cartridge.Cartridge) *MMC3196 {
	return &MMC3196{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3196) WritePRG(addr uint16, v byte) {
	if addr >= 0x6000 && addr < 0x7000 {
		m.prgOverride = true
		m.prg32 = (v & 0x0F) | (v >> 4)
		return
	}
	if addr >= 0x8000 {
		// Descramble the register address: the true A0 is OR of the
		// scattered address bits, differing above/below $C000.
		if addr >= 0xC000 {
			addr = (addr & 0xFFFE) | ((addr >> 2) & 0x01) | ((addr >> 3) & 0x01)
		} else {
			addr = (addr & 0xFFFE) | ((addr >> 2) & 0x01) | ((addr >> 3) & 0x01) | ((addr >> 1) & 0x01)
		}
		m.MMC3.WritePRG(addr, v)
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3196) ReadPRG(addr uint16) byte {
	if addr >= 0x8000 && m.prgOverride {
		page := int(m.prg32)<<2 | int(addr>>13&3)
		return m.win(m.prg, page, 0x2000)[addr&0x1FFF]
	}
	return m.MMC3.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3196) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = boolByte(m.prgOverride)
	s.Regs[18] = m.prg32
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3196) Restore(s *State) {
	m.MMC3.Restore(s)
	m.prgOverride = s.Regs[17] != 0
	m.prg32 = s.Regs[18]
}

// MMC3245 (mapper 245, Waixing): bit 1 of register R0 supplies PRG A18
// (a 64 KiB outer bank) applied to R6/R7; CHR-RAM boards fix the two
// 4 KiB CHR windows by the CHR mode bit.
type MMC3245 struct {
	MMC3
}

// NewMMC3245 wires the board.
func NewMMC3245(c *cartridge.Cartridge) *MMC3245 {
	return &MMC3245{MMC3: *NewMMC3(c)}
}

// prgSlot mirrors the reference's PRG mapping: bit 1 of R0 supplies a 64 KiB
// outer bank (A18) forced onto R6/R7 and the two fixed slots; the fixed
// slots point at the top of the selected 512 KiB block when the ROM is at
// least 512 KiB, else at the very last bank.
func (m *MMC3245) prgSlot(slot int) int {
	or := 0
	if m.banks[0]&0x02 != 0 {
		or = 0x40
	}
	r6 := int(m.banks[6]&0x3F) | or
	r7 := int(m.banks[7]&0x3F) | or

	last := -1
	if len(m.prg)/0x2000 >= 0x40 {
		last = 0x3F | or
	}

	if m.bankSelect&0x40 == 0 {
		switch slot {
		case 0:
			return r6
		case 1:
			return r7
		case 2:
			return last - 1
		default:
			return last
		}
	}
	switch slot {
	case 0:
		return last - 1
	case 1:
		return r7
	case 2:
		return r6
	default:
		return last
	}
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3245) ReadPRG(addr uint16) byte {
	if addr < 0x8000 {
		return m.MMC3.ReadPRG(addr)
	}
	return m.win(m.prg, m.prgSlot(int(addr>>13&3)), 0x2000)[addr&0x1FFF]
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *MMC3245) ReadCHR(addr uint16) byte {
	if m.chr == nil {
		// CHR RAM: two fixed 4 KiB windows swapped by the CHR mode bit.
		hi := addr >= 0x1000
		if m.bankSelect&0x80 != 0 {
			hi = !hi
		}
		base := 0
		if hi {
			base = 0x1000
		}
		return m.chrRAM[base|int(addr&0x0FFF)]
	}
	return m.MMC3.ReadCHR(addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *MMC3245) WriteCHR(addr uint16, v byte) {
	if m.chr == nil {
		hi := addr >= 0x1000
		if m.bankSelect&0x80 != 0 {
			hi = !hi
		}
		base := 0
		if hi {
			base = 0x1000
		}
		m.chrRAM[base|int(addr&0x0FFF)] = v
		return
	}
	m.MMC3.WriteCHR(addr, v)
}

// MMC3254 (mapper 254, Pikachu Y2K): PRG-RAM reads are XOR-scrambled by
// a key set at $A001 until a write to $8000 unlocks them.
type MMC3254 struct {
	MMC3

	unlocked bool
	key      byte
}

// NewMMC3254 wires the board.
func NewMMC3254(c *cartridge.Cartridge) *MMC3254 {
	return &MMC3254{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3254) WritePRG(addr uint16, v byte) {
	switch addr {
	case 0x8000:
		m.unlocked = true
	case 0xA001:
		m.key = v
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3254) ReadPRG(addr uint16) byte {
	if addr >= 0x6000 && addr < 0x8000 {
		v := m.readPRGRAM(addr)
		if m.unlocked {
			return v
		}
		return v ^ m.key
	}
	return m.MMC3.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3254) Save(s *State) {
	m.MMC3.Save(s)
	s.Regs[17] = boolByte(m.unlocked)
	s.Regs[18] = m.key
}

// Restore loads the board's mapper-specific state from s.
func (m *MMC3254) Restore(s *State) {
	m.MMC3.Restore(s)
	m.unlocked = s.Regs[17] != 0
	m.key = s.Regs[18]
}

// MMC3238 (mapper 238): a $4020-$7FFF register whose value is read back
// through a small security LUT, plus stock MMC3 banking.
type MMC3238 struct {
	MMC3

	exReg byte
}

// NewMMC3238 wires the board.
func NewMMC3238(c *cartridge.Cartridge) *MMC3238 {
	return &MMC3238{MMC3: *NewMMC3(c)}
}

var securityLut238 = [4]byte{0x00, 0x02, 0x02, 0x03}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *MMC3238) WritePRG(addr uint16, v byte) {
	if addr >= 0x4020 && addr < 0x8000 {
		m.exReg = securityLut238[v&0x03]
		return
	}
	m.MMC3.WritePRG(addr, v)
}

// ReadPRG returns the byte the PRG address space maps at addr.
func (m *MMC3238) ReadPRG(addr uint16) byte {
	if addr >= 0x4020 && addr < 0x8000 {
		return m.exReg
	}
	return m.MMC3.ReadPRG(addr)
}

// Save writes the board's mapper-specific state into s.
func (m *MMC3238) Save(s *State) { m.MMC3.Save(s); s.Regs[17] = m.exReg }

// Restore loads the board's mapper-specific state from s.
func (m *MMC3238) Restore(s *State) { m.MMC3.Restore(s); m.exReg = s.Regs[17] }
