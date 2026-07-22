package mapper

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

func nrom(t *testing.T, prgBanks, chrBanks int) *NROM {
	t.Helper()
	c := &cartridge.Cartridge{
		PRG:       make([]byte, prgBanks*16*1024),
		Mirroring: cartridge.Vertical,
	}
	for i := range c.PRG {
		c.PRG[i] = byte(i >> 8) // distinct per page, so banks differ
	}
	if chrBanks > 0 {
		c.CHR = make([]byte, chrBanks*8*1024)
		for i := range c.CHR {
			c.CHR[i] = byte(i ^ 0x5A)
		}
	}
	m, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	return m.(*NROM)
}

func TestNROM16KMirrorsUpperBank(t *testing.T) {
	m := nrom(t, 1, 1)
	if got, want := m.ReadPRG(0xC123), m.ReadPRG(0x8123); got != want {
		t.Errorf("$C123 = %02X, $8123 = %02X: 16K PRG must mirror", got, want)
	}
}

func TestNROM32KNoMirror(t *testing.T) {
	m := nrom(t, 2, 1)
	if m.ReadPRG(0x8001) == m.ReadPRG(0xC001) {
		t.Error("32K PRG must not mirror")
	}
}

func TestNROMCHRROMIgnoresWrites(t *testing.T) {
	m := nrom(t, 1, 1)
	before := m.ReadCHR(0x0100)
	m.WriteCHR(0x0100, ^before)
	if m.ReadCHR(0x0100) != before {
		t.Error("CHR ROM changed by write")
	}
}

func TestNROMCHRRAMWritable(t *testing.T) {
	m := nrom(t, 1, 0) // no CHR banks: board supplies CHR RAM
	m.WriteCHR(0x0100, 0x77)
	if got := m.ReadCHR(0x0100); got != 0x77 {
		t.Errorf("CHR RAM readback = %02X, want 77", got)
	}
}

func TestNROMMirroringFromCartridge(t *testing.T) {
	m := nrom(t, 1, 1)
	if m.Mirroring() != cartridge.Vertical {
		t.Errorf("mirroring = %v, want vertical", m.Mirroring())
	}
}

func TestNROMSaveRestore(t *testing.T) {
	m := nrom(t, 1, 0)
	m.WritePRG(0x6010, 0xAB)
	m.WriteCHR(0x0020, 0xCD)

	var s State
	m.Save(&s)

	m.WritePRG(0x6010, 0x00)
	m.WriteCHR(0x0020, 0x00)
	m.Restore(&s)

	if got := m.ReadPRG(0x6010); got != 0xAB {
		t.Errorf("PRG RAM after restore = %02X, want AB", got)
	}
	if got := m.ReadCHR(0x0020); got != 0xCD {
		t.Errorf("CHR RAM after restore = %02X, want CD", got)
	}
}

func TestUnsupportedMapperRejected(t *testing.T) {
	// 4095 is not a real iNES/NES 2.0 mapper number, so it must be rejected.
	_, err := New(&cartridge.Cartridge{PRG: make([]byte, 16*1024), MapperID: 4095})
	if err == nil {
		t.Error("expected error for unsupported mapper")
	}
}
