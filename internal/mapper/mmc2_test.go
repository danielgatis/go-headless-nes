package mapper

import "testing"

func TestMMC2CHRLatch(t *testing.T) {
	c := chrCart(9, 4, 8) // CHR bytes encode 1K bank; 4K bank b starts at 1K bank 4b
	m := NewMMC2(c)
	m.WritePRG(0xB000, 1) // low window, FD -> 4K bank 1
	m.WritePRG(0xC000, 2) // low window, FE -> 4K bank 2

	// Power-on latch is FD.
	if got := m.ReadCHR(0x0000); got != 4 {
		t.Fatalf("CHR read = %d, want 4 (bank 1)", got)
	}
	// Reading the FE trigger tile flips the latch after the fetch.
	if got := m.ReadCHR(0x0FE8); got != 7 { // still bank 1 for this read
		t.Fatalf("trigger read = %d, want 7 (bank 1, last 1K)", got)
	}
	if got := m.ReadCHR(0x0000); got != 8 { // now bank 2
		t.Errorf("post-latch read = %d, want 8 (bank 2)", got)
	}
	// $0FE9 is NOT a trigger on MMC2's low window (exact-address match).
	m.ReadCHR(0x0FD8) // back to FD
	m.ReadCHR(0x0FE9)
	if got := m.ReadCHR(0x0000); got != 4 {
		t.Error("non-trigger address flipped the MMC2 low latch")
	}
}

func TestMMC4PRGAndLatchRange(t *testing.T) {
	c := chrCart(10, 4, 8)
	for i := range c.PRG {
		c.PRG[i] = byte(i / 0x4000)
	}
	m := NewMMC4(c)
	m.WritePRG(0xA000, 2)
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("$8000 bank = %d, want 2", got)
	}
	if got := m.ReadPRG(0xC000); got != 3 {
		t.Errorf("$C000 bank = %d, want 3 (fixed last)", got)
	}

	// MMC4's low window latches on the whole $0FE8-$0FEF row.
	m.WritePRG(0xB000, 1)
	m.WritePRG(0xC000, 2)
	m.ReadCHR(0x0FE9)
	if got := m.ReadCHR(0x0000); got != 8 {
		t.Errorf("MMC4 row trigger failed: read = %d, want 8", got)
	}
}
