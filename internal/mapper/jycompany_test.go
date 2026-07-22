package mapper

import "testing"

func TestJYCompanyMultiplier(t *testing.T) {
	m := NewJYCompany(cart(90, 8, 16))
	m.WritePRG(0x5800, 13)
	m.WritePRG(0x5801, 7)
	if got := m.ReadPRG(0x5800); got != 91 {
		t.Errorf("product low = %d, want 91", got)
	}
	m.WritePRG(0x5800, 200)
	m.WritePRG(0x5801, 200)
	if lo, hi := m.ReadPRG(0x5800), m.ReadPRG(0x5801); lo != byte(40000&0xFF) || hi != byte(40000>>8) {
		t.Errorf("product = %d/%d, want %d/%d", lo, hi, 40000&0xFF, 40000>>8)
	}
	m.WritePRG(0x5803, 0x5A)
	if got := m.ReadPRG(0x5803); got != 0x5A {
		t.Errorf("RAM register = %#x, want 0x5A", got)
	}
}

func TestJYCompanyPrg16KMode(t *testing.T) {
	m := NewJYCompany(cart(90, 8, 16))
	m.WritePRG(0xD000, 0x01) // 16 KiB mode, last window fixed
	m.WritePRG(0x8001, 2)    // PRG register 1
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("switchable bank = %d, want 2", got)
	}
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Errorf("fixed bank = %d, want 7 (last)", got)
	}
}

func TestJYCompanyChrRomNametables(t *testing.T) {
	// Mapper 211 acts as though advanced NT control were always on.
	m := NewJYCompany(cart(211, 8, 16))
	m.WritePRG(0xD000, 0x40) // disable NT RAM: every table from CHR ROM
	m.WritePRG(0xB000, 9)    // NT 0 low: CHR page 9
	v, ok := m.ReadNT(0x2000)
	if !ok || v != 1 { // page 9 = offset $2400 = 8 KiB bank 1
		t.Errorf("NT read = %d,%v, want 1,true", v, ok)
	}
	// Mapper 90 never serves nametables from CHR.
	m90 := NewJYCompany(cart(90, 8, 16))
	m90.WritePRG(0xD000, 0x60)
	m90.WritePRG(0xB000, 9)
	if _, ok := m90.ReadNT(0x2000); ok {
		t.Error("mapper 90 must not serve CHR nametables")
	}
}
