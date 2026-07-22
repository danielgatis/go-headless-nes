package bus

import "testing"

func TestMemoryRAMMirroring(t *testing.T) {
	m := New()
	m.Write(0x0000, 0x42)
	for _, a := range []uint16{0x0000, 0x0800, 0x1000, 0x1800} {
		if got := m.Read(a); got != 0x42 {
			t.Errorf("RAM mirror at %04X = %02X, want 42", a, got)
		}
	}
	m.Write(0x07FF, 0x99)
	if got := m.Read(0x1FFF); got != 0x99 {
		t.Errorf("RAM mirror $1FFF = %02X, want 99", got)
	}
}

func TestMemoryOpenBus(t *testing.T) {
	m := New()
	// A write drives both latches.
	m.Write(0x0000, 0xAB) // RAM write drives open bus
	if m.OpenBus() != 0xAB || m.InternalOpenBus() != 0xAB {
		t.Errorf("write open bus = ext %02X int %02X, want AB/AB", m.OpenBus(), m.InternalOpenBus())
	}
	// A read of an unmapped address ($4020, cartridge port, no handler) returns
	// the external latch and does not change it beyond what it read.
	m.SetOpenBus(External, 0x5C)
	if got := m.Read(0x4020); got != 0x5C {
		t.Errorf("unmapped read = %02X, want 5C (external open bus)", got)
	}
}

func TestMemoryBusTypeExternalOnly(t *testing.T) {
	m := New()
	m.SetOpenBus(Both, 0x00)
	// Reading RAM on the external bus only must not touch the internal latch.
	m.Write(0x0010, 0x77)
	m.SetOpenBus(Internal, 0x33) // set internal to a distinct value
	_ = m.ReadBus(0x0010, External)
	if m.InternalOpenBus() != 0x33 {
		t.Errorf("external-only read changed internal latch to %02X, want 33", m.InternalOpenBus())
	}
	if m.OpenBus() != 0x77 {
		t.Errorf("external latch = %02X, want 77", m.OpenBus())
	}
}
