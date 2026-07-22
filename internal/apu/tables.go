package apu

// Hardware mixer tables, from https://www.nesdev.org/wiki/APU
// (the per-channel length/duty/period tables live with each channel).

// pulseTable and tndTable precompute the mixer's nonlinear DAC curves
// (nesdev "APU Mixer"). pulseTable is indexed by the summed pulse
// outputs with the exact formula; tndTable is indexed by
// 3*triangle + 2*noise + dmc, the standard weighting that folds the
// three DAC input divisors into a single index.
var (
	pulseTable [31]float32
	tndTable   [203]float32
)

func init() {
	for i := 1; i < len(pulseTable); i++ {
		pulseTable[i] = float32(95.88 / (8128/float64(i) + 100))
	}
	for i := 1; i < len(tndTable); i++ {
		tndTable[i] = float32(163.67 / (24329/float64(i) + 100))
	}
}
