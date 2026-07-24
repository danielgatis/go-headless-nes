package apu

// Noise channel.h: a 15-bit LFSR
// clocked by the timer, gated by the shared envelope's length counter.
type Noise struct {
	Env envelope
	tmr apuTimer

	ShiftRegister uint16
	ModeFlag      bool

	// periodTable points at the region's period table. Wiring, not state:
	// the APU re-seats it after a snapshot restore, so it is not serialized.
	periodTable *[16]uint16
}

// Noise period tables. Dendy uses the NTSC table.
var (
	noisePeriodTableNTSC = [16]uint16{
		4, 8, 16, 32, 64, 96, 128, 160, 202, 254, 380, 508, 762, 1016, 2034, 4068,
	}
	noisePeriodTablePAL = [16]uint16{
		4, 8, 14, 30, 60, 88, 118, 148, 188, 236, 354, 472, 708, 944, 1890, 3778,
	}
)

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
	n.tmr.setPeriod(n.periodTable[0] - 1)
	n.ShiftRegister = 1
	n.ModeFlag = false
}

// register writes ($400C/$400E/$400F).
func (n *Noise) writeControl(v byte) { n.Env.initialize(v) }

func (n *Noise) writePeriod(v byte) {
	n.tmr.setPeriod(n.periodTable[v&0x0F] - 1)
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
