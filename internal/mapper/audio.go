package mapper

// Expansion-audio channels. A board with an AudioLevel method has its
// level added to the APU mix every CPU cycle (the console wires it up).
// Levels are converted to the APU's analog scale with per-chip factors
// chosen so a full-volume expansion channel roughly matches a
// full-volume APU pulse.

// Hardware constants shared with the APU (private copies: these are
// physical ROM tables/clock rates, not emulator state).
const cpuHz = 21477272 / 12

var lengthTable = [32]byte{
	10, 254, 20, 2, 40, 4, 80, 6, 160, 8, 60, 10, 14, 12, 26, 14,
	12, 16, 24, 18, 48, 20, 96, 22, 192, 24, 72, 26, 16, 28, 32, 30,
}

var dutySequences = [4][8]byte{
	{0, 0, 0, 0, 0, 0, 0, 1},
	{0, 0, 0, 0, 0, 0, 1, 1},
	{0, 0, 0, 0, 1, 1, 1, 1},
	{1, 1, 1, 1, 1, 1, 0, 0},
}

const (
	// The APU pulse DAC's linear approximation is ~0.00752 per step.
	expPulseStep = 0.00752
	exp5BStep    = 0.0019
	expN163Step  = 0.0011
	expPCMStep   = 0.00167 // 8-bit PCM at DMC-comparable loudness

	// A DripGame channel outputs (sample-128)*volume, full scale ±1920;
	// this step puts a full-volume, full-swing sample at roughly a
	// full-volume APU pulse.
	expDripStep = 0.0000588
)

// --- VRC6 channels ---

type vrc6Pulse struct {
	Volume     byte
	DutyCycle  byte
	IgnoreDuty bool
	Frequency  uint16
	Enabled    bool
	Timer      int32
	Step       byte
	FreqShift  byte
}

func (p *vrc6Pulse) writeReg(addr uint16, v byte) {
	switch addr & 0x03 {
	case 0:
		p.Volume = v & 0x0F
		p.DutyCycle = (v & 0x70) >> 4
		p.IgnoreDuty = v&0x80 != 0
	case 1:
		p.Frequency = (p.Frequency & 0x0F00) | uint16(v)
	case 2:
		p.Frequency = (p.Frequency & 0xFF) | uint16(v&0x0F)<<8
		p.Enabled = v&0x80 != 0
		if !p.Enabled {
			p.Step = 0
		}
	}
}

func (p *vrc6Pulse) clock() {
	if p.Enabled {
		p.Timer--
		if p.Timer == 0 {
			p.Step = (p.Step + 1) & 0x0F
			p.Timer = int32(p.Frequency>>p.FreqShift) + 1
		}
	}
}

func (p *vrc6Pulse) volume() byte {
	switch {
	case !p.Enabled:
		return 0
	case p.IgnoreDuty:
		return p.Volume
	case p.Step <= p.DutyCycle:
		return p.Volume
	}
	return 0
}

type vrc6Saw struct {
	Rate        byte
	Accumulator byte
	Frequency   uint16
	Enabled     bool
	Timer       int32
	Step        byte
	FreqShift   byte
}

func (s *vrc6Saw) writeReg(addr uint16, v byte) {
	switch addr & 0x03 {
	case 0:
		s.Rate = v & 0x3F
	case 1:
		s.Frequency = (s.Frequency & 0x0F00) | uint16(v)
	case 2:
		s.Frequency = (s.Frequency & 0xFF) | uint16(v&0x0F)<<8
		s.Enabled = v&0x80 != 0
		if !s.Enabled {
			s.Accumulator = 0
			s.Step = 0
		}
	}
}

func (s *vrc6Saw) clock() {
	if s.Enabled {
		s.Timer--
		if s.Timer == 0 {
			s.Step = (s.Step + 1) % 14
			s.Timer = int32(s.Frequency>>s.FreqShift) + 1
			if s.Step == 0 {
				s.Accumulator = 0
			} else if s.Step&0x01 == 0 {
				s.Accumulator += s.Rate
			}
		}
	}
}

func (s *vrc6Saw) volume() byte {
	if !s.Enabled {
		return 0
	}
	return s.Accumulator >> 3
}

// save/restore pack a VRC6 channel into 8 bytes.
func (p *vrc6Pulse) save(r []byte) {
	r[0] = p.Volume
	r[1] = p.DutyCycle
	r[2] = boolByte(p.IgnoreDuty) | boolByte(p.Enabled)<<1
	r[3] = byte(p.Frequency)
	r[4] = byte(p.Frequency >> 8)
	r[5] = byte(p.Timer)
	r[6] = byte(p.Timer >> 8)
	r[7] = p.Step | p.FreqShift<<4
}

func (p *vrc6Pulse) restore(r []byte) {
	p.Volume = r[0]
	p.DutyCycle = r[1]
	p.IgnoreDuty = r[2]&1 != 0
	p.Enabled = r[2]&2 != 0
	p.Frequency = uint16(r[3]) | uint16(r[4])<<8
	p.Timer = int32(uint16(r[5]) | uint16(r[6])<<8)
	p.Step = r[7] & 0x0F
	p.FreqShift = r[7] >> 4
}

func (s *vrc6Saw) save(r []byte) {
	r[0] = s.Rate
	r[1] = s.Accumulator
	r[2] = boolByte(s.Enabled)
	r[3] = byte(s.Frequency)
	r[4] = byte(s.Frequency >> 8)
	r[5] = byte(s.Timer)
	r[6] = byte(s.Timer >> 8)
	r[7] = s.Step | s.FreqShift<<4
}

func (s *vrc6Saw) restore(r []byte) {
	s.Rate = r[0]
	s.Accumulator = r[1]
	s.Enabled = r[2]&1 != 0
	s.Frequency = uint16(r[3]) | uint16(r[4])<<8
	s.Timer = int32(uint16(r[5]) | uint16(r[6])<<8)
	s.Step = r[7] & 0x0F
	s.FreqShift = r[7] >> 4
}

// --- MMC5 squares (APU pulses without a sweep unit or the <8 period
// silencing) ---

type mmc5Square struct {
	Duty      byte
	DutyPos   byte
	Period    uint16
	Timer     uint16
	Output    byte
	LengthVal byte
	Halt      bool

	EnvConstant bool
	EnvVolume   byte
	EnvDivider  byte
	EnvDecay    byte
	EnvReload   bool

	lenReloadPending byte
	Enabled          bool
}

func (q *mmc5Square) writeReg(addr uint16, v byte) {
	switch addr & 0x03 {
	case 0:
		q.Duty = (v >> 6) & 0x03
		q.Halt = v&0x20 != 0
		q.EnvConstant = v&0x10 != 0
		q.EnvVolume = v & 0x0F
	case 2:
		q.Period = (q.Period & 0x0700) | uint16(v)
	case 3:
		q.Period = (q.Period & 0xFF) | uint16(v&0x07)<<8
		if q.Enabled {
			q.lenReloadPending = lengthTable[v>>3]
		}
		q.DutyPos = 0
		q.EnvReload = true
	}
}

func (q *mmc5Square) setEnabled(on bool) {
	q.Enabled = on
	if !on {
		q.LengthVal = 0
	}
}

func (q *mmc5Square) run() {
	if q.Timer == 0 {
		q.DutyPos = (q.DutyPos - 1) & 0x07
		q.Output = dutySequences[q.Duty][q.DutyPos] * q.envVolume()
		q.Timer = q.Period
	} else {
		q.Timer--
	}
}

func (q *mmc5Square) envVolume() byte {
	if q.LengthVal == 0 {
		return 0
	}
	if q.EnvConstant {
		return q.EnvVolume
	}
	return q.EnvDecay
}

func (q *mmc5Square) tickEnvelope() {
	if q.EnvReload {
		q.EnvReload = false
		q.EnvDecay = 15
		q.EnvDivider = q.EnvVolume
		return
	}
	if q.EnvDivider > 0 {
		q.EnvDivider--
		return
	}
	q.EnvDivider = q.EnvVolume
	if q.EnvDecay > 0 {
		q.EnvDecay--
	} else if q.Halt {
		q.EnvDecay = 15
	}
}

func (q *mmc5Square) tickLength() {
	if !q.Halt && q.LengthVal > 0 {
		q.LengthVal--
	}
}

func (q *mmc5Square) reloadLength() {
	if q.lenReloadPending != 0 {
		q.LengthVal = q.lenReloadPending
		q.lenReloadPending = 0
	}
}

// save/restore pack an MMC5 square into 11 bytes.
func (q *mmc5Square) save(r []byte) {
	r[0] = q.Duty | q.DutyPos<<2
	r[1] = byte(q.Period)
	r[2] = byte(q.Period >> 8)
	r[3] = byte(q.Timer)
	r[4] = byte(q.Timer >> 8)
	r[5] = q.Output
	r[6] = q.LengthVal
	r[7] = boolByte(q.Halt) | boolByte(q.EnvConstant)<<1 | boolByte(q.EnvReload)<<2 | boolByte(q.Enabled)<<3
	r[8] = q.EnvVolume | q.EnvDivider<<4
	r[9] = q.EnvDecay
	r[10] = q.lenReloadPending
}

func (q *mmc5Square) restore(r []byte) {
	q.Duty = r[0] & 0x03
	q.DutyPos = r[0] >> 2
	q.Period = uint16(r[1]) | uint16(r[2])<<8
	q.Timer = uint16(r[3]) | uint16(r[4])<<8
	q.Output = r[5]
	q.LengthVal = r[6]
	q.Halt = r[7]&1 != 0
	q.EnvConstant = r[7]&2 != 0
	q.EnvReload = r[7]&4 != 0
	q.Enabled = r[7]&8 != 0
	q.EnvVolume = r[8] & 0x0F
	q.EnvDivider = r[8] >> 4
	q.EnvDecay = r[9]
	q.lenReloadPending = r[10]
}

// --- Sunsoft 5B tone generators ---

type sunsoft5b struct {
	Regs        [0x10]byte
	Current     byte
	Timer       [3]int16
	ToneStep    [3]byte
	ProcessTick bool
}

// sunsoft5bVolume is the +3 dB-per-step volume DAC.
var sunsoft5bVolume = func() [16]byte {
	var lut [16]byte
	out := 1.0
	for i := 1; i < 16; i++ {
		out *= 1.1885022274370185
		out *= 1.1885022274370185
		lut[i] = byte(out)
	}
	return lut
}()

func (a *sunsoft5b) period(ch int) int16 {
	return int16(a.Regs[ch*2]) | int16(a.Regs[ch*2+1])<<8
}

func (a *sunsoft5b) toneEnabled(ch int) bool { return (a.Regs[7]>>ch)&0x01 == 0 }

func (a *sunsoft5b) writeRegister(addr uint16, v byte) {
	switch addr & 0xE000 {
	case 0xC000:
		a.Current = v
	case 0xE000:
		if a.Current <= 0x0F {
			a.Regs[a.Current] = v
		}
	}
}

// clock advances the tone units (at half the CPU rate) and returns the
// summed output level.
func (a *sunsoft5b) clock() int {
	if a.ProcessTick {
		for ch := 0; ch < 3; ch++ {
			a.Timer[ch]--
			if a.Timer[ch] <= 0 {
				a.Timer[ch] = a.period(ch)
				a.ToneStep[ch] = (a.ToneStep[ch] + 1) & 0x0F
			}
		}
	}
	a.ProcessTick = !a.ProcessTick
	sum := 0
	for ch := 0; ch < 3; ch++ {
		if a.toneEnabled(ch) && a.ToneStep[ch] < 0x08 {
			sum += int(sunsoft5bVolume[a.Regs[8+ch]&0x0F])
		}
	}
	return sum
}

// --- Namco 163 wavetable ---

// n163Audio operates on the board's 128-byte sound RAM: one channel is
// re-evaluated every 15 CPU cycles, round-robin over the enabled set.
type n163Audio struct {
	ChannelOut  [8]int16
	UpdateCount byte
	CurrentCh   int8
	Disabled    bool
}

func (a *n163Audio) channels(ram *[128]byte) int { return int(ram[0x7F]>>4) & 0x07 }

func (a *n163Audio) clock(ram *[128]byte) {
	if a.Disabled {
		return
	}
	a.UpdateCount++
	if a.UpdateCount < 15 {
		return
	}
	a.UpdateCount = 0
	a.updateChannel(ram, int(a.CurrentCh))
	a.CurrentCh--
	if int(a.CurrentCh) < 7-a.channels(ram) {
		a.CurrentCh = 7
	}
}

func (a *n163Audio) updateChannel(ram *[128]byte, ch int) {
	basePos := 0x40 + ch*0x08
	phase := uint32(ram[basePos+5])<<16 | uint32(ram[basePos+3])<<8 | uint32(ram[basePos+1])
	freq := uint32(ram[basePos+4]&0x03)<<16 | uint32(ram[basePos+2])<<8 | uint32(ram[basePos+0])
	length := uint32(256 - int(ram[basePos+4]&0xFC))
	offset := ram[basePos+6]
	volume := ram[basePos+7] & 0x0F

	phase = (phase + freq) % (length << 16)
	pos := (byte(phase>>16) + offset) & 0xFF
	var sample int8
	if pos&0x01 != 0 {
		sample = int8(ram[pos/2] >> 4)
	} else {
		sample = int8(ram[pos/2] & 0x0F)
	}
	a.ChannelOut[ch] = int16(sample-8) * int16(volume)

	ram[basePos+5] = byte(phase >> 16)
	ram[basePos+3] = byte(phase >> 8)
	ram[basePos+1] = byte(phase)
}

func (a *n163Audio) output(ram *[128]byte) int {
	n := a.channels(ram)
	sum := 0
	for i := 7; i >= 7-n; i-- {
		sum += int(a.ChannelOut[i])
	}
	return sum / (n + 1)
}
