package mapper

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

// mmc1Write drives the serial port: five LSB-first writes to addr.
func mmc1Write(m *MMC1, addr uint16, v byte) {
	for range 5 {
		m.WritePRG(addr, v&1)
		v >>= 1
	}
}

func TestMMC1PowerOnFixesLastBank(t *testing.T) {
	m := NewMMC1(cart(1, 8, 1))
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Errorf("$C000 bank = %d, want 7 (last)", got)
	}
	if got := m.ReadPRG(0x8000); got != 0 {
		t.Errorf("$8000 bank = %d, want 0", got)
	}
}

func TestMMC1PRGModes(t *testing.T) {
	m := NewMMC1(cart(1, 8, 1))

	mmc1Write(m, 0xE000, 3) // PRG bank 3 (mode 3: switchable at $8000)
	if got := m.ReadPRG(0x8000); got != 3 {
		t.Errorf("mode 3: $8000 bank = %d, want 3", got)
	}
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Errorf("mode 3: $C000 bank = %d, want 7", got)
	}

	mmc1Write(m, 0x8000, 0x08) // control: PRG mode 2 (first fixed)
	if got := m.ReadPRG(0x8000); got != 0 {
		t.Errorf("mode 2: $8000 bank = %d, want 0", got)
	}
	if got := m.ReadPRG(0xC000); got != 3 {
		t.Errorf("mode 2: $C000 bank = %d, want 3", got)
	}

	mmc1Write(m, 0x8000, 0x00) // control: 32 KiB mode
	mmc1Write(m, 0xE000, 3)    // bank 3 -> pair 2/3
	if got := m.ReadPRG(0x8000); got != 2 {
		t.Errorf("32K mode: $8000 bank = %d, want 2", got)
	}
	if got := m.ReadPRG(0xC000); got != 3 {
		t.Errorf("32K mode: $C000 bank = %d, want 3", got)
	}
}

func TestMMC1CHRModes(t *testing.T) {
	m := NewMMC1(cart(1, 2, 4)) // 4 x 8K CHR = 8 x 4K banks
	// CHR bytes encode 8K bank; 4K bank b lives in 8K bank b/2.
	mmc1Write(m, 0x8000, 0x1C)              // control: 4K CHR mode, PRG mode 3
	mmc1Write(m, 0xA000, 5)                 // CHR0 = 4K bank 5
	mmc1Write(m, 0xC000, 2)                 // CHR1 = 4K bank 2
	if got := m.ReadCHR(0x0000); got != 2 { // 4K bank 5 is in 8K bank 2
		t.Errorf("CHR $0000 = %d, want 2", got)
	}
	if got := m.ReadCHR(0x1000); got != 1 { // 4K bank 2 is in 8K bank 1
		t.Errorf("CHR $1000 = %d, want 1", got)
	}

	mmc1Write(m, 0x8000, 0x0C) // 8K CHR mode
	mmc1Write(m, 0xA000, 6)    // bank bit 0 ignored -> 8K bank 3
	if got := m.ReadCHR(0x1FFF); got != 3 {
		t.Errorf("8K CHR mode: read = %d, want 3", got)
	}
}

func TestMMC1ResetBit(t *testing.T) {
	m := NewMMC1(cart(1, 8, 1))
	mmc1Write(m, 0xE000, 3)
	m.WritePRG(0x8000, 0x01) // two stray bits...
	m.WritePRG(0x8000, 0x01)
	m.WritePRG(0x8000, 0x80) // ...cancelled by reset
	if m.count != 0 || m.shift != 0 {
		t.Error("reset bit did not clear the shift register")
	}
	// Reset also re-fixes the last bank at $C000.
	if got := m.ReadPRG(0xC000); got != 7 {
		t.Errorf("$C000 bank after reset = %d, want 7", got)
	}
}

func TestMMC1Mirroring(t *testing.T) {
	m := NewMMC1(cart(1, 2, 1))
	for control, want := range map[byte]cartridge.Mirroring{
		0: cartridge.SingleLow,
		1: cartridge.SingleHigh,
		2: cartridge.Vertical,
		3: cartridge.Horizontal,
	} {
		mmc1Write(m, 0x8000, control)
		if got := m.Mirroring(); got != want {
			t.Errorf("control %d: mirroring = %v, want %v", control, got, want)
		}
	}
}

func TestMMC1SaveRestore(t *testing.T) {
	m := NewMMC1(cart(1, 8, 4))
	mmc1Write(m, 0x8000, 0x1C)
	mmc1Write(m, 0xE000, 5)
	var s State
	m.Save(&s)

	m2 := NewMMC1(cart(1, 8, 4))
	m2.Restore(&s)
	if got := m2.ReadPRG(0x8000); got != 5 {
		t.Errorf("restored PRG bank read = %d, want 5", got)
	}
	if m2.control != 0x1C {
		t.Errorf("restored control = %02X, want 1C", m2.control)
	}
}
