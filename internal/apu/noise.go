package apu

// Noise channel.h: a 15-bit LFSR
// clocked by the timer, gated by the shared envelope's length counter.
type Noise struct {
	Env envelope
	tmr apuTimer

	ShiftRegister uint16
	ModeFlag      bool
}

// NTSC noise period table.
var noisePeriodTable = [16]uint16{
	4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
}

func (n *Noise) isMuted() bool { return n.ShiftRegister&0x01 == 0x01 }

// run advances the LFSR to targetCycle.
func (n *Noise) run(targetCycle uint32) {
	for n.tmr.run(targetCycle) {
		shift := uint(1)
		if n.ModeFlag {
			shift = 6
		}
		feedback := (n.ShiftRegister & 0x01) ^ ((n.ShiftRegister >> shift) & 0x01)
		n.ShiftRegister >>= 1
		n.ShiftRegister |= feedback << 14

		if n.isMuted() {
			n.tmr.addOutput(0)
		} else {
			n.tmr.addOutput(n.Env.getVolume())
		}
	}
}

func (n *Noise) reset(softReset bool) {
	n.Env.reset(softReset)
	n.tmr.reset()
	n.tmr.setPeriod(noisePeriodTable[0] - 1)
	n.ShiftRegister = 1
	n.ModeFlag = false
}

// register writes ($400C/$400E/$400F).
func (n *Noise) writeControl(v byte) { n.Env.initialize(v) }

func (n *Noise) writePeriod(v byte) {
	n.tmr.setPeriod(noisePeriodTable[v&0x0F] - 1)
	n.ModeFlag = v&0x80 == 0x80
}

func (n *Noise) writeLength(v byte) {
	n.Env.length.load(v >> 3)
	n.Env.resetEnvelope()
}

func (n *Noise) tickEnvelope()      { n.Env.tick() }
func (n *Noise) tickLengthCounter() { n.Env.length.tick() }
func (n *Noise) reloadLength()      { n.Env.length.reload() }
func (n *Noise) endFrame()          { n.tmr.endFrame() }
func (n *Noise) setEnabled(e bool)  { n.Env.length.setEnabled(e) }
func (n *Noise) status() bool       { return n.Env.length.status() }
func (n *Noise) output() byte       { return n.tmr.getLastOutput() }
