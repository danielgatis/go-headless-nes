package test

import "testing"

// blarggPPUVblNMIWant is the full pass: the core runs the PPU to the exact
// master-clock position at each CPU sub-cycle, which lands vblank, NMI and
// the odd-frame skip on the precise dots all 10 sub-tests check. (The
// pre-rewrite dot-split core failed at 05-nmi_timing.)
const blarggPPUVblNMIWant = "08 08 09 07 \n10-even_odd_timing\n\nPassed\nAll 10 tests passed"

func Test_blarggPPU(t *testing.T) {
	t.Parallel()
	runBlarggTable(t, []blarggCase{
		{"open bus", "roms/ppu_open_bus/ppu_open_bus.nes", S, 0, "ppu_open_bus\n\nPassed"},
		{"vbl nmi", "roms/ppu_vbl_nmi/ppu_vbl_nmi.nes", S, 0, blarggPPUVblNMIWant},

		{"sprite overflow basics", "roms/sprite_overflow_tests/1.Basics.nes", P, -1, "SPRITE OVERFLOW BASICS\nPASSED"},
		{"sprite overflow details", "roms/sprite_overflow_tests/2.Details.nes", P, -1, "SPRITE OVERFLOW DETAILS\nPASSED"},
		{"sprite overflow timing", "roms/sprite_overflow_tests/3.Timing.nes", P, -1, "SPRITE OVERFLOW TIMING\nPASSED"},
		{"sprite overflow obscure", "roms/sprite_overflow_tests/4.Obscure.nes", P, -1, "SPRITE OVERFLOW OBSCURE\nPASSED"},
		{"sprite overflow emulator", "roms/sprite_overflow_tests/5.Emulator.nes", P, -1, "SPRITE OVERFLOW EMULATION\nPASSED"},

		{"sprite hit basics", "roms/sprite_hit_tests_2005.10.05/01.basics.nes", P, -1, "SPRITE HIT BASICS\nPASSED"},
		{"sprite hit alignment", "roms/sprite_hit_tests_2005.10.05/02.alignment.nes", P, -1, "SPRITE HIT ALIGNMENT\nPASSED"},
		{"sprite hit corners", "roms/sprite_hit_tests_2005.10.05/03.corners.nes", P, -1, "SPRITE HIT CORNERS\nPASSED"},
		{"sprite hit flip", "roms/sprite_hit_tests_2005.10.05/04.flip.nes", P, -1, "SPRITE HIT FLIPPING\nPASSED"},
		{"sprite hit left clip", "roms/sprite_hit_tests_2005.10.05/05.left_clip.nes", P, -1, "SPRITE HIT LEFT CLIPPING\nPASSED"},
		{"sprite hit right edge", "roms/sprite_hit_tests_2005.10.05/06.right_edge.nes", P, -1, "SPRITE HIT RIGHT EDGE\nPASSED"},
		{"sprite hit screen bottom", "roms/sprite_hit_tests_2005.10.05/07.screen_bottom.nes", P, -1, "SPRITE HIT SCREEN BOTTOM\nPASSED"},
		{"sprite hit double height", "roms/sprite_hit_tests_2005.10.05/08.double_height.nes", P, -1, "SPRITE HIT DOUBLE HEIGHT\nPASSED"},
		{"sprite hit timing basics", "roms/sprite_hit_tests_2005.10.05/09.timing_basics.nes", P, -1, "SPRITE HIT TIMING\nPASSED"},
		{"sprite hit timing order", "roms/sprite_hit_tests_2005.10.05/10.timing_order.nes", P, -1, "SPRITE HIT ORDER\nPASSED"},
		{"sprite hit edge timing", "roms/sprite_hit_tests_2005.10.05/11.edge_timing.nes", P, -1, "SPRITE HIT EDGE TIMING\nPASSED"},
	})

	runFrameTable(t, []frameCase{
		{"palette RAM", "roms/blargg_ppu_tests_2005.09.15b/palette_ram.nes", 30, "$01"},
		{"sprite RAM", "roms/blargg_ppu_tests_2005.09.15b/sprite_ram.nes", 30, "$01"},
		{"vblank clear time", "roms/blargg_ppu_tests_2005.09.15b/vbl_clear_time.nes", 40, "$01"},
		{"vram access", "roms/blargg_ppu_tests_2005.09.15b/vram_access.nes", 30, "$01"},
	})
}
