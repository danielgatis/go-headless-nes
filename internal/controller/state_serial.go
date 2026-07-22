package controller

import "github.com/danielgatis/go-headless-nes/internal/serial"

// AppendState writes a Controller's latched state to w. (It is not named
// Append/Read to avoid colliding with the joypad's Read shift method.)
func (c *Controller) AppendState(w *serial.Writer) {
	w.Byte(c.Buttons)
	w.Bool(c.Strobe)
	w.Byte(c.Shift)
}

// ReadState restores a Controller from r in AppendState's order.
func (c *Controller) ReadState(r *serial.Reader) {
	c.Buttons = r.Byte()
	c.Strobe = r.Bool()
	c.Shift = r.Byte()
}

// Append writes the ManagerState (strobe scheduling + read-clock cache).
func (s *ManagerState) Append(w *serial.Writer) {
	w.U16(s.PrevReadAddr)
	w.U64(s.PadPrevCycle[0])
	w.U64(s.PadPrevCycle[1])
	w.Byte(s.PadPrevValue[0])
	w.Byte(s.PadPrevValue[1])
	w.Byte(s.WriteValue)
	w.Byte(s.WritePending)
}

// Read restores the ManagerState from r in Append's order.
func (s *ManagerState) Read(r *serial.Reader) {
	s.PrevReadAddr = r.U16()
	s.PadPrevCycle[0] = r.U64()
	s.PadPrevCycle[1] = r.U64()
	s.PadPrevValue[0] = r.Byte()
	s.PadPrevValue[1] = r.Byte()
	s.WriteValue = r.Byte()
	s.WritePending = r.Byte()
}
