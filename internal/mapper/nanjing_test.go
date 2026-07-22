package mapper

import "testing"

func TestNanjingBankingAndProtection(t *testing.T) {
	m := NewNanjing(cart(163, 8, 0))
	m.WritePRG(0x5000, 0x03)
	m.WritePRG(0x5200, 0x00)
	if got := m.ReadPRG(0x8000); got != 6 { // 32 KiB bank 3 = 16 KiB bank 6
		t.Errorf("PRG bank = %d, want 6", got)
	}
	// Protection: $5500 reads regs[3]|regs[0] while the trigger is set...
	m.WritePRG(0x5300, 0x40)
	if got := m.ReadPRG(0x5500); got != 0x43 {
		t.Errorf("$5500 = %#x, want 0x43", got)
	}
	// ...and 0 after $5101 falls from nonzero to zero.
	m.WritePRG(0x5101, 1)
	m.WritePRG(0x5101, 0)
	if got := m.ReadPRG(0x5500); got != 0 {
		t.Errorf("$5500 after toggle = %#x, want 0", got)
	}
}
