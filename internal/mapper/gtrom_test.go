package mapper

import "testing"

func TestGTROMFlash(t *testing.T) {
	m := NewGTROM(cart(111, 32, 0))
	// Software ID mode.
	m.WritePRG(0xD555, 0xAA)
	m.WritePRG(0xAAAA, 0x55)
	m.WritePRG(0xD555, 0x90)
	if got := m.ReadPRG(0x8000); got != 0xBF {
		t.Errorf("manufacturer ID = %#x, want 0xBF", got)
	}
	if got := m.ReadPRG(0x8001); got != 0xB7 {
		t.Errorf("device ID = %#x, want 0xB7", got)
	}
	m.WritePRG(0x8000, 0xF0) // exit
	// Byte program in bank 1 (flash can only clear bits).
	m.WritePRG(0x5000, 0x01)
	m.WritePRG(0xD555, 0xAA)
	m.WritePRG(0xAAAA, 0x55)
	m.WritePRG(0xD555, 0xA0)
	m.WritePRG(0x8010, 0x00)
	if got := m.ReadPRG(0x8010); got != 0 {
		t.Errorf("programmed byte = %#x, want 0", got)
	}
}

func TestGTROMNametableRAM(t *testing.T) {
	m := NewGTROM(cart(111, 32, 0))
	m.WriteNT(0x2000, 0x11)
	m.WritePRG(0x5000, 0x20) // switch to the second NT page
	m.WriteNT(0x2000, 0x22)
	if v, _ := m.ReadNT(0x2000); v != 0x22 {
		t.Errorf("NT page 1 = %#x, want 0x22", v)
	}
	m.WritePRG(0x5000, 0x00)
	if v, _ := m.ReadNT(0x2000); v != 0x11 {
		t.Errorf("NT page 0 = %#x, want 0x11", v)
	}
}
