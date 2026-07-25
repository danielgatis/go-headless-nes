package apu

// ChannelState is a decoded snapshot of one tone channel for a debugger's
// APU view: whether its length counter is keeping it audible, its timer
// period, and its current output volume (0-15). The triangle has no volume
// control, so its Volume is left zero.
type ChannelState struct {
	Enabled bool
	Period  uint16
	Volume  byte
	Duty    byte // pulse channels only
}

// DMCState is a decoded snapshot of the delta-modulation channel.
type DMCState struct {
	Enabled    bool
	Output     byte   // 7-bit output level
	Rate       uint16 // timer period for the current rate index
	SampleAddr uint16
	SampleLen  uint16
	BytesLeft  uint16
	IRQEnabled bool
	Loop       bool
}

// Snapshot is the whole audio unit decoded for a debugger.
type Snapshot struct {
	Pulse1, Pulse2  ChannelState
	Triangle, Noise ChannelState
	DMC             DMCState
	FrameIRQ        bool
	DMCIRQ          bool
}

// Snapshot decodes the current audio state without disturbing it.
func (a *APU) Snapshot() Snapshot {
	return Snapshot{
		Pulse1: ChannelState{
			Enabled: a.Pulse1.Env.length.status(),
			Period:  a.Pulse1.tmr.period,
			Volume:  a.Pulse1.Env.getVolume(),
			Duty:    a.Pulse1.Duty,
		},
		Pulse2: ChannelState{
			Enabled: a.Pulse2.Env.length.status(),
			Period:  a.Pulse2.tmr.period,
			Volume:  a.Pulse2.Env.getVolume(),
			Duty:    a.Pulse2.Duty,
		},
		Triangle: ChannelState{
			Enabled: a.Tri.Len.status(),
			Period:  a.Tri.tmr.period,
		},
		Noise: ChannelState{
			Enabled: a.Noise.Env.length.status(),
			Period:  a.Noise.tmr.period,
			Volume:  a.Noise.Env.getVolume(),
		},
		DMC: DMCState{
			Enabled:    a.DMC.BytesLeft > 0,
			Output:     a.DMC.OutputLevel,
			Rate:       a.DMC.tmr.period,
			SampleAddr: a.DMC.SampleAddr,
			SampleLen:  a.DMC.SampleLen,
			BytesLeft:  a.DMC.BytesLeft,
			IRQEnabled: a.DMC.IRQEnable,
			Loop:       a.DMC.Loop,
		},
		FrameIRQ: a.FrameIRQ,
		DMCIRQ:   a.DMC.IRQ,
	}
}
