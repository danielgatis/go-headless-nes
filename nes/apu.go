package nes

import "github.com/danielgatis/go-headless-nes/internal/apu"

// APUChannel is the decoded state of one tone channel (pulse 1/2, triangle,
// noise) for an APU-channels panel: whether its length counter keeps it
// audible, its timer period, its current volume (0-15), and, for the pulse
// channels, the duty setting. The triangle has no volume control, so its
// Volume stays zero.
type APUChannel struct {
	Enabled bool
	Period  uint16
	Volume  byte
	Duty    byte
}

// APUDMC is the decoded delta-modulation channel state.
type APUDMC struct {
	Enabled    bool
	Output     byte
	Rate       uint16
	SampleAddr uint16
	SampleLen  uint16
	BytesLeft  uint16
	IRQEnabled bool
	Loop       bool
}

// APUState is the whole audio unit decoded for a debugger: the four tone
// channels, the DMC, and the two IRQ line levels.
type APUState struct {
	Pulse1, Pulse2  APUChannel
	Triangle, Noise APUChannel
	DMC             APUDMC
	FrameIRQ        bool
	DMCIRQ          bool
}

// APUState reports the current decoded audio state, backing the APU-channel
// and waveform panels.
func (c *Console) APUState() APUState {
	s := c.core.APU.Snapshot()
	ch := func(x apu.ChannelState) APUChannel {
		return APUChannel{Enabled: x.Enabled, Period: x.Period, Volume: x.Volume, Duty: x.Duty}
	}
	return APUState{
		Pulse1:   ch(s.Pulse1),
		Pulse2:   ch(s.Pulse2),
		Triangle: ch(s.Triangle),
		Noise:    ch(s.Noise),
		DMC: APUDMC{
			Enabled:    s.DMC.Enabled,
			Output:     s.DMC.Output,
			Rate:       s.DMC.Rate,
			SampleAddr: s.DMC.SampleAddr,
			SampleLen:  s.DMC.SampleLen,
			BytesLeft:  s.DMC.BytesLeft,
			IRQEnabled: s.DMC.IRQEnabled,
			Loop:       s.DMC.Loop,
		},
		FrameIRQ: s.FrameIRQ,
		DMCIRQ:   s.DMCIRQ,
	}
}
