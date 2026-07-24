package nes

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

// TestRegionTiming checks that each TV system runs at its real
// master-clock rate per frame, which is what makes PAL games play at 50
// fps instead of NTSC's 60. The test ROM never enables rendering, so no
// odd-frame dot is skipped and every frame is the full grid: NTSC
// 341*262 dots * 4 = 357368 master clocks, PAL and Dendy 341*312 * 5 =
// 531960.
func TestRegionTiming(t *testing.T) {
	cases := []struct {
		region   Region
		masterHz float64
		perFrame float64
		tol      float64
	}{
		{RegionNTSC, 21477272, 357368, 1},
		{RegionPAL, 26601712, 531960, 1},
		{RegionDendy, 26601712, 531960, 1},
	}
	for _, tc := range cases {
		t.Run(tc.region.String(), func(t *testing.T) {
			c, err := NewConsole(testrom.Image(t))
			if err != nil {
				t.Fatal(err)
			}
			c.SetRegion(tc.region)
			c.RunFrame() // settle onto a frame boundary
			start := c.State().MasterClock
			const n = 120
			for i := 0; i < n; i++ {
				c.RunFrame()
			}
			got := float64(c.State().MasterClock-start) / n
			if diff := got - tc.perFrame; diff < -tc.tol || diff > tc.tol {
				t.Errorf("master clocks/frame = %.1f, want %.0f (+/-%.0f); fps %.4f",
					got, tc.perFrame, tc.tol, tc.masterHz/got)
			}
		})
	}
}

// TestRegionAutoDetect checks that a PAL-flagged header powers on as PAL
// without an explicit override.
func TestRegionAutoDetect(t *testing.T) {
	rom := testrom.Image(t)
	rom[9] |= 0x01 // iNES byte 9 bit 0: PAL
	c, err := NewConsole(rom)
	if err != nil {
		t.Fatal(err)
	}
	c.RunFrame()
	start := c.State().MasterClock
	c.RunFrame()
	if perFrame := c.State().MasterClock - start; perFrame < 531000 || perFrame > 533000 {
		t.Errorf("auto-detected PAL frame = %d master clocks, want ~531960", perFrame)
	}
}

// TestRegionNotInSnapshot checks that a save state carries no region: a
// state taken on PAL restores onto an NTSC console without changing its
// timing (the region is a fixed property of the machine, like the ROM).
func TestRegionNotInSnapshot(t *testing.T) {
	pal, err := NewConsole(testrom.Image(t))
	if err != nil {
		t.Fatal(err)
	}
	pal.SetRegion(RegionPAL)
	pal.RunFrame()
	blob := pal.SaveState()

	ntsc, err := NewConsole(testrom.Image(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := ntsc.LoadState(blob); err != nil {
		t.Fatal(err)
	}
	ntsc.RunFrame() // settle
	start := ntsc.State().MasterClock
	ntsc.RunFrame()
	if perFrame := ntsc.State().MasterClock - start; perFrame > 358000 {
		t.Errorf("after loading a PAL state, NTSC console runs %d master clocks/frame; region leaked into the snapshot", perFrame)
	}
}
