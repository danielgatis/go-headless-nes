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

func TestVRAMNametableRoundTrip(t *testing.T) {
	c := newConsole(t)
	// $2000 is CIRAM (a nametable), always writable regardless of the board.
	c.PokeVRAM(0x2000, 0xAB)
	if got := c.PeekVRAM(0x2000); got != 0xAB {
		t.Fatalf("nametable PeekVRAM(2000) = %02X, want AB", got)
	}
}

func TestVRAMPaletteRoundTrip(t *testing.T) {
	c := newConsole(t)
	c.PokeVRAM(0x3F01, 0x21)
	if got := c.PeekVRAM(0x3F01); got != 0x21 {
		t.Fatalf("palette PeekVRAM(3F01) = %02X, want 21", got)
	}
	// The same byte is visible through the flat palette accessor.
	if got := c.PaletteRAM()[0x01]; got != 0x21 {
		t.Fatalf("PaletteRAM[1] = %02X, want 21", got)
	}
}

func TestPPUStateAgreesWithState(t *testing.T) {
	c := newConsole(t)
	for i := 0; i < 3; i++ {
		c.RunFrame()
	}
	ps := c.PPUState()
	s := c.State()
	if ps.Frame != s.Frame {
		t.Errorf("PPUState.Frame = %d, State.Frame = %d", ps.Frame, s.Frame)
	}
	if ps.Scanline != s.Scanline || ps.Dot != s.Dot {
		t.Errorf("PPUState position (%d,%d) != State (%d,%d)", ps.Scanline, ps.Dot, s.Scanline, s.Dot)
	}
}

func TestMapperInfo(t *testing.T) {
	c := newConsole(t)
	info := c.MapperInfo()
	if info.ID != 0 {
		t.Errorf("mapper ID = %d, want 0", info.ID)
	}
	if info.Mirroring == "" {
		t.Error("Mirroring empty")
	}
	// NROM implements BankMapper, so the layout is known.
	if !info.HasBankInfo {
		t.Fatal("HasBankInfo = false, expected NROM to report banks")
	}
	if len(info.PRGBanks) != 4 {
		t.Errorf("PRGBanks length = %d, want 4", len(info.PRGBanks))
	}
	if len(info.CHRBanks) != 8 {
		t.Errorf("CHRBanks length = %d, want 8", len(info.CHRBanks))
	}
}

func TestCartInfo(t *testing.T) {
	c := newConsole(t)
	info := c.CartInfo()
	if info.MapperID != 0 {
		t.Errorf("MapperID = %d, want 0 (synthetic NROM)", info.MapperID)
	}
	if info.PRGSize == 0 {
		t.Error("PRGSize = 0, want non-empty PRG ROM")
	}
	if info.Mirroring == "" {
		t.Error("Mirroring is empty")
	}
}
