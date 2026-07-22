package apu

// Pulse is a square-wave channel.h.
// It advances its timer via run(targetCycle); the sequencer steps on each
// timer expiry and updateOutput records the level into the timer's output,
// which mix() samples. Sweep, envelope and length match hardware exactly.
type Pulse struct {
	Env envelope
	tmr apuTimer

	IsChannel1 bool

	Duty    byte
	DutyPos byte

	SweepEnabled bool
	SweepPeriod  byte
	SweepNegate  bool
	SweepShift   byte
	ReloadSweep  bool
	SweepDivider byte
	SweepTarget  uint32
	RealPeriod   uint16
}

var dutySequences = [4][8]byte{
	{0, 0, 0, 0, 0, 0, 0, 1},
	{0, 0, 0, 0, 0, 0, 1, 1},
	{0, 0, 0, 0, 1, 1, 1, 1},
	{1, 1, 1, 1, 1, 1, 0, 0},
}

// isMuted: period < 8, or a non-negating sweep whose target overflows.
func (p *Pulse) isMuted() bool {
	return p.RealPeriod < 8 || (!p.SweepNegate && p.SweepTarget > 0x7FF)
}

func (p *Pulse) initializeSweep(v byte) {
	p.SweepEnabled = v&0x80 == 0x80
	p.SweepNegate = v&0x08 == 0x08
	p.SweepPeriod = (v&0x70)>>4 + 1
	p.SweepShift = v & 0x07
	p.updateTargetPeriod()
	p.ReloadSweep = true
}

func (p *Pulse) updateTargetPeriod() {
	shiftResult := p.RealPeriod >> p.SweepShift
	if p.SweepNegate {
		p.SweepTarget = uint32(p.RealPeriod - shiftResult)
		if p.IsChannel1 {
			// Pulse 1's negative sweep subtracts one extra (ones' complement).
			p.SweepTarget--
		}
	} else {
		p.SweepTarget = uint32(p.RealPeriod + shiftResult)
	}
}

func (p *Pulse) setPeriod(newPeriod uint16) {
	p.RealPeriod = newPeriod
	p.tmr.setPeriod(p.RealPeriod*2 + 1)
	p.updateTargetPeriod()
}

func (p *Pulse) updateOutput() {
	if p.isMuted() {
		p.tmr.addOutput(0)
	} else {
		p.tmr.addOutput(dutySequences[p.Duty][p.DutyPos] * p.Env.getVolume())
	}
}

// run advances the sequencer to targetCycle.
func (p *Pulse) run(targetCycle uint32) {
	for p.tmr.run(targetCycle) {
		p.DutyPos = (p.DutyPos - 1) & 0x07
		p.updateOutput()
	}
}

func (p *Pulse) reset(softReset bool) {
	p.Env.reset(softReset)
	p.tmr.reset()
	p.Duty = 0
	p.DutyPos = 0
	p.RealPeriod = 0
	p.SweepEnabled = false
	p.SweepPeriod = 0
	p.SweepNegate = false
	p.SweepShift = 0
	p.ReloadSweep = false
	p.SweepDivider = 0
	p.SweepTarget = 0
	p.updateTargetPeriod()
}

// register writes ($4000-$4003 / $4004-$4007).
func (p *Pulse) writeControl(v byte) {
	p.Env.initialize(v)
	p.Duty = (v & 0xC0) >> 6
	p.updateOutput()
}

func (p *Pulse) writeSweep(v byte) {
	p.initializeSweep(v)
	p.updateOutput()
}

func (p *Pulse) writeTimerLow(v byte) {
	p.setPeriod(p.RealPeriod&0x0700 | uint16(v))
	p.updateOutput()
}

func (p *Pulse) writeTimerHigh(v byte) {
	p.Env.length.load(v >> 3)
	p.setPeriod(p.RealPeriod&0xFF | uint16(v&0x07)<<8)
	p.DutyPos = 0
	p.Env.resetEnvelope()
	p.updateOutput()
}

func (p *Pulse) tickSweep() {
	p.SweepDivider--
	if p.SweepDivider == 0 {
		if p.SweepShift > 0 && p.SweepEnabled && p.RealPeriod >= 8 && p.SweepTarget <= 0x7FF {
			p.setPeriod(uint16(p.SweepTarget))
		}
		p.SweepDivider = p.SweepPeriod
	}
	if p.ReloadSweep {
		p.SweepDivider = p.SweepPeriod
		p.ReloadSweep = false
	}
}

func (p *Pulse) tickEnvelope()      { p.Env.tick() }
func (p *Pulse) tickLengthCounter() { p.Env.length.tick() }
func (p *Pulse) reloadLength()      { p.Env.length.reload() }
func (p *Pulse) endFrame()          { p.tmr.endFrame() }
func (p *Pulse) setEnabled(e bool)  { p.Env.length.setEnabled(e) }
func (p *Pulse) status() bool       { return p.Env.length.status() }
func (p *Pulse) output() byte       { return p.tmr.getLastOutput() }
