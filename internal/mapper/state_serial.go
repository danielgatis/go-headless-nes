package mapper

import "github.com/danielgatis/go-headless-nes/internal/serial"

// Append writes the mapper State (register file plus PRG/CHR RAM) to w.
func (s *State) Append(w *serial.Writer) {
	w.Bytes(s.Regs[:])
	w.Bytes(s.PRGRAM[:])
	w.Bytes(s.CHRRAM[:])
}

// Read restores the mapper State from r in Append's order.
func (s *State) Read(r *serial.Reader) {
	r.ReadBytes(s.Regs[:])
	r.ReadBytes(s.PRGRAM[:])
	r.ReadBytes(s.CHRRAM[:])
}
