package cartridge

import (
	"bytes"
	"errors"
	"testing"
)

// header builds a 16-byte iNES header for synthetic test images.
func header(prgBanks, chrBanks, flags6, flags7 byte) []byte {
	h := make([]byte, headerSize)
	copy(h, magic[:])
	h[4] = prgBanks
	h[5] = chrBanks
	h[6] = flags6
	h[7] = flags7
	return h
}

// image appends PRG and CHR payloads of the declared sizes.
func image(prgBanks, chrBanks, flags6, flags7 byte) []byte {
	img := header(prgBanks, chrBanks, flags6, flags7)
	img = append(img, bytes.Repeat([]byte{0xAA}, int(prgBanks)*prgUnit)...)
	img = append(img, bytes.Repeat([]byte{0xBB}, int(chrBanks)*chrUnit)...)
	return img
}

func TestLoadMirrors16KAndReadsVectors(t *testing.T) {
	// A single-bank NROM board (16 KiB PRG, 8 KiB CHR, mapper 0,
	// horizontal) like nestest's. The loader must mirror the half-size
	// PRG up to the 32 KiB window so bank arithmetic is safe on every
	// board, and the reset vector at $FFFC must survive intact.
	img := image(1, 1, 0, 0)
	// Point the reset vector at $C004 (bytes at $FFFC/$FFFD, which sit at
	// the end of the single 16 KiB PRG bank).
	img[headerSize+prgUnit-4] = 0x04 // $FFFC low
	img[headerSize+prgUnit-3] = 0xC0 // $FFFD high

	c, err := Load(bytes.NewReader(img))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(c.PRG), 2*prgUnit; got != want {
		t.Errorf("PRG size = %d, want %d", got, want)
	}
	if !bytes.Equal(c.PRG[:prgUnit], c.PRG[prgUnit:]) {
		t.Error("mirrored PRG halves differ")
	}
	if got, want := len(c.CHR), chrUnit; got != want {
		t.Errorf("CHR size = %d, want %d", got, want)
	}
	if c.MapperID != 0 {
		t.Errorf("mapper = %d, want 0", c.MapperID)
	}
	if c.Mirroring != Horizontal {
		t.Errorf("mirroring = %v, want horizontal", c.Mirroring)
	}
	lo := c.PRG[0xFFFC-0xC000]
	hi := c.PRG[0xFFFD-0xC000]
	if v := uint16(hi)<<8 | uint16(lo); v != 0xC004 {
		t.Errorf("reset vector = $%04X, want $C004", v)
	}
}

func TestLoadRejectsBadMagic(t *testing.T) {
	img := image(1, 1, 0, 0)
	img[0] = 'X'
	if _, err := Load(bytes.NewReader(img)); !errors.Is(err, ErrNotINES) {
		t.Errorf("err = %v, want ErrNotINES", err)
	}
}

func TestLoadRejectsTruncated(t *testing.T) {
	img := image(2, 1, 0, 0)
	for _, cut := range []int{4, headerSize, headerSize + 100, len(img) - 1} {
		if _, err := Load(bytes.NewReader(img[:cut])); err == nil {
			t.Errorf("cut at %d: expected error", cut)
		}
	}
}

func TestLoadRejectsZeroPRG(t *testing.T) {
	if _, err := Load(bytes.NewReader(image(0, 1, 0, 0))); err == nil {
		t.Error("expected error for zero PRG banks")
	}
}

func TestLoadFlags(t *testing.T) {
	tests := []struct {
		name           string
		flags6, flags7 byte
		mirroring      Mirroring
		mapper         uint16
		battery        bool
	}{
		{"horizontal", 0x00, 0x00, Horizontal, 0, false},
		{"vertical", 0x01, 0x00, Vertical, 0, false},
		{"four-screen wins", 0x09, 0x00, FourScreen, 0, false},
		{"battery", 0x02, 0x00, Horizontal, 0, true},
		{"mapper nibbles", 0x40, 0x30, Horizontal, 0x34, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(bytes.NewReader(image(1, 1, tt.flags6, tt.flags7)))
			if err != nil {
				t.Fatal(err)
			}
			if c.Mirroring != tt.mirroring {
				t.Errorf("mirroring = %v, want %v", c.Mirroring, tt.mirroring)
			}
			if c.MapperID != tt.mapper {
				t.Errorf("mapper = %d, want %d", c.MapperID, tt.mapper)
			}
			if c.HasBattery != tt.battery {
				t.Errorf("battery = %v, want %v", c.HasBattery, tt.battery)
			}
		})
	}
}

func TestLoadNES20ExtendedMapper(t *testing.T) {
	// NES 2.0: flags7 bits 2-3 = 10; byte 8 carries mapper bits 8-11
	// (low nibble) and the submapper (high nibble).
	img := header(1, 1, 0x40, 0x38)
	img[8] = 0x21 // submapper 2, mapper high nibble 1
	img = append(img, bytes.Repeat([]byte{0xAA}, prgUnit)...)
	img = append(img, bytes.Repeat([]byte{0xBB}, chrUnit)...)

	c, err := Load(bytes.NewReader(img))
	if err != nil {
		t.Fatal(err)
	}
	if want := uint16(0x134); c.MapperID != want {
		t.Errorf("mapper = %03X, want %03X", c.MapperID, want)
	}
	if c.Submapper != 2 {
		t.Errorf("submapper = %d, want 2", c.Submapper)
	}
}

func TestLoadNES20RejectsExponentSizes(t *testing.T) {
	img := header(1, 1, 0, 0x08)
	img[9] = 0x0F // PRG size uses exponent notation
	img = append(img, bytes.Repeat([]byte{0xAA}, prgUnit)...)
	img = append(img, bytes.Repeat([]byte{0xBB}, chrUnit)...)
	if _, err := Load(bytes.NewReader(img)); err == nil {
		t.Error("expected error for exponent-encoded sizes")
	}
}

func TestLoadSkipsTrainer(t *testing.T) {
	img := header(1, 0, 0x04, 0)
	img = append(img, make([]byte, trainerSize)...) // trainer junk
	prg := bytes.Repeat([]byte{0xCC}, prgUnit)
	img = append(img, prg...)
	c, err := Load(bytes.NewReader(img))
	if err != nil {
		t.Fatal(err)
	}
	if c.PRG[0] != 0xCC {
		t.Errorf("PRG[0] = %02X, want CC (trainer not skipped?)", c.PRG[0])
	}
	if c.CHR != nil {
		t.Errorf("CHR = %d bytes, want none (CHR RAM board)", len(c.CHR))
	}
}

func TestLoadDirtyArchaicHeader(t *testing.T) {
	// Pre-iNES-0.7 dumps carry ripper signatures in bytes 8-15,
	// spilling garbage into byte 7's mapper nibble.
	img := header(1, 1, 0x10, 0x44) // byte 7 = 'D' of "DiskDude!"
	copy(img[8:], "iskDude!")
	img = append(img, bytes.Repeat([]byte{0xAA}, prgUnit)...)
	img = append(img, bytes.Repeat([]byte{0xBB}, chrUnit)...)
	c, err := Load(bytes.NewReader(img))
	if err != nil {
		t.Fatal(err)
	}
	if c.MapperID != 1 {
		t.Errorf("mapper = %d, want 1 (byte 7 garbage ignored)", c.MapperID)
	}
}

func TestLoadRejectsImplausibleBankCounts(t *testing.T) {
	img := header(1, 1, 0, 0x08) // NES 2.0
	img[9] = 0x01                // PRG banks |= 1<<8 -> 257
	if _, err := Load(bytes.NewReader(img)); err == nil {
		t.Error("expected error for implausible PRG bank count")
	}
}

func TestLoadCHRRAMBoard(t *testing.T) {
	c, err := Load(bytes.NewReader(image(1, 0, 0, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.CHR) != 0 {
		t.Errorf("CHR = %d bytes, want 0", len(c.CHR))
	}
}
