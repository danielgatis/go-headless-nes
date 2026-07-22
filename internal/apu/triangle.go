package apu

// Triangle channel.h. Its timer
// steps a 32-entry sequence, gated by both the length counter and the
// linear counter being nonzero.
type Triangle struct {
	Len lengthCounter
	tmr apuTimer

	LinearCounter       byte
	LinearCounterReload byte
	LinearReloadFlag    bool
	LinearControlFlag   bool

	SequencePosition byte
}

var triangleSequence = [32]byte{
	15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
}

// run advances the sequencer to targetCycle.
func (t *Triangle) run(targetCycle uint32) {
	for t.tmr.run(targetCycle) {
		if t.Len.status() && t.LinearCounter > 0 {
			t.SequencePosition = (t.SequencePosition + 1) & 0x1F
			t.tmr.addOutput(triangleSequence[t.SequencePosition])
		}
	}
}

func (t *Triangle) reset(softReset bool) {
	t.tmr.reset()
	t.Len.isTriangle = true
	t.Len.reset(softReset)
	t.LinearCounter = 0
	t.LinearCounterReload = 0
	t.LinearReloadFlag = false
	t.LinearControlFlag = false
	t.SequencePosition = 0
}

// register writes ($4008/$400A/$400B).
func (t *Triangle) writeControl(v byte) {
	t.LinearControlFlag = v&0x80 == 0x80
	t.LinearCounterReload = v & 0x7F
	t.Len.initialize(t.LinearControlFlag)
}

func (t *Triangle) writeTimerLow(v byte) {
	t.tmr.setPeriod(t.tmr.getPeriod()&0xFF00 | uint16(v))
}

func (t *Triangle) writeTimerHigh(v byte) {
	t.Len.load(v >> 3)
	t.tmr.setPeriod(t.tmr.getPeriod()&0xFF | uint16(v&0x07)<<8)
	t.LinearReloadFlag = true
}

func (t *Triangle) tickLinearCounter() {
	if t.LinearReloadFlag {
		t.LinearCounter = t.LinearCounterReload
	} else if t.LinearCounter > 0 {
		t.LinearCounter--
	}
	if !t.LinearControlFlag {
		t.LinearReloadFlag = false
	}
}

func (t *Triangle) tickLengthCounter() { t.Len.tick() }
func (t *Triangle) reloadLength()      { t.Len.reload() }
func (t *Triangle) endFrame()          { t.tmr.endFrame() }
func (t *Triangle) setEnabled(e bool)  { t.Len.setEnabled(e) }
func (t *Triangle) status() bool       { return t.Len.status() }
func (t *Triangle) output() byte       { return t.tmr.getLastOutput() }
