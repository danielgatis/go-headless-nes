package test

import "testing"

// Test_blarggMMC3Suite runs the two MMC3 IRQ test suites. The core
// implements the normal MMC3B revision, so 5-MMC3 (zero latch IRQs every
// clock) passes. Test 6 (6-MMC6 / 6-MMC3_alt) checks the *alternate*
// silicon revision, whose reload-to-zero IRQ suppression is mutually
// exclusive with 5-MMC3 — no single implementation passes both. Those
// two cases live in Test_blarggMMC3Alt below, skipped by design.
func Test_blarggMMC3Suite(t *testing.T) {
	t.Parallel()
	runBlarggTable(t, []blarggCase{
		{"v1 clocking", "roms/mmc3_test/1-clocking.nes", S, 0, "1-clocking\n\nPassed"},
		{"v1 details", "roms/mmc3_test/2-details.nes", S, 0, "2-details\n\nPassed"},
		{"v1 A12 clocking", "roms/mmc3_test/3-A12_clocking.nes", S, 0, "3-A12_clocking\n\nPassed"},
		{"v1 scanline timing", "roms/mmc3_test/4-scanline_timing.nes", S, 0, "4-scanline_timing\n\nPassed"},
		{"v1 MMC3", "roms/mmc3_test/5-MMC3.nes", S, 0, "5-MMC3\n\nPassed"},

		{"v2 clocking", "roms/mmc3_test_2/rom_singles/1-clocking.nes", S, 0, "1-clocking\n\nPassed"},
		{"v2 details", "roms/mmc3_test_2/rom_singles/2-details.nes", S, 0, "2-details\n\nPassed"},
		{"v2 A12 clocking", "roms/mmc3_test_2/rom_singles/3-A12_clocking.nes", S, 0, "3-A12_clocking\n\nPassed"},
		{"v2 scanline timing", "roms/mmc3_test_2/rom_singles/4-scanline_timing.nes", S, 0, "4-scanline_timing\n\nPassed"},
		{"v2 MMC3", "roms/mmc3_test_2/rom_singles/5-MMC3.nes", S, 0, "5-MMC3\n\nPassed"},
	})
}

// Test_blarggMMC3Alt covers test 6 of the MMC3 suites, which targets the
// alternate MMC3 revision (some MMC3B-marked chips, e.g. certain Crystalis
// carts). Its reload-to-zero IRQ suppression contradicts 5-MMC3, so a
// core that passes the normal suite cannot pass this one. Skipped by
// design rather than emulated, since the games targeted here use the
// normal revision.
func Test_blarggMMC3Alt(t *testing.T) {
	t.Skip("alternate MMC3 revision (6-MMC6 / 6-MMC3_alt): mutually exclusive with normal-revision 5-MMC3, which the core implements")
}
