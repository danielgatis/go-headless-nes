package nes

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

// recorder is a test Observer that logs the reads and writes it sees.
type recorder struct {
	reads  map[uint16]byte
	writes map[uint16]byte
}

func newRecorder() *recorder {
	return &recorder{reads: map[uint16]byte{}, writes: map[uint16]byte{}}
}

func (r *recorder) OnRead(addr uint16, v byte)  { r.reads[addr] = v }
func (r *recorder) OnWrite(addr uint16, v byte) { r.writes[addr] = v }

// filterFunc adapts two closures to the MemFilter interface.
type filterFunc struct {
	read  func(uint16, byte) (byte, FilterAction)
	write func(uint16, byte) (byte, FilterAction)
}

func (f filterFunc) FilterRead(a uint16, v byte) (byte, FilterAction) {
	if f.read == nil {
		return v, Pass
	}
	return f.read(a, v)
}

func (f filterFunc) FilterWrite(a uint16, v byte) (byte, FilterAction) {
	if f.write == nil {
		return v, Pass
	}
	return f.write(a, v)
}

func newConsole(t *testing.T) *Console {
	t.Helper()
	c, err := NewConsole(testrom.Image(t))
	if err != nil {
		t.Fatalf("NewConsole: %v", err)
	}
	return c
}

func TestObserverSeesOpcodeFetch(t *testing.T) {
	c := newConsole(t)
	rec := newRecorder()
	c.SetObserver(rec)

	pc := c.State().PC
	want := c.Peek(pc) // the opcode about to be fetched
	c.Step()

	got, ok := rec.reads[pc]
	if !ok {
		t.Fatalf("observer saw no read at the opcode-fetch address %04X", pc)
	}
	if got != want {
		t.Errorf("observed fetch at %04X = %02X, want %02X (matching Peek)", pc, got, want)
	}
}

func TestObserverSeesWrite(t *testing.T) {
	c := newConsole(t)
	rec := newRecorder()
	c.SetObserver(rec)

	c.Poke(0x0200, 0x42)

	if got, ok := rec.writes[0x0200]; !ok || got != 0x42 {
		t.Fatalf("observer write at 0200 = %02X (seen=%v), want 42", got, ok)
	}
}

func TestSetObserverNilRemoves(t *testing.T) {
	c := newConsole(t)
	rec := newRecorder()
	c.SetObserver(rec)
	c.SetObserver(nil)

	c.Poke(0x0200, 0x42)

	if len(rec.writes) != 0 {
		t.Fatalf("observer still recording after SetObserver(nil): %v", rec.writes)
	}
}

func TestFilterReadSubstitutes(t *testing.T) {
	c := newConsole(t)
	rec := newRecorder()
	c.SetObserver(rec)

	pc := c.State().PC
	// Game-Genie style: the byte fetched at pc reads back as 0xAB.
	c.SetMemFilter(filterFunc{read: func(a uint16, v byte) (byte, FilterAction) {
		if a == pc {
			return 0xAB, Replace
		}
		return v, Pass
	}})

	c.Step()

	// The filter runs before the observer, so the observer must see the
	// substituted value, proving the CPU saw it too.
	if got := rec.reads[pc]; got != 0xAB {
		t.Fatalf("observed fetch at %04X = %02X, want AB (filter not applied)", pc, got)
	}
}

func TestFilterWriteBlocks(t *testing.T) {
	c := newConsole(t)
	c.Poke(0x0200, 0x11)
	c.SetMemFilter(filterFunc{write: func(a uint16, v byte) (byte, FilterAction) {
		if a == 0x0200 {
			return v, Block
		}
		return v, Pass
	}})

	c.Poke(0x0200, 0x99)

	if got := c.Peek(0x0200); got != 0x11 {
		t.Fatalf("blocked write still landed: 0200 = %02X, want 11", got)
	}
}

func TestFilterWriteReplaces(t *testing.T) {
	c := newConsole(t)
	c.SetMemFilter(filterFunc{write: func(a uint16, v byte) (byte, FilterAction) {
		if a == 0x0200 {
			return 0x7F, Replace
		}
		return v, Pass
	}})

	c.Poke(0x0200, 0x99)

	if got := c.Peek(0x0200); got != 0x7F {
		t.Fatalf("replaced write = %02X, want 7F", got)
	}
}

func TestNoHooksLeavesMemoryIntact(t *testing.T) {
	c := newConsole(t)
	c.Poke(0x0200, 0x55)
	if got := c.Peek(0x0200); got != 0x55 {
		t.Fatalf("plain poke/peek = %02X, want 55", got)
	}
}
