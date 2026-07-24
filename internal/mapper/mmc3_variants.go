package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// TxSROM (mapper 118): an MMC3 whose CHR registers' top bit drives the
// nametable selection (CHR A17 is wired to CIRAM A10); the $A000
// mirroring register is disconnected.
type TxSROM struct {
	MMC3

	ntPages [4]byte
}

// NewTxSROM wires a TKSROM/TLSROM board.
func NewTxSROM(c *cartridge.Cartridge) *TxSROM {
	return &TxSROM{MMC3: *NewMMC3(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *TxSROM) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 && addr&0xE001 == 0x8001 {
		nt := v >> 7
		if m.bankSelect&0x80 == 0 {
			switch m.bankSelect & 0x07 {
			case 0:
				m.ntPages[0], m.ntPages[1] = nt, nt
			case 1:
				m.ntPages[2], m.ntPages[3] = nt, nt
			}
		} else {
			switch m.bankSelect & 0x07 {
			case 2:
				m.ntPages[0] = nt
			case 3:
				m.ntPages[1] = nt
			case 4:
				m.ntPages[2] = nt
			case 5:
				m.ntPages[3] = nt
			}
		}
	}
	m.MMC3.WritePRG(addr, v)
}

// NametablePage overrides the MMC3 mirroring register.
func (m *TxSROM) NametablePage(table byte) byte { return m.ntPages[table&3] }

// Save writes the board's mapper-specific state into s.
func (m *TxSROM) Save(s *State) {
	m.MMC3.Save(s)
	copy(s.Regs[17:21], m.ntPages[:])
}

// Restore loads the board's mapper-specific state from s.
func (m *TxSROM) Restore(s *State) {
	m.MMC3.Restore(s)
	copy(m.ntPages[:], s.Regs[17:21])
}

// TQROM (mapper 119): an MMC3 that mixes CHR ROM and 8 KiB of CHR RAM, // bit 6 of a CHR bank register selects RAM for that window.
type TQROM struct {
	MMC3
}

// NewTQROM wires a TQROM board.
func NewTQROM(c *cartridge.Cartridge) *TQROM {
	return &TQROM{MMC3: *NewMMC3(c)}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *TQROM) ReadCHR(addr uint16) byte {
	p := m.chrPage1K(addr)
	if p&0x40 != 0 {
		return m.chrRAM[(p&0x07)<<10|int(addr&0x3FF)]
	}
	return window(m.chr, p&0x3F, 0x400)[addr&0x3FF]
}

// WriteCHR handles a write into the CHR address space.
func (m *TQROM) WriteCHR(addr uint16, v byte) {
	p := m.chrPage1K(addr)
	if p&0x40 != 0 {
		m.chrRAM[(p&0x07)<<10|int(addr&0x3FF)] = v
	}
}

// DxROM76 (mapper 76, Namco 108 with 2 KiB CHR): registers 2-5 bank
// four 2 KiB CHR windows.
type DxROM76 struct {
	DxROM
}

// NewDxROM76 wires the board.
func NewDxROM76(c *cartridge.Cartridge) *DxROM76 {
	return &DxROM76{DxROM: *NewDxROM(c)}
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *DxROM76) ReadCHR(addr uint16) byte {
	return m.chrRead(int(m.banks[2+(addr>>11)]), 0x800, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *DxROM76) WriteCHR(addr uint16, v byte) {
	m.chrWrite(int(m.banks[2+(addr>>11)]), 0x800, addr, v)
}

// DxROM88 (mapper 88, Namco 108 with CHR A16 wired to A12): the 2 KiB
// windows address the first 64 KiB of CHR and the 1 KiB windows the
// second.
type DxROM88 struct {
	DxROM
}

// NewDxROM88 wires the board.
func NewDxROM88(c *cartridge.Cartridge) *DxROM88 {
	return &DxROM88{DxROM: *NewDxROM(c)}
}

func (m *DxROM88) chrWindow(addr uint16) (bank, size int) {
	if addr < 0x1000 {
		r := addr >> 11 & 1
		return int(m.banks[r]&0x3E) >> 1, 2048
	}
	return int(m.banks[2+(addr>>10)&3] | 0x40), 1024
}

// ReadCHR returns the byte the CHR address space maps at addr.
func (m *DxROM88) ReadCHR(addr uint16) byte {
	bank, size := m.chrWindow(addr)
	return m.chrRead(bank, size, addr)
}

// WriteCHR handles a write into the CHR address space.
func (m *DxROM88) WriteCHR(addr uint16, v byte) {
	bank, size := m.chrWindow(addr)
	m.chrWrite(bank, size, addr, v)
}

// DxROM95 (mapper 95, Namco 108 on TLSROM wiring): the top bit of CHR
// registers 0/1 selects the nametables.
type DxROM95 struct {
	DxROM
}

// NewDxROM95 wires the board.
func NewDxROM95(c *cartridge.Cartridge) *DxROM95 {
	return &DxROM95{DxROM: *NewDxROM(c)}
}

// NametablePage maps tables 0-1 from register 0 and 2-3 from register 1.
func (m *DxROM95) NametablePage(table byte) byte {
	if table < 2 {
		return (m.banks[0] >> 5) & 0x01
	}
	return (m.banks[1] >> 5) & 0x01
}

// DxROM154 (mapper 154, Namco 108 with one-screen mirroring in the bank
// select write, on the mapper-88 CHR wiring).
type DxROM154 struct {
	DxROM88
}

// NewDxROM154 wires the board.
func NewDxROM154(c *cartridge.Cartridge) *DxROM154 {
	return &DxROM154{DxROM88: *NewDxROM88(c)}
}

// WritePRG handles a CPU write into the PRG address space ($6000-$FFFF).
func (m *DxROM154) WritePRG(addr uint16, v byte) {
	if addr >= 0x8000 {
		if v&0x40 != 0 {
			m.mirroring = cartridge.SingleHigh
		} else {
			m.mirroring = cartridge.SingleLow
		}
	}
	m.DxROM88.WritePRG(addr, v)
}
