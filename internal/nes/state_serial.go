package nes

import "github.com/danielgatis/go-headless-nes/internal/serial"

// Append writes the whole console Snapshot to w by composing the
// per-package State codecs in a fixed order. This is the serialized form
// used by SaveState/LoadState; it covers every field including the
// unexported ones a reflection encoder cannot reach. ROM contents stay in
// the Cartridge and are never part of a snapshot.
func (s *Snapshot) Append(w *serial.Writer) {
	s.CPU.Append(w)
	s.PPU.Append(w)
	s.APU.Append(w)
	s.Bus.Append(w)
	s.Mapper.Append(w)
	s.Pads[0].AppendState(w)
	s.Pads[1].AppendState(w)
	s.Ctrl.Append(w)
}

// Read restores the whole Snapshot from r in Append's order.
func (s *Snapshot) Read(r *serial.Reader) {
	s.CPU.Read(r)
	s.PPU.Read(r)
	s.APU.Read(r)
	s.Bus.Read(r)
	s.Mapper.Read(r)
	s.Pads[0].ReadState(r)
	s.Pads[1].ReadState(r)
	s.Ctrl.Read(r)
}

// MarshalBinary encodes the Snapshot to a new byte slice.
func (s *Snapshot) MarshalBinary() []byte {
	w := serial.NewWriter(nil)
	s.Append(w)
	return w.B
}

// UnmarshalBinary decodes a Snapshot previously produced by
// MarshalBinary. It reports a short read (truncated data); trailing bytes
// are ignored so a newer, longer wire form stays backward-compatible.
func (s *Snapshot) UnmarshalBinary(b []byte) error {
	r := serial.NewReader(b)
	s.Read(r)
	return r.Err()
}
