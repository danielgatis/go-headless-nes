package mapper

import "testing"

func TestDragonFighterProtection(t *testing.T) {
	m := NewDragonFighter(cart(292, 8, 16))
	m.SetCPUPeek(func(addr uint16) byte {
		if addr == 0xFF {
			return 0x12
		}
		return 0
	})
	_ = m.ReadPRG(0x6000) // latch exRegs[2] from RAM $FF
	// CHR $1000-$1FFF is a 4 KiB window straight from the latch.
	if got := m.ReadCHR(0x1000); got != 9 { // 4 KiB bank 18 = 8 KiB bank 9
		t.Errorf("CHR high window = %d, want 9", got)
	}
}
