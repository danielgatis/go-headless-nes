package nes

import "testing"

func TestAPUStatePulseDuty(t *testing.T) {
	c := newConsole(t)
	// $4000 bits 6-7 select pulse 1's duty; 0xC0 selects duty 3.
	c.Poke(0x4000, 0xC0)

	if got := c.APUState().Pulse1.Duty; got != 3 {
		t.Fatalf("Pulse1.Duty = %d, want 3", got)
	}
}

func TestAPUStateDMCOutput(t *testing.T) {
	c := newConsole(t)
	// $4011 sets the DMC's 7-bit output level directly.
	c.Poke(0x4011, 0x40)

	if got := c.APUState().DMC.Output; got != 0x40 {
		t.Fatalf("DMC.Output = %02X, want 40", got)
	}
}

func TestAPUStateReadable(t *testing.T) {
	c := newConsole(t)
	for i := 0; i < 2; i++ {
		c.RunFrame()
	}
	// Smoke: the whole snapshot is reachable without panicking and the DMC
	// rate is a real (non-zero) period.
	s := c.APUState()
	if s.DMC.Rate == 0 {
		t.Error("DMC.Rate = 0, expected a region rate-table period")
	}
}
