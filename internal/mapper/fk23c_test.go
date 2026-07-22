package mapper

import "testing"

func TestFk23CPrgModes(t *testing.T) {
	m := NewFk23C(cart(176, 32, 16))
	// Power-on: MMC3 layout, fixed banks at the top of the ROM.
	if got := m.ReadPRG(0xE000); got != 31 {
		t.Errorf("fixed bank = %d, want 31 (last)", got)
	}
	// 16 KiB whole-window mode from the outer registers.
	m.WritePRG(0x5010, 0x03)
	m.WritePRG(0x5011, 0x05)
	if got := m.ReadPRG(0x8000); got != 5 {
		t.Errorf("16K window = %d, want 5", got)
	}
	if got := m.ReadPRG(0xC000); got != 5 {
		t.Errorf("16K window mirror = %d, want 5", got)
	}
}
