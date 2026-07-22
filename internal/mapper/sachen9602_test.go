package mapper

import "testing"

func TestSachen9602ChrRAM(t *testing.T) {
	m := NewSachen9602(cart(513, 32, 0))
	m.WriteCHR(0x0000, 0x77)
	if got := m.ReadCHR(0x0000); got != 0x77 {
		t.Errorf("CHR RAM = %#x, want 0x77", got)
	}
	// Fixed PRG windows: $C000 -> $3E, $E000 -> $3F.
	if got := m.ReadPRG(0xC000); got != 31 {
		t.Errorf("$C000 bank = %d, want 31", got)
	}
	if got := m.ReadPRG(0xE000); got != 31 {
		t.Errorf("$E000 bank = %d, want 31", got)
	}
}
