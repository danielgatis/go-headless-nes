package nes

import "testing"

func TestOAMRoundTrip(t *testing.T) {
	c := newConsole(t)
	if got := len(c.OAM()); got != 256 {
		t.Fatalf("OAM length = %d, want 256", got)
	}

	want := make([]byte, 256)
	for i := range want {
		want[i] = byte(i)
	}
	c.SetOAM(want)

	got := c.OAM()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OAM[%d] = %02X, want %02X", i, got[i], want[i])
		}
	}
}

func TestPaletteRAMRoundTrip(t *testing.T) {
	c := newConsole(t)
	if got := len(c.PaletteRAM()); got != 32 {
		t.Fatalf("PaletteRAM length = %d, want 32", got)
	}

	c.SetPaletteRAM([]byte{0x0F, 0x21, 0x11, 0x01})
	got := c.PaletteRAM()
	for i, want := range []byte{0x0F, 0x21, 0x11, 0x01} {
		if got[i] != want {
			t.Fatalf("PaletteRAM[%d] = %02X, want %02X", i, got[i], want)
		}
	}
}

func TestSetRegister(t *testing.T) {
	c := newConsole(t)

	for _, tc := range []struct {
		name string
		val  uint16
		get  func() uint16
	}{
		{"A", 0x80, func() uint16 { return uint16(c.State().A) }},
		{"X", 0x05, func() uint16 { return uint16(c.State().X) }},
		{"Y", 0x7F, func() uint16 { return uint16(c.State().Y) }},
		{"SP", 0xFD, func() uint16 { return uint16(c.State().SP) }},
		{"P", 0x24, func() uint16 { return uint16(c.State().P) }},
		{"PC", 0xC000, func() uint16 { return c.State().PC }},
	} {
		if err := c.SetRegister(tc.name, tc.val); err != nil {
			t.Fatalf("SetRegister(%q): %v", tc.name, err)
		}
		if got := tc.get(); got != tc.val {
			t.Errorf("after SetRegister(%q, %04X): got %04X", tc.name, tc.val, got)
		}
	}
}

func TestSetRegisterUnknown(t *testing.T) {
	c := newConsole(t)
	if err := c.SetRegister("Z", 0); err == nil {
		t.Fatal("SetRegister with unknown name should error")
	}
}
