package test

import "testing"

func Test_blarggOAM(t *testing.T) {
	t.Parallel()
	runBlarggTable(t, []blarggCase{
		{"read", "roms/oam_read/oam_read.nes", S, 0, blarggOAMReadWant},
		{"stress", "roms/oam_stress/oam_stress.nes", S, 0, blarggOAMStressWant},
	})
}

const blarggOAMReadWant = `----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------

oam_read

Passed`

const blarggOAMStressWant = `----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------
----------------

oam_stress

Passed`
