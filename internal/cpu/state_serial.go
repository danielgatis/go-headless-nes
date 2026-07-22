package cpu

import "github.com/danielgatis/go-headless-nes/internal/serial"

// Append writes the CPU State to w in a fixed field order, covering every
// field including the unexported interrupt-pipeline and DMA-scratch flags
// a reflection encoder would miss. state_serial_test.go round-trips this.
func (s *State) Append(w *serial.Writer) {
	// Registers.
	w.Byte(s.Reg.A)
	w.Byte(s.Reg.X)
	w.Byte(s.Reg.Y)
	w.Byte(s.Reg.SP)
	w.Byte(s.Reg.P)
	w.U16(s.Reg.PC)

	w.U64(s.Cycles)

	// Interrupt recognition.
	w.Bool(s.NMIFlag)
	w.Byte(s.IRQFlag)
	w.Bool(s.needNMI)
	w.Bool(s.prevNeedNMI)
	w.Bool(s.prevNMIFlag)
	w.Bool(s.runIRQ)
	w.Bool(s.prevRunIRQ)
	w.Byte(s.irqMask)

	// DMA scratch.
	w.Bool(s.spriteDMATransfer)
	w.Byte(s.spriteDMAOffset)
	w.Bool(s.dmcDMARunning)
	w.Bool(s.abortDMCDma)
	w.Bool(s.needHalt)
	w.Bool(s.needDummyRead)

	w.Bool(s.crashed)
	w.U64(s.masterClock)
	w.U64(s.ppuOffset)
	w.U16(s.Stall)
	w.Bool(s.Jammed)
}

// Read restores the CPU State from r in Append's order.
func (s *State) Read(r *serial.Reader) {
	s.Reg.A = r.Byte()
	s.Reg.X = r.Byte()
	s.Reg.Y = r.Byte()
	s.Reg.SP = r.Byte()
	s.Reg.P = r.Byte()
	s.Reg.PC = r.U16()

	s.Cycles = r.U64()

	s.NMIFlag = r.Bool()
	s.IRQFlag = r.Byte()
	s.needNMI = r.Bool()
	s.prevNeedNMI = r.Bool()
	s.prevNMIFlag = r.Bool()
	s.runIRQ = r.Bool()
	s.prevRunIRQ = r.Bool()
	s.irqMask = r.Byte()

	s.spriteDMATransfer = r.Bool()
	s.spriteDMAOffset = r.Byte()
	s.dmcDMARunning = r.Bool()
	s.abortDMCDma = r.Bool()
	s.needHalt = r.Bool()
	s.needDummyRead = r.Bool()

	s.crashed = r.Bool()
	s.masterClock = r.U64()
	s.ppuOffset = r.U64()
	s.Stall = r.U16()
	s.Jammed = r.Bool()
}
