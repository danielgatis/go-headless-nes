package mapper

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

func TestBandaiKaraoke(t *testing.T) {
	c := cart(188, 8, 1)
	m := NewBandaiKaraoke(c)
	c.PRG[7*0x4000+0x0100] = 0xFF // conflict-free write site in fixed bank
	m.WritePRG(0xC100, 0x17)      // internal ROM, bank 7
	if got := m.ReadPRG(0x8000); got != 7 {
		t.Errorf("PRG bank = %d, want 7", got)
	}
	if m.Mirroring() != cartridge.Vertical {
		t.Errorf("mirroring = %v, want vertical", m.Mirroring())
	}
	// Selecting the absent expansion pack floats the window.
	m.SetOpenBus(0xAB)
	m.WritePRG(0xC100, 0x03)
	if got := m.ReadPRG(0x8000); got != 0xAB {
		t.Errorf("expansion window = %#x, want open bus 0xAB", got)
	}
	// Microphone port: idle bits under open bus.
	m.SetOpenBus(0xFF)
	if got := m.ReadPRG(0x6000); got != 0xF8 {
		t.Errorf("microphone read = %#x, want 0xF8", got)
	}
}
