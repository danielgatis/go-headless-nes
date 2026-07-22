// Package serial is a tiny sequential binary reader/writer used by each
// emulated component to marshal its snapshot State. It exists so the
// per-package codecs stay short and uniform: every field is appended in
// a fixed order and read back in the same order, all big-endian.
//
// The State types mix exported and unexported fields, so reflection-based
// encoders (encoding/gob, binary.Write) cannot see the whole value.
// Writing the codecs by hand against this helper is the price of a
// dependency-free, complete snapshot; a per-package round-trip test
// guards against a field being forgotten.
package serial

import (
	"encoding/binary"

	"github.com/danielgatis/go-headless-nes/internal/errs"
)

func errUnderflow(want, have int) error {
	return errs.Errorf("serial: short read: want %d bytes, have %d", want, have)
}

// Writer appends fields to a byte slice.
type Writer struct{ B []byte }

// NewWriter starts a Writer, optionally reusing dst's backing array.
func NewWriter(dst []byte) *Writer { return &Writer{B: dst[:0]} }

// Byte appends a single byte.
func (w *Writer) Byte(v byte) { w.B = append(w.B, v) }

// Bool appends a bool as one byte (0 or 1).
func (w *Writer) Bool(v bool) { w.B = append(w.B, b2i(v)) }

// U16 appends a big-endian uint16.
func (w *Writer) U16(v uint16) { w.B = binary.BigEndian.AppendUint16(w.B, v) }

// U32 appends a big-endian uint32.
func (w *Writer) U32(v uint32) { w.B = binary.BigEndian.AppendUint32(w.B, v) }

// U64 appends a big-endian uint64.
func (w *Writer) U64(v uint64) { w.B = binary.BigEndian.AppendUint64(w.B, v) }

// I16 appends a big-endian int16.
func (w *Writer) I16(v int16) { w.U16(uint16(v)) }

// I32 appends a big-endian int32.
func (w *Writer) I32(v int32) { w.U32(uint32(v)) }

// Bytes appends raw bytes.
func (w *Writer) Bytes(v []byte) { w.B = append(w.B, v...) }

// Reader consumes fields from a byte slice in the same order Writer wrote
// them. The first error is sticky: once a read runs past the end, every
// subsequent read is a no-op and Err reports it.
type Reader struct {
	b   []byte
	err error
}

// NewReader reads from b.
func NewReader(b []byte) *Reader { return &Reader{b: b} }

// Err returns the first underflow error, if any.
func (r *Reader) Err() error { return r.err }

// Rest returns the unconsumed tail (for composing multiple States).
func (r *Reader) Rest() []byte { return r.b }

func (r *Reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if len(r.b) < n {
		r.err = errUnderflow(n, len(r.b))
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

// Byte reads a single byte.
func (r *Reader) Byte() byte {
	p := r.take(1)
	if p == nil {
		return 0
	}
	return p[0]
}

// Bool reads one byte as a bool.
func (r *Reader) Bool() bool { return r.Byte() != 0 }

// U16 reads a big-endian uint16.
func (r *Reader) U16() uint16 {
	p := r.take(2)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint16(p)
}

// U32 reads a big-endian uint32.
func (r *Reader) U32() uint32 {
	p := r.take(4)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint32(p)
}

// U64 reads a big-endian uint64.
func (r *Reader) U64() uint64 {
	p := r.take(8)
	if p == nil {
		return 0
	}
	return binary.BigEndian.Uint64(p)
}

// I16 reads a big-endian int16.
func (r *Reader) I16() int16 { return int16(r.U16()) }

// I32 reads a big-endian int32.
func (r *Reader) I32() int32 { return int32(r.U32()) }

// ReadBytes copies the next len(dst) bytes into dst.
func (r *Reader) ReadBytes(dst []byte) {
	p := r.take(len(dst))
	if p != nil {
		copy(dst, p)
	}
}

func b2i(v bool) byte {
	if v {
		return 1
	}
	return 0
}
