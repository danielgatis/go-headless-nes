package nes

import "testing"

// halter halts the console after a fixed number of instructions.
type halter struct {
	c  *Console
	at int
	n  int
}

func (h *halter) OnEvent(e Event) {
	if e.Kind != EventInstruction {
		return
	}
	h.n++
	if h.n == h.at {
		h.c.Halt()
	}
}

func TestHaltInterruptsRunFrame(t *testing.T) {
	c := newConsole(t)
	h := &halter{c: c, at: 100}
	c.SetEventSink(h, EventInstruction)

	before := c.State().Frame
	stop := c.RunFrame()

	if stop.Reason != StopHalt {
		t.Fatalf("RunFrame stop reason = %d, want StopHalt (%d)", stop.Reason, StopHalt)
	}
	// A full frame is thousands of instructions, so halting at 100 must
	// have returned before this frame completed.
	if got := c.State().Frame; got != before {
		t.Errorf("frame advanced from %d to %d despite halting mid-frame", before, got)
	}
}

func TestRunFrameClearsStaleHalt(t *testing.T) {
	c := newConsole(t)
	h := &halter{c: c, at: 100}
	c.SetEventSink(h, EventInstruction)

	if stop := c.RunFrame(); stop.Reason != StopHalt {
		t.Fatalf("first RunFrame = %d, want StopHalt", stop.Reason)
	}
	// The halter only fires once (n passes 100 and never equals it again),
	// so the next RunFrame must run to a real frame boundary.
	if stop := c.RunFrame(); stop.Reason != StopNone {
		t.Fatalf("second RunFrame = %d, want StopNone (halt not cleared)", stop.Reason)
	}
}
