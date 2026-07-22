package nes

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// TestFrameRoundTrip encodes representative frames and decodes them back,
// asserting the opcode and payload survive the wire form.
func TestFrameRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		op      Op
		payload []byte
	}{
		{"empty", OpRunFrame, nil},
		{"input", OpSetInput, []byte{0x81, 0x00}},
		{"peek addr", OpPeek, []byte{0xC0, 0x00}},
		{"video frame", OpVideo, bytes.Repeat([]byte{0x2A}, 61440)},
		{"error string", OpError, []byte("no ROM loaded")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewEncoder(&buf).Write(tc.op, tc.payload); err != nil {
				t.Fatal(err)
			}
			f, err := NewDecoder(&buf).Read()
			if err != nil {
				t.Fatal(err)
			}
			if f.Op != tc.op {
				t.Errorf("op = 0x%02X, want 0x%02X", byte(f.Op), byte(tc.op))
			}
			if !bytes.Equal(f.Payload, tc.payload) {
				t.Errorf("payload = %v, want %v", f.Payload, tc.payload)
			}
		})
	}
}

// TestMultipleFrames decodes several frames written back-to-back into one
// stream, in order.
func TestMultipleFrames(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	for _, f := range []Frame{
		{Op: OpRunFrame},
		{Op: OpPoke, Payload: []byte{0x00, 0x00, 0x42}},
		{Op: OpReset},
	} {
		if err := enc.Write(f.Op, f.Payload); err != nil {
			t.Fatalf("write %v: %v", f.Op, err)
		}
	}

	dec := NewDecoder(&buf)
	for _, want := range []Op{OpRunFrame, OpPoke, OpReset} {
		f, err := dec.Read()
		if err != nil {
			t.Fatal(err)
		}
		if f.Op != want {
			t.Errorf("op = 0x%02X, want 0x%02X", byte(f.Op), byte(want))
		}
	}
	if _, err := dec.Read(); !errors.Is(err, io.EOF) {
		t.Errorf("after last frame: err = %v, want EOF", err)
	}
}

func TestHandshake(t *testing.T) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).WriteHandshake(ProtocolVersion); err != nil {
		t.Fatal(err)
	}
	v, err := NewDecoder(&buf).ReadHandshake()
	if err != nil {
		t.Fatal(err)
	}
	if v != ProtocolVersion {
		t.Errorf("version = %d, want %d", v, ProtocolVersion)
	}
}

func TestHandshakeRejectsWrongOpcode(t *testing.T) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Write(OpRunFrame, nil); err != nil { // not a handshake
		t.Fatalf("write: %v", err)
	}
	if _, err := NewDecoder(&buf).ReadHandshake(); err == nil {
		t.Error("expected error when first frame is not a handshake")
	}
}

func TestTruncatedHeader(t *testing.T) {
	// Only 3 of the 5 header bytes.
	dec := NewDecoder(bytes.NewReader([]byte{0x02, 0x00, 0x00}))
	if _, err := dec.Read(); err == nil {
		t.Error("expected error on truncated header")
	}
}

func TestTruncatedPayload(t *testing.T) {
	// Header claims 10 bytes, only 4 follow.
	stream := []byte{byte(OpVideo), 0x00, 0x00, 0x00, 0x0A, 1, 2, 3, 4}
	dec := NewDecoder(bytes.NewReader(stream))
	if _, err := dec.Read(); err == nil {
		t.Error("expected error on truncated payload")
	}
}

func TestRejectsOversizePayload(t *testing.T) {
	// Header claims MaxPayload+1 bytes.
	var hdr [5]byte
	hdr[0] = byte(OpVideo)
	// length = MaxPayload+1
	n := uint32(MaxPayload + 1)
	hdr[1] = byte(n >> 24)
	hdr[2] = byte(n >> 16)
	hdr[3] = byte(n >> 8)
	hdr[4] = byte(n)
	dec := NewDecoder(bytes.NewReader(hdr[:]))
	if _, err := dec.Read(); err == nil {
		t.Error("expected error on oversize payload length")
	}
}

func TestCleanEOF(t *testing.T) {
	if _, err := NewDecoder(bytes.NewReader(nil)).Read(); !errors.Is(err, io.EOF) {
		t.Errorf("empty stream: err = %v, want EOF", err)
	}
}
