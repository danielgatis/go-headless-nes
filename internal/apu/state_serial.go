package apu

import "github.com/danielgatis/go-headless-nes/internal/serial"

// This file gives every APU state type a leaf-first Append/Read pair so
// the whole State composes without reflection. The DMC's requestDMA and
// stopDMA function fields are deliberately not serialized: the machine
// re-installs them on Restore (see nes.Restore), so a snapshot never
// carries stale wiring. state_serial_test.go round-trips the State.

func (t *apuTimer) append(w *serial.Writer) {
	w.U32(t.previousCycle)
	w.U16(t.timer)
	w.U16(t.period)
	w.Byte(t.lastOutput)
}
func (t *apuTimer) read(r *serial.Reader) {
	t.previousCycle = r.U32()
	t.timer = r.U16()
	t.period = r.U16()
	t.lastOutput = r.Byte()
}

func (l *lengthCounter) append(w *serial.Writer) {
	w.Bool(l.isTriangle)
	w.Bool(l.enabled)
	w.Bool(l.halt)
	w.Bool(l.newHaltValue)
	w.Byte(l.counter)
	w.Byte(l.reloadValue)
	w.Byte(l.previousValue)
}
func (l *lengthCounter) read(r *serial.Reader) {
	l.isTriangle = r.Bool()
	l.enabled = r.Bool()
	l.halt = r.Bool()
	l.newHaltValue = r.Bool()
	l.counter = r.Byte()
	l.reloadValue = r.Byte()
	l.previousValue = r.Byte()
}

func (e *envelope) append(w *serial.Writer) {
	e.length.append(w)
	w.Bool(e.constantVolume)
	w.Byte(e.volume)
	w.Bool(e.start)
	w.Byte(byte(e.divider))
	w.Byte(e.counter)
}
func (e *envelope) read(r *serial.Reader) {
	e.length.read(r)
	e.constantVolume = r.Bool()
	e.volume = r.Byte()
	e.start = r.Bool()
	e.divider = int8(r.Byte())
	e.counter = r.Byte()
}

func (p *Pulse) append(w *serial.Writer) {
	p.Env.append(w)
	p.tmr.append(w)
	w.Bool(p.IsChannel1)
	w.Byte(p.Duty)
	w.Byte(p.DutyPos)
	w.Bool(p.SweepEnabled)
	w.Byte(p.SweepPeriod)
	w.Bool(p.SweepNegate)
	w.Byte(p.SweepShift)
	w.Bool(p.ReloadSweep)
	w.Byte(p.SweepDivider)
	w.U32(p.SweepTarget)
	w.U16(p.RealPeriod)
}
func (p *Pulse) read(r *serial.Reader) {
	p.Env.read(r)
	p.tmr.read(r)
	p.IsChannel1 = r.Bool()
	p.Duty = r.Byte()
	p.DutyPos = r.Byte()
	p.SweepEnabled = r.Bool()
	p.SweepPeriod = r.Byte()
	p.SweepNegate = r.Bool()
	p.SweepShift = r.Byte()
	p.ReloadSweep = r.Bool()
	p.SweepDivider = r.Byte()
	p.SweepTarget = r.U32()
	p.RealPeriod = r.U16()
}

func (t *Triangle) append(w *serial.Writer) {
	t.Len.append(w)
	t.tmr.append(w)
	w.Byte(t.LinearCounter)
	w.Byte(t.LinearCounterReload)
	w.Bool(t.LinearReloadFlag)
	w.Bool(t.LinearControlFlag)
	w.Byte(t.SequencePosition)
}
func (t *Triangle) read(r *serial.Reader) {
	t.Len.read(r)
	t.tmr.read(r)
	t.LinearCounter = r.Byte()
	t.LinearCounterReload = r.Byte()
	t.LinearReloadFlag = r.Bool()
	t.LinearControlFlag = r.Bool()
	t.SequencePosition = r.Byte()
}

func (n *Noise) append(w *serial.Writer) {
	n.Env.append(w)
	n.tmr.append(w)
	w.U16(n.ShiftRegister)
	w.Bool(n.ModeFlag)
}
func (n *Noise) read(r *serial.Reader) {
	n.Env.read(r)
	n.tmr.read(r)
	n.ShiftRegister = r.U16()
	n.ModeFlag = r.Bool()
}

func (d *DMC) append(w *serial.Writer) {
	d.tmr.append(w)
	w.U16(d.SampleAddr)
	w.U16(d.SampleLen)
	w.Byte(d.OutputLevel)
	w.Bool(d.IRQEnable)
	w.Bool(d.Loop)
	w.U16(d.CurrentAddr)
	w.U16(d.BytesLeft)
	w.Byte(d.ReadBuffer)
	w.Bool(d.BufferEmpty)
	w.Byte(d.ShiftRegister)
	w.Byte(d.BitsLeft)
	w.Bool(d.Silence)
	w.Bool(d.NeedToRunF)
	w.Byte(byte(d.TransferStartDelay))
	w.Byte(byte(d.DisableDelay))
	w.Bool(d.IRQ)
	// requestDMA / stopDMA are re-installed by the machine on Restore.
}
func (d *DMC) read(r *serial.Reader) {
	d.tmr.read(r)
	d.SampleAddr = r.U16()
	d.SampleLen = r.U16()
	d.OutputLevel = r.Byte()
	d.IRQEnable = r.Bool()
	d.Loop = r.Bool()
	d.CurrentAddr = r.U16()
	d.BytesLeft = r.U16()
	d.ReadBuffer = r.Byte()
	d.BufferEmpty = r.Bool()
	d.ShiftRegister = r.Byte()
	d.BitsLeft = r.Byte()
	d.Silence = r.Bool()
	d.NeedToRunF = r.Bool()
	d.TransferStartDelay = int8(r.Byte())
	d.DisableDelay = int8(r.Byte())
	d.IRQ = r.Bool()
}

// ClearDMAHooks nils the DMA callback fields. They are re-installed by
// the machine on Restore and never serialized, so tests use this to drop
// live wiring before comparing a round-tripped snapshot.
func (d *DMC) ClearDMAHooks() {
	d.requestDMA = nil
	d.stopDMA = nil
}

func (f *frameCounter) append(w *serial.Writer) {
	w.I32(f.previousCycle)
	w.U32(f.currentStep)
	w.U32(f.stepMode)
	w.Bool(f.inhibitIRQ)
	w.Byte(f.blockTick)
	w.I16(f.newValue)
	w.Byte(byte(f.writeDelay))
	w.Bool(f.irqFlag)
	w.U64(f.irqFlagClearClock)
}
func (f *frameCounter) read(r *serial.Reader) {
	f.previousCycle = r.I32()
	f.currentStep = r.U32()
	f.stepMode = r.U32()
	f.inhibitIRQ = r.Bool()
	f.blockTick = r.Byte()
	f.newValue = r.I16()
	f.writeDelay = int8(r.Byte())
	f.irqFlag = r.Bool()
	f.irqFlagClearClock = r.U64()
}

// Append writes the whole APU State.
func (s *State) Append(w *serial.Writer) {
	s.Pulse1.append(w)
	s.Pulse2.append(w)
	s.Tri.append(w)
	s.Noise.append(w)
	s.DMC.append(w)
	s.Frame.append(w)
	w.Bool(s.FrameIRQ)
	w.U32(s.CurrentCycle)
	w.U32(s.PreviousCycle)
	w.Bool(s.NeedRun)
	w.U64(s.CPUCycle)
	w.U64(uint64(s.SampleAcc))
}

// Read restores the whole APU State in Append's order.
func (s *State) Read(r *serial.Reader) {
	s.Pulse1.read(r)
	s.Pulse2.read(r)
	s.Tri.read(r)
	s.Noise.read(r)
	s.DMC.read(r)
	s.Frame.read(r)
	s.FrameIRQ = r.Bool()
	s.CurrentCycle = r.U32()
	s.PreviousCycle = r.U32()
	s.NeedRun = r.Bool()
	s.CPUCycle = r.U64()
	s.SampleAcc = int(r.U64())
}
