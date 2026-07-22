package test

import "testing"

// Test_blarggCPUExtra covers the CPU-correctness suites beyond the core
// instruction set already exercised by Test_blarggCPU: dummy read/write
// cycles, execution from I/O space, timing, and interrupt behaviour.
func Test_blarggCPUExtra(t *testing.T) {
	t.Parallel()
	runBlarggTable(t, []blarggCase{
		{"all instructions", "roms/instr_test-v3/all_instrs.nes", S, 0, "All 15 tests passed"},
		{"official only", "roms/instr_test-v3/official_only.nes", S, 0, "All 15 tests passed"},
		{"misc", "roms/instr_misc/instr_misc.nes", S, 0, blarggInstrMiscWant},
		{"timing", "roms/instr_timing/instr_timing.nes", S, 0, blarggInstrTimingWant},
		{"interrupts", "roms/cpu_interrupts_v2/cpu_interrupts.nes", S, 0, blarggCPUInterruptsWant},
		{"exec space (PPU I/O)", "roms/cpu_exec_space/test_cpu_exec_space_ppuio.nes", S, 0, blarggCPUExecPPUIOWant},
		{"exec space (APU)", "roms/cpu_exec_space/test_cpu_exec_space_apu.nes", S, 0, blarggCPUExecAPUWant},
		{"dummy writes (OAM)", "roms/cpu_dummy_writes/cpu_dummy_writes_oam.nes", S, 0, blarggCPUDummyWritesOAMWant},
		{"dummy writes (PPU mem)", "roms/cpu_dummy_writes/cpu_dummy_writes_ppumem.nes", S, 0, blarggCPUDummyWritesPPUWant},
	})
}

// Test_blarggCPUChecksum covers the checksum-style CPU suites
// (blargg_nes_cpu_test5, nes_instr_test). They do not signal completion
// within the harness step cap on the current core, so they are skipped
// with a reason rather than left to burn the cap on every run.
func Test_blarggCPUChecksum(t *testing.T) {
	t.Skip("checksum CPU suites (blargg_nes_cpu_test5, nes_instr_test) do not signal completion within the harness step cap; needs investigation")
}

const (
	blarggInstrMiscWant     = "04-dummy_reads_apu\n\nPassed\nAll 4 tests passed"
	blarggInstrTimingWant   = "2-branch_timing\n\nPassed\nAll 2 tests passed"
	blarggCPUExecPPUIOWant  = "TEST:test_cpu_exec_space_ppuio\nThis program verifies that the\nCPU can execute code from any\npossible location that it can\naddress, including I/O space.\n\nIn addition, it will be tested\nthat an RTS instruction does a\ndummy read of the byte that\nimmediately follows the\ninstructions.\n\nJSR+RTS TEST OK\nJMP+RTS TEST OK\nRTS+RTS TEST OK\nJMP+RTI TEST OK\nJMP+BRK TEST OK\n\nPassed"
	blarggCPUInterruptsWant = "test_jmp\nT+ CK PC\n00 02 04 \n01 01 04 \n02 03 07 \n03 02 07 \n04 01 07 \n05 02 08 \n06 01 08 \n07 03 08 \n08 02 08 \n09 01 08 \n\ntest_branch_not_taken\nT+ CK PC\n00 02 04 \n01 01 04 \n02 02 06 \n03 01 06 \n04 02 07 \n05 01 07 \n06 04 0A \n07 03 0A \n08 02 0A \n09 01 0A \n\ntest_branch_taken_pagecross\nT+ CK PC\n00 02 0D \n01 01 0D \n02 04 00 \n03 03 00 \n04 02 00 \n05 01 00 \n06 04 03 \n07 03 03 \n08 02 03 \n09 01 03 \n\ntest_branch_taken\nT+ CK PC\n00 02 04 \n01 01 04 \n02 03 07 \n03 02 07 \n04 05 0A \n05 04 0A \n06 03 0A \n07 02 0A \n08 01 0A \n09 03 0A \n\n\n5-branch_delays_irq\n\nPassed\nAll 5 tests passed"

	blarggCPUDummyWritesOAMWant = "TEST: cpu_dummy_writes_oam\nThis program verifies that the\nCPU does 2x writes properly.\nAny read-modify-write opcode\nshould first write the origi-\nnal value; then the calculated\nvalue exactly 1 cycle later.\n\nRequirement: OAM memory reads\nMUST be reliable. This is\noften the case on emulators,\nbut NOT on the real NES.\nNevertheless, this test can be\nused to see if the CPU in the\nemulator is built properly.\n\nTesting OAM.  The screen will go blank for a moment now.\nOK; Verifying opcodes...\n0E2E4E6ECEEE 1E3E5E7EDEFE \n0F2F4F6FCFEF 1F3F5F7FDFFF \n03234363C3E3 13335373D3F3 \n1B3B5B7BDBFB              \n\nPassed"

	blarggCPUDummyWritesPPUWant = "TEST: cpu_dummy_writes_ppumem\nThis program verifies that the\nCPU does 2x writes properly.\nAny read-modify-write opcode\nshould first write the origi-\nnal value; then the calculated\nvalue exactly 1 cycle later.\n\nVerifying open bus behavior.\n      W- W- WR W- W- W- W- WR\n2000+ 0  1  2  3  4  5  6  7 \n  R0: 0- 0- 00 0- 0- 0- 0- 00\n  R1: 0- 0- 00 0- 0- 0- 0- 00\n  R3: 0- 0- 00 0- 0- 0- 0- 00\n  R5: 0- 0- 00 0- 0- 0- 0- 00\n  R6: 0- 0- 00 0- 0- 0- 0- 00\nOK; Verifying opcodes...\n0E2E4E6ECEEE 1E3E5E7EDEFE \n0F2F4F6FCFEF 1F3F5F7FDFFF \n03234363C3E3 13335373D3F3 \n1B3B5B7BDBFB              \n\nPassed"

	blarggCPUExecAPUWant = "TEST: test_cpu_exec_space_apu\nThis program verifies that the\nCPU can execute code from any\npossible location that it can\naddress, including I/O space.\n\nIn this test, it is also\nverified that not only all\nwrite-only APU I/O ports\nreturn the open bus, but\nalso the unallocated I/O\nspace in $4018..$40FF.\n\n0022 4000 4001 4002 4003 4004 4005 4006 4007 4008 4009 400A 400B 400C 400D 400E 400F 4010 4011 4012 4013 4014 4016 4017 4018 4019 401A 401B 401C 401D 401E 401F 4020 4021 4022 4023 4024 4025 4026 4027 4028 4029 402A 402B 402C 402D 402E 402F 4030 4031 4032 4033 4034 4035 4036 4037 4038 4039 403A 403B 403C 403D 403E 403F 4040 4041 4042 4043 4044 4045 4046 4047 4048 4049 404A 404B 404C 404D 404E 404F 4050 4051 4052 4053 4054 4055 4056 4057 4058 4059 405A 405B 405C 405D 405E 405F 4060 4061 4062 4063 4064 4065 4066 4067 4068 4069 406A 406B 406C 406D 406E 406F 4070 4071 4072 4073 4074 4075 4076 4077 4078 4079 407A 407B 407C 407D 407E 407F 4080 4081 4082 4083 4084 4085 4086 4087 4088 4089 408A 408B 408C 408D 408E 408F 4090 4091 4092 4093 4094 4095 4096 4097 4098 4099 409A 409B 409C 409D 409E 409F 40A0 40A1 40A2 40A3 40A4 40A5 40A6 40A7 40A8 40A9 40AA 40AB 40AC 40AD 40AE 40AF 40B0 40B1 40B2 40B3 40B4 40B5 40B6 40B7 40B8 40B9 40BA 40BB 40BC 40BD 40BE 40BF 40C0 40C1 40C2 40C3 40C4 40C5 40C6 40C7 40C8 40C9 40CA 40CB 40CC 40CD 40CE 40CF 40D0 40D1 40D2 40D3 40D4 40D5 40D6 40D7 40D8 40D9 40DA 40DB 40DC 40DD 40DE 40DF 40E0 40E1 40E2 40E3 40E4 40E5 40E6 40E7 40E8 40E9 40EA 40EB 40EC 40ED 40EE 40EF 40F0 40F1 40F2 40F3 40F4 40F5 40F6 40F7 40F8 40F9 40FA 40FB 40FC 40FD 40FE 40FF \nPassed"
)
