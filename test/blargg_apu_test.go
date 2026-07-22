package test

import "testing"

func Test_blarggAPU(t *testing.T) {
	t.Parallel()
	runBlarggTable(t, []blarggCase{
		{"len ctr", "roms/apu_test/rom_singles/1-len_ctr.nes", S, 0, "1-len_ctr\n\nPassed"},
		{"IRQ flag", "roms/apu_test/rom_singles/3-irq_flag.nes", S, 0, "3-irq_flag\n\nPassed"},
		{"DMC basics", "roms/apu_test/rom_singles/7-dmc_basics.nes", S, 0, "7-dmc_basics\n\nPassed"},
		{"reset clears $4015", "roms/apu_reset/4015_cleared.nes", S, 0, "4015_cleared\n\nPassed"},
		{"reset clears IRQ", "roms/apu_reset/irq_flag_cleared.nes", S, 0, "irq_flag_cleared\n\nPassed"},
	})
}
