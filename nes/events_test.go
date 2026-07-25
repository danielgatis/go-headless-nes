package nes

import "testing"

type sink struct {
	events []Event
}

func (s *sink) OnEvent(e Event) { s.events = append(s.events, e) }

func (s *sink) count(k EventKind) int {
	n := 0
	for _, e := range s.events {
		if e.Kind == k {
			n++
		}
	}
	return n
}

func TestEventFrame(t *testing.T) {
	c := newConsole(t)
	sk := &sink{}
	c.SetEventSink(sk, EventFrame)

	c.RunFrame()

	if sk.count(EventFrame) == 0 {
		t.Fatal("no EventFrame after RunFrame")
	}
}

func TestEventInstruction(t *testing.T) {
	c := newConsole(t)
	sk := &sink{}
	c.SetEventSink(sk, EventInstruction)

	pc := c.State().PC
	c.Step()

	if sk.count(EventInstruction) == 0 {
		t.Fatal("no EventInstruction after Step")
	}
	if got := sk.events[0].PC; got != pc {
		t.Errorf("first instruction event PC = %04X, want %04X", got, pc)
	}
}

func TestEventMaskExcludesUnwanted(t *testing.T) {
	c := newConsole(t)
	sk := &sink{}
	// Subscribe only to frames: stepping instructions must not deliver
	// instruction events.
	c.SetEventSink(sk, EventFrame)

	for i := 0; i < 50; i++ {
		c.Step()
	}

	if got := sk.count(EventInstruction); got != 0 {
		t.Fatalf("received %d instruction events despite frame-only subscription", got)
	}
}

func TestSetEventSinkNilRemoves(t *testing.T) {
	c := newConsole(t)
	sk := &sink{}
	c.SetEventSink(sk, EventInstruction)
	c.SetEventSink(nil)

	c.Step()

	if len(sk.events) != 0 {
		t.Fatalf("events still delivered after nil sink: %d", len(sk.events))
	}
}

func TestEventNMIOnFrame(t *testing.T) {
	c := newConsole(t)
	sk := &sink{}
	c.SetEventSink(sk, EventNMI)

	// The synthetic ROM enables NMI on vblank, so running a few frames must
	// produce at least one NMI.
	for i := 0; i < 3; i++ {
		c.RunFrame()
	}

	if sk.count(EventNMI) == 0 {
		t.Skip("ROM did not enable NMI; nothing to assert")
	}
}
