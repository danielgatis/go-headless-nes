package mapper

import "testing"

func TestMMC3PRGModes(t *testing.T) {
	m := NewMMC3(prg8Cart(4, 8)) // 16 x 8K banks
	m.WritePRG(0x8000, 6)        // select R6
	m.WritePRG(0x8001, 3)
	m.WritePRG(0x8000, 7) // select R7
	m.WritePRG(0x8001, 5)

	// Mode 0: $8000=R6, $A000=R7, $C000=-2, $E000=-1.
	for _, tt := range []struct {
		addr uint16
		bank byte
	}{{0x8000, 3}, {0xA000, 5}, {0xC000, 14}, {0xE000, 15}} {
		if got := m.ReadPRG(tt.addr); got != tt.bank {
			t.Errorf("mode 0: $%04X bank = %d, want %d", tt.addr, got, tt.bank)
		}
	}

	m.WritePRG(0x8000, 0x46) // PRG swap mode, R6 still selected
	for _, tt := range []struct {
		addr uint16
		bank byte
	}{{0x8000, 14}, {0xA000, 5}, {0xC000, 3}, {0xE000, 15}} {
		if got := m.ReadPRG(tt.addr); got != tt.bank {
			t.Errorf("mode 1: $%04X bank = %d, want %d", tt.addr, got, tt.bank)
		}
	}
}

func TestMMC3CHRModes(t *testing.T) {
	m := NewMMC3(chrCart(4, 2, 16)) // 128 x 1K banks
	set := func(reg byte, v byte) {
		m.WritePRG(0x8000, reg)
		m.WritePRG(0x8001, v)
	}
	set(0, 4) // R0: 2K at $0000 (low bit ignored)
	set(1, 6) // R1: 2K at $0800
	set(2, 20)
	set(3, 21)
	set(4, 22)
	set(5, 23)

	for _, tt := range []struct {
		addr uint16
		bank byte
	}{
		{0x0000, 4}, {0x0400, 5}, {0x0800, 6}, {0x0C00, 7},
		{0x1000, 20}, {0x1400, 21}, {0x1800, 22}, {0x1C00, 23},
	} {
		if got := m.ReadCHR(tt.addr); got != tt.bank {
			t.Errorf("mode 0: CHR $%04X bank = %d, want %d", tt.addr, got, tt.bank)
		}
	}

	// CHR inversion (bank select bit 7) swaps the halves.
	m.WritePRG(0x8000, 0x80)
	if got := m.ReadCHR(0x0000); got != 20 {
		t.Errorf("inverted: CHR $0000 bank = %d, want 20", got)
	}
	if got := m.ReadCHR(0x1000); got != 4 {
		t.Errorf("inverted: CHR $1000 bank = %d, want 4", got)
	}
}

func TestMMC3IRQCounter(t *testing.T) {
	m := NewMMC3(prg8Cart(4, 2))
	m.WritePRG(0xC000, 3) // latch
	m.WritePRG(0xC001, 0) // reload on next clock
	m.WritePRG(0xE001, 0) // enable

	// Clock 1 reloads to 3; clocks 2-4 count 2,1,0 -> IRQ on 4th.
	for i := 1; i <= 3; i++ {
		m.Scanline()
		if m.IRQ() {
			t.Fatalf("IRQ asserted early, at clock %d", i)
		}
	}
	m.Scanline()
	if !m.IRQ() {
		t.Fatal("IRQ not asserted when counter reached 0")
	}

	// $E000 acknowledges and disables.
	m.WritePRG(0xE000, 0)
	if m.IRQ() {
		t.Error("$E000 did not acknowledge the IRQ")
	}
	for range 8 {
		m.Scanline()
	}
	if m.IRQ() {
		t.Error("IRQ asserted while disabled")
	}
}

func TestMMC3RAMProtect(t *testing.T) {
	m := NewMMC3(prg8Cart(4, 2))
	m.WritePRG(0x6000, 0x42)
	if got := m.ReadPRG(0x6000); got != 0x42 {
		t.Fatalf("PRG RAM = %02X, want 42", got)
	}
	m.WritePRG(0xA001, 0xC0) // enabled + write-protected
	m.WritePRG(0x6000, 0x99)
	if got := m.ReadPRG(0x6000); got != 0x42 {
		t.Error("write-protected RAM changed")
	}
	m.WritePRG(0xA001, 0x00) // disabled
	// With RAM disabled the port floats: the read returns whatever the
	// bus last drove, which the bus feeds in via SetOpenBus.
	m.SetOpenBus(0x5A)
	if got := m.ReadPRG(0x6000); got != 0x5A {
		t.Errorf("disabled RAM read = %02X, want 5A (open bus)", got)
	}
}
