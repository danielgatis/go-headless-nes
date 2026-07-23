package apu

// DMC (delta modulation channel). It plays 1-bit delta samples fetched from CPU
// memory through the CPU's DMA unit: when its buffer empties it requests
// a transfer (requestDMA → cpu.StartDmcTransfer), and the CPU delivers the
// byte via Deliver on a later read cycle.
type DMC struct {
	tmr apuTimer

	SampleAddr uint16
	SampleLen  uint16

	OutputLevel byte
	IRQEnable   bool
	Loop        bool

	CurrentAddr uint16
	BytesLeft   uint16
	ReadBuffer  byte
	BufferEmpty bool

	ShiftRegister byte
	BitsLeft      byte
	Silence       bool
	NeedToRunF    bool

	TransferStartDelay int8
	DisableDelay       int8

	IRQ bool

	// hooks installed by the machine (cpu.StartDmcTransfer / StopDmcTransfer)
	requestDMA func()
	stopDMA    func()
}

// NTSC DMC rate table.
var dmcPeriodTable = [16]uint16{
	428, 380, 340, 320, 286, 254, 226, 214, 190, 160, 142, 128, 106, 84, 72, 54,
}

func (d *DMC) initSample() {
	d.CurrentAddr = d.SampleAddr
	d.BytesLeft = d.SampleLen
	if d.BytesLeft > 0 {
		d.NeedToRunF = true
	}
}

// startTransfer requests a DMA fetch when the buffer needs a byte.
func (d *DMC) startTransfer() {
	if d.BufferEmpty && d.BytesLeft > 0 && d.requestDMA != nil {
		d.requestDMA()
	}
}

// ReadAddr is the CPU DMA unit's hook for the sample address.
func (d *DMC) ReadAddr() uint16 { return d.CurrentAddr }

// Deliver accepts a byte fetched by the CPU DMA unit.
func (d *DMC) Deliver(value byte) {
	if d.BytesLeft > 0 {
		d.ReadBuffer = value
		d.BufferEmpty = false

		d.CurrentAddr++
		if d.CurrentAddr == 0 {
			d.CurrentAddr = 0x8000
		}

		d.BytesLeft--
		if d.BytesLeft == 0 {
			if d.Loop {
				d.initSample()
			} else if d.IRQEnable {
				d.IRQ = true
			}
		}
	}

	// When the DMA ends on the APU cycle right before the bit counter
	// resets on a 1-byte non-looping sample, the reload of the sample
	// triggers another DMA that is aborted one cycle later — a 1-cycle
	// halt on the CPU (the "implicit DMA abort").
	if d.SampleLen == 1 && !d.Loop && d.BitsLeft == 1 && d.tmr.getTimer() < 2 {
		d.ShiftRegister = d.ReadBuffer
		d.BufferEmpty = false
		d.initSample()
		d.DisableDelay = 3
	}
}

// run advances the output shifter to targetCycle.
func (d *DMC) run(targetCycle uint32) {
	for d.tmr.run(targetCycle) {
		if !d.Silence {
			if d.ShiftRegister&0x01 != 0 {
				if d.OutputLevel <= 125 {
					d.OutputLevel += 2
				}
			} else if d.OutputLevel >= 2 {
				d.OutputLevel -= 2
			}
		}
		d.ShiftRegister >>= 1

		d.BitsLeft--
		if d.BitsLeft == 0 {
			d.BitsLeft = 8
			if d.BufferEmpty {
				d.Silence = true
			} else {
				d.Silence = false
				d.ShiftRegister = d.ReadBuffer
				d.BufferEmpty = true
				d.NeedToRunF = true
				if d.TransferStartDelay == 0 {
					d.startTransfer()
				}
			}
		}
		d.tmr.addOutput(d.OutputLevel)
	}
}

// processClock handles the transfer/disable delay counters each APU cycle.
func (d *DMC) processClock() {
	if d.DisableDelay != 0 {
		d.DisableDelay--
		if d.DisableDelay == 0 {
			d.BytesLeft = 0
			if d.stopDMA != nil {
				d.stopDMA()
			}
		}
	}
	if d.TransferStartDelay != 0 {
		d.TransferStartDelay--
		if d.TransferStartDelay == 0 {
			d.startTransfer()
		}
	}
	d.NeedToRunF = d.DisableDelay != 0 || d.TransferStartDelay != 0 || d.BytesLeft != 0
}

func (d *DMC) needToRun() bool {
	if d.NeedToRunF {
		d.processClock()
	}
	return d.NeedToRunF
}

func (d *DMC) reset(softReset bool) {
	d.tmr.reset()
	if !softReset {
		d.SampleAddr = 0xC000
		d.SampleLen = 1
	}
	d.OutputLevel = 0
	d.IRQEnable = false
	d.Loop = false
	d.CurrentAddr = 0
	d.BytesLeft = 0
	d.ReadBuffer = 0
	d.BufferEmpty = true
	d.ShiftRegister = 0
	d.BitsLeft = 8
	d.Silence = true
	d.NeedToRunF = false
	d.TransferStartDelay = 0
	d.DisableDelay = 0
	d.IRQ = false
	d.tmr.setPeriod(dmcPeriodTable[0] - 1)
	d.tmr.setTimer(d.tmr.getPeriod())
}

// register writes ($4010-$4013).
func (d *DMC) writeControl(v byte) {
	d.IRQEnable = v&0x80 == 0x80
	d.Loop = v&0x40 == 0x40
	d.tmr.setPeriod(dmcPeriodTable[v&0x0F] - 1)
	if !d.IRQEnable {
		d.IRQ = false
	}
}

func (d *DMC) writeLevel(v byte) {
	d.OutputLevel = v & 0x7F
	d.tmr.addOutput(d.OutputLevel)
}

func (d *DMC) writeAddr(v byte)   { d.SampleAddr = 0xC000 | uint16(v)<<6 }
func (d *DMC) writeLength(v byte) { d.SampleLen = uint16(v)<<4 | 0x0001 }

// setEnabled tracks $4015 bit 4, with the odd/even
// delay that dmc_dma_start tests require.
func (d *DMC) setEnabled(enabled bool, oddCycle bool) {
	if !enabled {
		if d.DisableDelay == 0 {
			if !oddCycle {
				d.DisableDelay = 2
			} else {
				d.DisableDelay = 3
			}
		}
		d.NeedToRunF = true
	} else if d.BytesLeft == 0 {
		d.initSample()
		if !oddCycle {
			d.TransferStartDelay = 2
		} else {
			d.TransferStartDelay = 3
		}
		d.NeedToRunF = true
	}
}

func (d *DMC) endFrame()    { d.tmr.endFrame() }
func (d *DMC) status() bool { return d.BytesLeft > 0 }
func (d *DMC) output() byte { return d.tmr.getLastOutput() }
