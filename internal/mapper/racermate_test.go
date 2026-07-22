package mapper

import "testing"

func TestRacermateIRQ(t *testing.T) {
	m := NewRacermate(cart(168, 8, 0))
	for range 65536 {
		m.Tick()
	}
	if !m.IRQ() {
		t.Error("IRQ not asserted after the counter wrapped")
	}
	m.WritePRG(0xC000, 0)
	if m.IRQ() {
		t.Error("IRQ not acknowledged by $C000")
	}
	for range 1024 {
		m.Tick()
	}
	if !m.IRQ() {
		t.Error("IRQ not asserted 1024 cycles after restart")
	}
}
