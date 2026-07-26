package mapper

import "testing"

func TestUxROMBankMap(t *testing.T) {
	c := cart(2, 8, 0) // 8 x 16 KiB PRG, CHR RAM
	m := NewUxROM(c)

	// Select 16 KiB bank 3 at $8000 (bus-conflict-safe write).
	c.PRG[0x0200] = 0xFF
	m.WritePRG(0x8200, 3)

	// $8000/$A000 are the two 8 KiB halves of 16 KiB bank 3 (6, 7);
	// $C000/$E000 are the halves of the hardwired last bank 7 (14, 15).
	if got := m.PRGBankMap(); got != [4]int{6, 7, 14, 15} {
		t.Errorf("PRGBankMap = %v, want [6 7 14 15]", got)
	}
	if got := m.CHRBankMap(); got != [8]int{0, 1, 2, 3, 4, 5, 6, 7} {
		t.Errorf("CHRBankMap = %v, want linear (CHR RAM)", got)
	}
}

func TestCNROMBankMap(t *testing.T) {
	c := cart(3, 1, 4) // 1 x 16 KiB PRG, 4 x 8 KiB CHR
	m := NewCNROM(c)

	c.PRG[0x0100] = 0xFF
	m.WritePRG(0x8100, 2) // select 8 KiB CHR bank 2

	// 8 KiB CHR bank 2 spans 1 KiB banks 16..23.
	if got := m.CHRBankMap(); got != [8]int{16, 17, 18, 19, 20, 21, 22, 23} {
		t.Errorf("CHRBankMap = %v, want 16..23", got)
	}
	// PRG is fixed; a 16 KiB ROM mirrors across the 32 KiB window.
	if got := m.PRGBankMap(); got != [4]int{0, 1, 0, 1} {
		t.Errorf("PRGBankMap = %v, want [0 1 0 1]", got)
	}
}
