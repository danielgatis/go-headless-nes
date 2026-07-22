package apu

import "math"

// cpuHz is the NTSC CPU clock: the 21.477272 MHz master clock divided
// by 12.
const cpuHz = 21477272 / 12

// Band-limited synthesis, the technique behind blargg's blip_buf (as
// used by LaiNES and most accurate emulators). Sampling the mixer at
// output times folds the square waves' high harmonics back into the
// audible range as inharmonic aliasing; instead, every change of the
// mixed level enters the output as a band-limited step at its exact
// CPU-cycle position: a windowed-sinc kernel is accumulated into a
// small buffer of pending samples (as deltas), and an integrator
// reconstructs the waveform as samples are emitted.
const (
	blipHalfWidth = 8  // kernel reach in output samples on each side
	blipPhases    = 32 // sub-sample positions resolved by the table
	blipTaps      = 2 * blipHalfWidth

	// synthBufLen must exceed blipTaps; power of two for cheap wrap.
	synthBufLen = 64

	// integratorLeak bounds float rounding drift. It must be gentle —
	// a ~4.5 s time constant — because it sags the waveform baseline;
	// the delivery layer owns actual DC blocking.
	integratorLeak = 0.999995
)

// blipKernel[p][t] is the windowed-sinc impulse for a step landing at
// sub-sample phase p/blipPhases. Integrating it yields a band-limited
// step of exactly the delta's height (rows are normalized). Row
// blipPhases (a full sample of phase) is row 0 delayed one sample —
// the window is zero at |x| = blipHalfWidth, so the shifted row still
// fits in blipTaps taps — giving phase interpolation an upper
// endpoint. Built once; immutable afterwards.
var blipKernel = buildBlipKernel()

func buildBlipKernel() [blipPhases + 1][blipTaps]float32 {
	var kernel [blipPhases + 1][blipTaps]float32
	for p := range blipPhases + 1 {
		frac := float64(p) / blipPhases
		center := float64(blipHalfWidth-1) + frac
		var row [blipTaps]float64
		var sum float64
		for t := range blipTaps {
			x := float64(t) - center
			// sinc cut at the output Nyquist, under a Blackman window
			// spanning the kernel (zero at |x| = blipHalfWidth).
			sinc := 1.0
			if x != 0 {
				sinc = math.Sin(math.Pi*x) / (math.Pi * x)
			}
			w := 0.42 +
				0.5*math.Cos(math.Pi*x/blipHalfWidth) +
				0.08*math.Cos(2*math.Pi*x/blipHalfWidth)
			row[t] = sinc * w
			sum += row[t]
		}
		for t := range blipTaps {
			kernel[p][t] = float32(row[t] / sum)
		}
	}
	return kernel
}

// synth is the band-limited accumulator. Like the sample ring it is
// derived audio state, not part of State: after a rewind the next
// level change re-seats it.
type synth struct {
	buf        [synthBufLen]float32 // pending output samples, as deltas
	head       int
	integrator float64
	lastLevel  float32
}

// synthTick runs once per CPU cycle: record any change of the mixed
// level at this cycle's fractional output position, then advance the
// output clock, emitting finished samples.
func (a *APU) synthTick() {
	level := a.mix()
	if a.external != nil {
		level += a.external()
	}
	if level != a.synth.lastLevel {
		a.addDelta(level - a.synth.lastLevel)
		a.synth.lastLevel = level
	}
	a.SampleAcc += emitRate
	if a.SampleAcc >= cpuHz {
		a.SampleAcc -= cpuHz
		a.emitSample()
	}
}

// addDelta spreads a level change across the pending samples through
// the two kernel rows bracketing its sub-sample position, linearly
// interpolated; the rows are individually normalized, so the blend
// still integrates to exactly the delta's height. The kernel starts
// at the sample being assembled, so all audio is uniformly delayed by
// blipHalfWidth samples (0.2 ms) — pure latency, no distortion.
func (a *APU) addDelta(d float32) {
	pos := a.SampleAcc * blipPhases
	phase := pos / cpuHz
	frac := float32(pos-phase*cpuHz) / float32(cpuHz)
	lo, hi := &blipKernel[phase], &blipKernel[phase+1]
	for t := range blipTaps {
		k := lo[t] + frac*(hi[t]-lo[t])
		a.synth.buf[(a.synth.head+t)&(synthBufLen-1)] += d * k
	}
}

// emitSample integrates the next pending delta into the running level
// and pushes it to the sample ring.
func (a *APU) emitSample() {
	h := a.synth.head & (synthBufLen - 1)
	a.synth.integrator = a.synth.integrator*integratorLeak + float64(a.synth.buf[h])
	a.synth.buf[h] = 0
	a.synth.head++
	a.pushSample(float32(a.synth.integrator))
}
