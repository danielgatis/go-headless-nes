package mapper

import "testing"

func TestProbeBankMapUxROM(t *testing.T) {
	c := cart(2, 8, 0) // 8 x 16 KiB PRG, CHR RAM
	m := NewUxROM(c)

	c.PRG[0x0200] = 0xFF
	m.WritePRG(0x8200, 3) // select 16 KiB bank 3 at $8000

	bm := ProbeBankMap(m)
	// $8000/$A000 are the two 8 KiB halves of 16 KiB bank 3 (6, 7);
	// $C000/$E000 are the halves of the hardwired last bank 7 (14, 15).
	if bm.PRG != [4]int{6, 7, 14, 15} {
		t.Errorf("PRG = %v, want [6 7 14 15]", bm.PRG)
	}
	if bm.CHR != [8]int{0, 1, 2, 3, 4, 5, 6, 7} {
		t.Errorf("CHR = %v, want linear (CHR RAM)", bm.CHR)
	}
}

func TestProbeBankMapCNROM(t *testing.T) {
	c := cart(3, 1, 4) // 1 x 16 KiB PRG, 4 x 8 KiB CHR
	m := NewCNROM(c)

	c.PRG[0x0100] = 0xFF
	m.WritePRG(0x8100, 2) // select 8 KiB CHR bank 2

	bm := ProbeBankMap(m)
	if bm.CHR != [8]int{16, 17, 18, 19, 20, 21, 22, 23} {
		t.Errorf("CHR = %v, want 16..23", bm.CHR)
	}
	// Fixed 16 KiB PRG mirrors across the 32 KiB window.
	if bm.PRG != [4]int{0, 1, 0, 1} {
		t.Errorf("PRG = %v, want [0 1 0 1]", bm.PRG)
	}
}

func TestProbeBankMapAllSupported(t *testing.T) {
	knownPRG, knownCHR, total := 0, 0, 0
	for _, tc := range supportedMapperIDs {
		m, err := New(smokeCart(tc.id, tc.sub))
		if err != nil {
			continue
		}
		total++
		bm := ProbeBankMap(m) // must not panic on any board

		anyPRG := false
		for _, b := range bm.PRG {
			if b >= 0 {
				anyPRG = true
			}
		}
		anyCHR := false
		for _, b := range bm.CHR {
			if b >= 0 {
				anyCHR = true
			}
		}
		if anyPRG {
			knownPRG++
		}
		if anyCHR {
			knownCHR++
		}
	}
	if total == 0 {
		t.Fatal("no mappers built")
	}
	t.Logf("probed %d mappers: %d report PRG banks, %d report CHR banks", total, knownPRG, knownCHR)
	// Nearly every board banks through the shared helper, so most must
	// report banks. A far lower count means the generic probe broke.
	if knownPRG < total*3/4 {
		t.Errorf("only %d/%d mappers reported PRG banks", knownPRG, total)
	}
	if knownCHR < total*3/4 {
		t.Errorf("only %d/%d mappers reported CHR banks", knownCHR, total)
	}
}

func TestProbeBankMapMMC3(t *testing.T) {
	c := cart(4, 8, 8) // 8 x 16 KiB PRG, 8 x 8 KiB CHR
	m, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	// A freshly reset MMC3 has a defined layout; the probe must return
	// concrete 8 KiB PRG banks (never -1, since MMC3 banks all four
	// windows) and the fixed last two banks at $C000/$E000.
	bm := ProbeBankMap(m)
	for w, b := range bm.PRG {
		if b < 0 {
			t.Errorf("PRG window %d = -1, MMC3 maps every window", w)
		}
	}
	// MMC3 fixes the last 8 KiB bank at $E000.
	last := len(c.PRG)/0x2000 - 1
	if bm.PRG[3] != last {
		t.Errorf("PRG[$E000] = %d, want last bank %d", bm.PRG[3], last)
	}
}
