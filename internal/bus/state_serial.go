package bus

import "github.com/danielgatis/go-headless-nes/internal/serial"

// Append writes the bus State to w in a fixed field order. A round-trip
// test (state_serial_test.go) guards against a field being missed.
func (s *State) Append(w *serial.Writer) {
	w.Bytes(s.RAM[:])
	w.Byte(s.ExternalOpenBus)
	w.Byte(s.InternalOpenBus)
}

// Read restores the bus State from r in the same order Append wrote it.
func (s *State) Read(r *serial.Reader) {
	r.ReadBytes(s.RAM[:])
	s.ExternalOpenBus = r.Byte()
	s.InternalOpenBus = r.Byte()
}
