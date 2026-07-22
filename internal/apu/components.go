package apu

// Shared APU building blocks (timer, length counter, envelope, frame
// counter). These are driven by the per-cycle model: each channel calls
// timer.run(currentCycle) to advance its sequencer, and mix() reads the
// resulting output level. The timer's "output" is the channel's current
// level, sampled by the band-limited synth rather than a delta mixer.

// lengthLookup is the APU length-counter load table.
var lengthLookup = [32]byte{
	10, 254, 20, 2, 40, 4, 80, 6, 160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22, 192, 24, 72, 26, 16, 28, 32, 30,
}

// apuTimer is a down-counter that reloads to period and reports a tick
// . run advances it toward targetCycle; each expiry
// returns true so the caller clocks its sequencer. previousCycle tracks
// how far the timer has been advanced within the current APU frame.
type apuTimer struct {
	previousCycle uint32
	timer         uint16
	period        uint16
	lastOutput    byte
}

// run advances the timer to targetCycle, returning true once per period
// expiry (the caller loops on it). This is the classic APU down-counter.
func (t *apuTimer) run(targetCycle uint32) bool {
	cyclesToRun := int32(targetCycle) - int32(t.previousCycle)
	if cyclesToRun > int32(t.timer) {
		t.previousCycle += uint32(t.timer) + 1
		t.timer = t.period
		return true
	}
	t.timer -= uint16(cyclesToRun)
	t.previousCycle = targetCycle
	return false
}

func (t *apuTimer) endFrame()             { t.previousCycle = 0 }
func (t *apuTimer) setPeriod(p uint16)    { t.period = p }
func (t *apuTimer) getPeriod() uint16     { return t.period }
func (t *apuTimer) getTimer() uint16      { return t.timer }
func (t *apuTimer) setTimer(v uint16)     { t.timer = v }
func (t *apuTimer) addOutput(output byte) { t.lastOutput = output }
func (t *apuTimer) getLastOutput() byte   { return t.lastOutput }

func (t *apuTimer) reset() {
	t.timer = 0
	t.period = 0
	t.previousCycle = 0
	t.lastOutput = 0
}

// lengthCounter is the APU length counter. It
// gates a channel's output and counts down on half-frame clocks.
type lengthCounter struct {
	isTriangle bool

	enabled       bool
	halt          bool
	newHaltValue  bool
	counter       byte
	reloadValue   byte
	previousValue byte
}

func (l *lengthCounter) initialize(haltFlag bool) { l.newHaltValue = haltFlag }

func (l *lengthCounter) load(value byte) {
	if l.enabled {
		l.reloadValue = lengthLookup[value]
		l.previousValue = l.counter
	}
}

func (l *lengthCounter) status() bool   { return l.counter > 0 }
func (l *lengthCounter) isHalted() bool { return l.halt }

// reload applies a pending length load after the frame counter clocked
// .
func (l *lengthCounter) reload() {
	if l.reloadValue != 0 {
		if l.counter == l.previousValue {
			l.counter = l.reloadValue
		}
		l.reloadValue = 0
	}
	l.halt = l.newHaltValue
}

func (l *lengthCounter) tick() {
	if l.counter > 0 && !l.halt {
		l.counter--
	}
}

func (l *lengthCounter) setEnabled(enabled bool) {
	if !enabled {
		l.counter = 0
	}
	l.enabled = enabled
}

func (l *lengthCounter) reset(softReset bool) {
	if softReset && l.isTriangle {
		// "At reset, length counters should be enabled, triangle unaffected."
		l.enabled = false
		return
	}
	l.enabled = false
	l.halt = false
	l.counter = 0
	l.newHaltValue = false
	l.reloadValue = 0
	l.previousValue = 0
}

// envelope is the APU volume envelope, which embeds a
// length counter (matching the composition).
type envelope struct {
	length lengthCounter

	constantVolume bool
	volume         byte
	start          bool
	divider        int8
	counter        byte
}

func (e *envelope) initialize(regValue byte) {
	e.length.initialize(regValue&0x20 == 0x20)
	e.constantVolume = regValue&0x10 == 0x10
	e.volume = regValue & 0x0F
}

func (e *envelope) resetEnvelope() { e.start = true }

func (e *envelope) getVolume() byte {
	if e.length.status() {
		if e.constantVolume {
			return e.volume
		}
		return e.counter
	}
	return 0
}

func (e *envelope) tick() {
	if !e.start {
		e.divider--
		if e.divider < 0 {
			e.divider = int8(e.volume)
			if e.counter > 0 {
				e.counter--
			} else if e.length.isHalted() {
				e.counter = 15
			}
		}
	} else {
		e.start = false
		e.counter = 15
		e.divider = int8(e.volume)
	}
}

func (e *envelope) reset(softReset bool) {
	e.length.reset(softReset)
	e.constantVolume = false
	e.volume = 0
	e.start = false
	e.divider = 0
	e.counter = 0
}

// frameType is what a frame-counter step clocks.
type frameType byte

const (
	frameNone frameType = iota
	frameQuarter
	frameHalf
)

// NTSC frame-counter step cycles and types.
// Index [stepMode][step]; stepMode 0 = 4-step, 1 = 5-step.
var frameStepCycles = [2][6]int32{
	{7457, 14913, 22371, 29828, 29829, 29830},
	{7457, 14913, 22371, 29829, 37281, 37282},
}

var frameStepType = [6]frameType{
	frameQuarter, frameHalf, frameQuarter, frameNone, frameHalf, frameNone,
}

// frameCounter is the APU frame sequencer. It runs
// in bulk via run(cyclesToRun) and calls back into the APU to clock the
// channels' envelope/length/sweep units. The write delay, IRQ assertion
// and $4017 side effects are reproduced.
type frameCounter struct {
	previousCycle int32
	currentStep   uint32
	stepMode      uint32 // 0: 4-step, 1: 5-step
	inhibitIRQ    bool
	blockTick     byte
	newValue      int16 // pending $4017 value, -1 when none
	writeDelay    int8

	irqFlag           bool
	irqFlagClearClock uint64 // master clock at which a polled IRQ flag self-clears
}

// getIrqFlag reports the frame IRQ line. The 4-step IRQ flag is held only
// briefly: once polled while set, it is scheduled to clear at the start of
// the next APU cycle. clock is the shared master clock.
func (f *frameCounter) getIrqFlag(clock uint64) bool {
	if f.irqFlag {
		if f.irqFlagClearClock == 0 {
			if clock&1 != 0 {
				f.irqFlagClearClock = clock + 2
			} else {
				f.irqFlagClearClock = clock + 1
			}
		} else if clock >= f.irqFlagClearClock {
			f.irqFlagClearClock = 0
			f.irqFlag = false
		}
	}
	return f.irqFlag
}

// peekIrqFlag is getIrqFlag without the self-clear side effect.
func (f *frameCounter) peekIrqFlag(clock uint64) bool {
	if f.irqFlag && f.irqFlagClearClock != 0 && clock >= f.irqFlagClearClock {
		return false
	}
	return f.irqFlag
}

func (f *frameCounter) reset(softReset bool) {
	f.previousCycle = 0
	f.irqFlag = false
	if !softReset {
		f.stepMode = 0
	}
	f.currentStep = 0
	if f.stepMode != 0 {
		f.newValue = 0x80
	} else {
		f.newValue = 0x00
	}
	f.writeDelay = 3
	f.inhibitIRQ = false
	f.blockTick = 0
}

// run advances the frame counter by up to cyclesToRun, clocking a frame
// step if one lands, and returns how many cycles it consumed. tick is the
// APU's FrameCounterTick callback; setIRQ raises the frame IRQ line.
func (f *frameCounter) run(cyclesToRun *int32, tick func(frameType), setIRQ func(bool)) uint32 {
	var cyclesRan uint32

	if f.previousCycle+*cyclesToRun >= frameStepCycles[f.stepMode][f.currentStep] {
		if f.stepMode == 0 && f.currentStep >= 3 {
			// Set the IRQ status flag on the last 3 cycles in 4-step mode.
			// Resetting the clear clock here is essential: the flag re-asserts
			// every frame, so its self-clear window must re-arm each time.
			f.irqFlag = true
			f.irqFlagClearClock = 0
			if !f.inhibitIRQ {
				setIRQ(true)
			} else if f.currentStep == 5 {
				f.irqFlag = false
				f.irqFlagClearClock = 0
			}
		}

		ft := frameStepType[f.currentStep]
		if ft != frameNone && f.blockTick == 0 {
			tick(ft)
			f.blockTick = 2
		}

		if frameStepCycles[f.stepMode][f.currentStep] < f.previousCycle {
			cyclesRan = 0
		} else {
			cyclesRan = uint32(frameStepCycles[f.stepMode][f.currentStep] - f.previousCycle)
		}
		*cyclesToRun -= int32(cyclesRan)

		f.currentStep++
		if f.currentStep == 6 {
			f.currentStep = 0
			f.previousCycle = 0
		} else {
			f.previousCycle += int32(cyclesRan)
		}
	} else {
		cyclesRan = uint32(*cyclesToRun)
		*cyclesToRun = 0
		f.previousCycle += int32(cyclesRan)
	}

	if f.newValue >= 0 {
		f.writeDelay--
		if f.writeDelay == 0 {
			if f.newValue&0x80 == 0x80 {
				f.stepMode = 1
			} else {
				f.stepMode = 0
			}
			f.writeDelay = -1
			f.currentStep = 0
			f.previousCycle = 0
			f.newValue = -1
			if f.stepMode != 0 && f.blockTick == 0 {
				// A bit-7 write immediately clocks quarter+half units.
				tick(frameHalf)
				f.blockTick = 2
			}
		}
	}

	if f.blockTick > 0 {
		f.blockTick--
	}
	return cyclesRan
}

// needToRun reports whether the frame counter must run this cycle rather
// than being deferred.
func (f *frameCounter) needToRun(cyclesToRun int32) bool {
	return f.newValue >= 0 || f.blockTick > 0 ||
		f.previousCycle+cyclesToRun >= frameStepCycles[f.stepMode][f.currentStep]-1
}

// write handles a $4017 write, with the
// odd/even write delay. clearIRQ clears the frame IRQ source on inhibit.
func (f *frameCounter) write(value byte, oddCycle bool, clearIRQ func()) {
	f.newValue = int16(value)
	if oddCycle {
		f.writeDelay = 4
	} else {
		f.writeDelay = 3
	}
	f.inhibitIRQ = value&0x40 == 0x40
	if f.inhibitIRQ {
		clearIRQ()
		f.irqFlag = false
		f.irqFlagClearClock = 0
	}
}
