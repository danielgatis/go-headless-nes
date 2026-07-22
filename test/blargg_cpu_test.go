package test

import "testing"

const blarggCPUTimingWant = `6502 TIMING TEST (16 SECONDS)
OFFICIAL INSTRUCTIONS ONLY
PASSED`

func Test_blarggCPU(t *testing.T) {
	t.Parallel()
	runBlarggTable(t, []blarggCase{
		{"instructions", "roms/instr_test-v5/all_instrs.nes", S, 0, "All 16 tests passed"},
		{"instruction timing", "roms/cpu_timing_test6/cpu_timing_test.nes", P, -1, blarggCPUTimingWant},
		{"branch timing basics", "roms/branch_timing_tests/1.Branch_Basics.nes", P, -1, "BRANCH TIMING BASICS\nPASSED"},
		{"branch timing backward", "roms/branch_timing_tests/2.Backward_Branch.nes", P, -1, "BACKWARD BRANCH TIMING\nPASSED"},
		{"branch timing forward", "roms/branch_timing_tests/3.Forward_Branch.nes", P, -1, "FORWARD BRANCH TIMING\nPASSED"},
		{"ram after reset", "roms/cpu_reset/ram_after_reset.nes", S, 0, "ram_after_reset\n\nPassed"},
		{"registers after reset", "roms/cpu_reset/registers.nes", S, 0, "A  X  Y  P  S\n34 56 78 FF 0F \n\nregisters\n\nPassed"},
	})
}
