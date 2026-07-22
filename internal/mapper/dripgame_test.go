package mapper

import "testing"

func TestDripGameExtendedAttributes(t *testing.T) {
	m := NewDripGame(cart(284, 8, 4))
	m.WritePRG(0x800A, 0x04) // vertical mirroring + extended attributes
	m.WritePRG(0xC400, 0x02) // tile 0 of the second attribute bank
	if _, ok := m.ReadNT(0x2000); ok {
		t.Error("plain nametable fetches must fall through to CIRAM")
	}
	// Attribute fetch in the second logical table (vertical: bit 10).
	if _, ok := m.ReadNT(0x2400); ok {
		t.Error("nametable fetch must fall through")
	}
	v, ok := m.ReadNT(0x27C0)
	if !ok || v != 0xAA {
		t.Errorf("extended attribute = %#x,%v, want 0xAA,true", v, ok)
	}
}

func TestDripGameAudioFIFO(t *testing.T) {
	m := NewDripGame(cart(284, 8, 4))
	if got := m.ReadPRG(0x5000); got != 0x40 {
		t.Errorf("channel status = %#x, want 0x40 (empty)", got)
	}
	m.WritePRG(0x8002, 0x10) // period
	m.WritePRG(0x8003, 0xF0) // volume 15
	m.WritePRG(0x8001, 0xFF) // push a sample; playback starts
	if got := m.ReadPRG(0x5000); got&0x40 != 0 {
		t.Errorf("channel status = %#x, want not empty", got)
	}
	if m.AudioLevel() <= 0 {
		t.Errorf("audio level = %v, want > 0", m.AudioLevel())
	}
}
