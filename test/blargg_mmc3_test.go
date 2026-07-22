package test

import "testing"

func Test_blarggMMC3(t *testing.T) {
	t.Parallel()
	runBlarggTable(t, []blarggCase{
		{"IRQ clocking", "roms/mmc3_irq_tests/1.Clocking.nes", P, -1, "MMC3 IRQ COUNTER\nPASSED"},
		{"IRQ details", "roms/mmc3_irq_tests/2.Details.nes", P, -1, "MMC3 IRQ COUNTER DETAILS\nPASSED"},
		{"IRQ A12 clocking", "roms/mmc3_irq_tests/3.A12_clocking.nes", P, -1, "MMC3 IRQ COUNTER A12\nPASSED"},
		{"IRQ scanline timing", "roms/mmc3_irq_tests/4.Scanline_timing.nes", P, -1, "MMC3 IRQ TIMING\nPASSED"},
	})
}
