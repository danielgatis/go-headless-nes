// Package apu emulates the 2A03's audio unit: two pulse channels, a
// triangle, a noise channel, the DMC sample player, and the frame
// counter that paces their envelopes and length counters.
//
// The APU is ticked once per CPU cycle and produces mixed samples at a
// fixed output rate into an internal ring, which the UI driver drains
// once per frame. Emulation never depends on how fast audio is
// consumed, so sound cannot break determinism.
//
// Known approximation: on hardware a length-counter reload
// ($4003/$4007/$400B/$400F) landing on the same CPU cycle as a
// half-frame length clock is ignored while the counter is nonzero.
// The machine steps the CPU a whole instruction ahead of the APU, so
// the frame counter's phase at write time lags the true write cycle
// by up to the instruction length and the collision cannot be
// detected; reloads always take effect here.
package apu

import (
	"github.com/danielgatis/go-headless-nes/internal/bus"
	"github.com/danielgatis/go-headless-nes/internal/region"
)

// SampleRate is the audio device's playback rate.
const SampleRate = 44100

// The emit rate is how many samples the APU produces per emulated second
// (Params.EmitRate). It is deliberately higher than SampleRate: UI
// drivers run one emulated frame per display refresh, but an emulated
// second is slightly more than the refresh count of frames (NTSC 60.0988
// vs 60), so emitting proportionally more samples makes production match
// the device's real-time consumption, keeping the delivery buffer
// balanced ("sync to video"); the equivalent sub-percent pitch shift is
// far below audibility.

// sampleRingSize must hold a few frames of samples (735 per frame)
// between frontend drains.
const sampleRingSize = 8192

// State is the APU's complete mutable state, copyable by assignment.
type State struct {
	Pulse1, Pulse2 Pulse
	Tri            Triangle
	Noise          Noise
	DMC            DMC
	Frame          frameCounter

	FrameIRQ bool // frame-counter IRQ line level (from Frame.irqFlag)

	// run model: currentCycle is the APU's position within the frame
	// window; previousCycle is how far the channels have been advanced.
	CurrentCycle  uint32
	PreviousCycle uint32
	NeedRun       bool

	CPUCycle uint64 // running CPU cycle count, for odd/even write timing

	SampleAcc int // fractional resampling accumulator
}

// apuCycleLength is the frame window after which the APU flushes audio and
// resets its cycle counters.
const apuCycleLength = 10000

// APU couples the copyable State with its wiring, the band-limited
// synthesis accumulator, and the sample ring.
type APU struct {
	State

	// requestDMA / stopDMA drive the DMC's transfers through the CPU's DMA
	// unit. The machine installs them; the CPU delivers fetched bytes back
	// via DMCDeliver.
	requestDMA func()
	stopDMA    func()

	// clock returns the shared master clock, used to time the brief window
	// the frame-counter IRQ flag is held before it self-clears. The console
	// installs it; nil means clock 0 (bare unit tests).
	clock func() uint64

	// internalOpenBus returns the CPU's internal open-bus latch; bit 5 of a
	// $4015 read is that latch's bit 5 (the register does not drive it).
	internalOpenBus func() byte

	// external returns the cartridge's expansion-audio level, mixed on
	// top of the 2A03 channels; nil when the board has none.
	external func() float32

	synth synth

	// Per-region audio rates and the resolved region (for re-seating the
	// channel table pointers after a snapshot restore). Not part of State:
	// the region is fixed for a machine and never enters a snapshot.
	region   region.Region
	cpuHz    int
	emitRate int

	samples [sampleRingSize]float32
	sampleR int
	sampleW int
}

// masterClock returns the current master clock, or 0 if none is wired.
func (a *APU) masterClock() uint64 {
	if a.clock != nil {
		return a.clock()
	}
	return 0
}

// SetClock installs the master-clock source for the frame-IRQ self-clear.
func (a *APU) SetClock(clock func() uint64) { a.clock = clock }

// SetInternalOpenBus installs the CPU's internal open-bus latch reader
// (bit 5 of a $4015 read comes from it).
func (a *APU) SetInternalOpenBus(f func() byte) { a.internalOpenBus = f }

// SetExternalAudio installs the cartridge's expansion-audio level source.
func (a *APU) SetExternalAudio(f func() float32) { a.external = f }

// SetDMAHooks wires the DMC's transfers through the CPU's DMA unit.
func (a *APU) SetDMAHooks(request, stop func()) {
	a.requestDMA = request
	a.stopDMA = stop
	a.DMC.requestDMA = request
	a.DMC.stopDMA = stop
}

// New returns a powered-on APU. requestDMA asks the CPU's DMA unit to
// fetch the next DMC sample byte (delivered back via DMCDeliver); stopDMA
// aborts a queued DMC transfer.
func New(requestDMA, stopDMA func()) *APU {
	a := &APU{requestDMA: requestDMA, stopDMA: stopDMA}
	a.Pulse1.IsChannel1 = true
	a.DMC.requestDMA = requestDMA
	a.DMC.stopDMA = stopDMA
	// Default to NTSC so the tables are seated before the reset seeds the
	// noise/DMC periods from them; Configure overrides for other regions.
	a.Configure(region.ParamsFor(region.NTSC))
	a.reset(false)
	return a
}

// Configure applies a region's audio rates and installs the matching
// channel tables. Call it before the reset that seeds channel periods.
func (a *APU) Configure(p region.Params) {
	a.cpuHz = p.CPUHz
	a.emitRate = p.EmitRate
	a.installRegionTables(p.Region)
}

// installRegionTables re-seats the channel table pointers for the region.
// It runs at configuration time and again after a snapshot restore, which
// overwrites the channel structs (and their pointers) wholesale.
func (a *APU) installRegionTables(r region.Region) {
	a.region = r
	if r == region.PAL {
		a.Frame.stepCycles = &frameStepCyclesPAL
		a.Noise.periodTable = &noisePeriodTablePAL
		a.DMC.periodTable = &dmcPeriodTablePAL
		return
	}
	// NTSC and Dendy share the NTSC APU tables.
	a.Frame.stepCycles = &frameStepCyclesNTSC
	a.Noise.periodTable = &noisePeriodTableNTSC
	a.DMC.periodTable = &dmcPeriodTableNTSC
}

// ReinstallRegionTables re-seats the channel table pointers after a
// snapshot restore replaced the APU State (and its table pointers).
func (a *APU) ReinstallRegionTables() { a.installRegionTables(a.region) }

// Reset models the console's reset line reaching the APU (soft reset).
func (a *APU) Reset() { a.reset(true) }

func (a *APU) reset(softReset bool) {
	a.CurrentCycle = 0
	a.PreviousCycle = 0
	a.Pulse1.reset(softReset)
	a.Pulse2.reset(softReset)
	a.Tri.reset(softReset)
	a.Noise.reset(softReset)
	a.DMC.reset(softReset)
	a.Frame.reset(softReset)
}

// DMCReadAddr is the CPU DMA unit's hook for the next DMC fetch address.
func (a *APU) DMCReadAddr() uint16 { return a.DMC.ReadAddr() }

// DMCDeliver hands a fetched DMC sample byte back to the channel.
func (a *APU) DMCDeliver(v byte) { a.DMC.Deliver(v) }

// Tick advances the APU by one CPU cycle.
func (a *APU) Tick() {
	a.CPUCycle++
	a.CurrentCycle++
	if a.CurrentCycle == apuCycleLength-1 {
		a.DMC.processClock()
		a.endFrame()
	} else {
		// the APU runs the channels lazily (batching per-period expiries) and
		// mixes with timestamped deltas. This emulator instead samples the mixed
		// level every CPU cycle in synthTick, so the channel outputs must be
		// current on every cycle. needToRun is still called for its
		// DMC/frame-counter side effects (it advances the DMC delay counters
		// and consumes NeedRun); its result is discarded because we run every
		// cycle regardless, keeping the synth's waveform up to date.
		a.needToRun(a.CurrentCycle)
		a.run()
	}

	// Band-limited synthesis samples the mixed level every CPU cycle.
	a.synthTick()
}

// needToRun reports whether the channels must be advanced this cycle.
func (a *APU) needToRun(currentCycle uint32) bool {
	if a.DMC.needToRun() || a.NeedRun {
		a.NeedRun = false
		return true
	}
	cyclesToRun := int32(currentCycle - a.PreviousCycle)
	return a.Frame.needToRun(cyclesToRun)
}

// run advances the frame counter and all channels up to CurrentCycle.
func (a *APU) run() {
	cyclesToRun := int32(a.CurrentCycle) - int32(a.PreviousCycle)
	for cyclesToRun > 0 {
		a.PreviousCycle += a.Frame.run(&cyclesToRun, a.frameCounterTick, a.setFrameIRQ)

		// Reload length counters after running the frame counter, so a
		// same-cycle half-frame clock ticks first (len_reload_timing).
		a.Pulse1.reloadLength()
		a.Pulse2.reloadLength()
		a.Noise.reloadLength()
		a.Tri.reloadLength()

		a.Pulse1.run(a.PreviousCycle)
		a.Pulse2.run(a.PreviousCycle)
		a.Noise.run(a.PreviousCycle)
		a.Tri.run(a.PreviousCycle)
		a.DMC.run(a.PreviousCycle)
	}
}

// endFrame flushes the APU frame: run to the boundary, reset per-frame
// cycle bookkeeping. Audio output is handled by
// the per-cycle synth, so no buffer is played here.
func (a *APU) endFrame() {
	a.run()
	a.Pulse1.endFrame()
	a.Pulse2.endFrame()
	a.Tri.endFrame()
	a.Noise.endFrame()
	a.DMC.endFrame()
	a.CurrentCycle = 0
	a.PreviousCycle = 0
}

// frameCounterTick clocks the channels' envelope/linear/length/sweep units.
func (a *APU) frameCounterTick(t frameType) {
	a.Pulse1.tickEnvelope()
	a.Pulse2.tickEnvelope()
	a.Tri.tickLinearCounter()
	a.Noise.tickEnvelope()

	if t == frameHalf {
		a.Pulse1.tickLengthCounter()
		a.Pulse2.tickLengthCounter()
		a.Tri.tickLengthCounter()
		a.Noise.tickLengthCounter()

		a.Pulse1.tickSweep()
		a.Pulse2.tickSweep()
	}
}

func (a *APU) setFrameIRQ(bool) { a.FrameIRQ = true }

// IRQ reports the level of the APU's interrupt line. It is the wired-OR of
// two interrupt sources: the frame counter's IRQ source (raised while the
// 4-step sequence holds it and not inhibited, cleared by a $4015 read or an
// inhibit write) and the DMC's IRQ. This is separate from the $4015 status
// flag, which has its own brief self-clearing window (see getIrqFlag).
func (a *APU) IRQ() bool {
	return a.FrameIRQ || a.DMC.IRQ
}

// Ranges maps the APU into the CPU address space: it takes writes to
// $4000-$4013, $4015 and $4017, and the $4015 status read. ($4014 is OAM
// DMA and $4016/$4017 reads are the controllers — those are other handlers.)
func (a *APU) Ranges() *bus.Ranges {
	r := bus.NewRanges()
	r.Add(bus.OpWrite, 0x4000, 0x4013)
	r.AddOne(bus.OpWrite, 0x4015)
	r.AddOne(bus.OpWrite, 0x4017)
	r.AddOne(bus.OpRead, 0x4015)
	return r
}

// ReadReg services a CPU read of an APU register ($4015 status).
func (a *APU) ReadReg(addr uint16) byte {
	if addr == 0x4015 {
		// Bit 5 of a $4015 read is open bus (the internal latch), not driven
		// by the register.
		return a.ReadStatus() | (a.internalOpenBusBits() & 0x20)
	}
	return 0
}

// internalOpenBusBits returns the CPU's internal open-bus latch, or 0 if no
// source is wired (bare unit tests).
func (a *APU) internalOpenBusBits() byte {
	if a.internalOpenBus != nil {
		return a.internalOpenBus()
	}
	return 0
}

// WriteReg services a CPU write to an APU register.
func (a *APU) WriteReg(addr uint16, v byte) { a.WriteRegister(addr, v) }

// PeekReg reads an APU register without side effects (for debuggers).
func (a *APU) PeekReg(addr uint16) byte {
	if addr == 0x4015 {
		return a.PeekStatus()
	}
	return 0
}

// WriteRegister services CPU writes to $4000-$4017 (except $4014,
// which is OAM DMA and belongs to the bus).
func (a *APU) WriteRegister(addr uint16, v byte) {
	// the APU runs the APU up to the current cycle before every register
	// write so the change lands at the right sub-frame position.
	a.run()
	switch addr {
	case 0x4000:
		a.Pulse1.writeControl(v)
	case 0x4001:
		a.Pulse1.writeSweep(v)
	case 0x4002:
		a.Pulse1.writeTimerLow(v)
	case 0x4003:
		a.Pulse1.writeTimerHigh(v)
	case 0x4004:
		a.Pulse2.writeControl(v)
	case 0x4005:
		a.Pulse2.writeSweep(v)
	case 0x4006:
		a.Pulse2.writeTimerLow(v)
	case 0x4007:
		a.Pulse2.writeTimerHigh(v)
	case 0x4008:
		a.Tri.writeControl(v)
	case 0x400A:
		a.Tri.writeTimerLow(v)
	case 0x400B:
		a.Tri.writeTimerHigh(v)
	case 0x400C:
		a.Noise.writeControl(v)
	case 0x400E:
		a.Noise.writePeriod(v)
	case 0x400F:
		a.Noise.writeLength(v)
	case 0x4010:
		a.DMC.writeControl(v)
	case 0x4011:
		a.DMC.writeLevel(v)
	case 0x4012:
		a.DMC.writeAddr(v)
	case 0x4013:
		a.DMC.writeLength(v)
	case 0x4015:
		a.writeStatus(v)
	case 0x4017:
		a.writeFrameCounter(v)
	}
}

// oddCycle reports the parity of the CPU cycle count, which the $4017 and
// $4015 write timing depends on (the write's effect lands 3 or 4 CPU cycles
// later depending on whether it fell between or during an APU cycle).
func (a *APU) oddCycle() bool { return a.masterClock()&0x01 == 1 }

func (a *APU) writeStatus(v byte) {
	a.run()
	// Clear the DMC IRQ before enabling (enabling can raise a new one).
	a.DMC.IRQ = false
	a.Pulse1.setEnabled(v&0x01 != 0)
	a.Pulse2.setEnabled(v&0x02 != 0)
	a.Tri.setEnabled(v&0x04 != 0)
	a.Noise.setEnabled(v&0x08 != 0)
	a.DMC.setEnabled(v&0x10 != 0, a.oddCycle())
}

func (a *APU) writeFrameCounter(v byte) {
	a.run()
	a.Frame.write(v, a.oddCycle(), func() { a.FrameIRQ = false })
}

// ReadStatus services $4015 reads: channel activity flags plus the two
// IRQ flags. The status bit comes from getIrqFlag (which arms its own brief
// self-clear); the read also clears the frame-counter IRQ *source* (the
// interrupt line). The DMC IRQ is untouched.
func (a *APU) ReadStatus() byte {
	a.run()
	v := a.statusBits(a.Frame.getIrqFlag(a.masterClock()))
	// Reading clears the frame-counter IRQ *source* (the interrupt line); the
	// status flag is left to getIrqFlag's own brief self-clear, matching the
	// reference (it does not zero _irqFlag here).
	a.FrameIRQ = false
	return v
}

// PeekStatus is ReadStatus without side effects.
func (a *APU) PeekStatus() byte { return a.statusBits(a.Frame.peekIrqFlag(a.masterClock())) }

func (a *APU) statusBits(frameIRQ bool) byte {
	var v byte
	if a.Pulse1.status() {
		v |= 0x01
	}
	if a.Pulse2.status() {
		v |= 0x02
	}
	if a.Tri.status() {
		v |= 0x04
	}
	if a.Noise.status() {
		v |= 0x08
	}
	if a.DMC.status() {
		v |= 0x10
	}
	if frameIRQ {
		v |= 0x40
	}
	if a.DMC.IRQ {
		v |= 0x80
	}
	return v
}

// mix combines the five channels through the precomputed nonlinear DAC
// tables (see tables.go).
func (a *APU) mix() float32 {
	p := int(a.Pulse1.output()) + int(a.Pulse2.output())
	tnd := 3*int(a.Tri.output()) + 2*int(a.Noise.output()) + int(a.DMC.output())
	// The result is unipolar in [0, ~1], as the hardware DAC's is; the
	// console's output stage AC-couples away the DC and so does the
	// sound card. UI drivers scale to their device's range.
	return pulseTable[p] + tndTable[tnd]
}

func (a *APU) pushSample(s float32) {
	next := (a.sampleW + 1) % sampleRingSize
	if next == a.sampleR {
		return // frontend stalled; dropping is better than blocking
	}
	a.samples[a.sampleW] = s
	a.sampleW = next
}

// DrainSamples copies buffered samples into dst and returns how many
// were copied. The UI driver calls this once per frame.
func (a *APU) DrainSamples(dst []float32) int {
	n := 0
	for n < len(dst) && a.sampleR != a.sampleW {
		dst[n] = a.samples[a.sampleR]
		a.sampleR = (a.sampleR + 1) % sampleRingSize
		n++
	}
	return n
}
