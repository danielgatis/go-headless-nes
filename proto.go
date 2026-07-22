// The binary control protocol: a framed, dependency-free wire format a
// consumer uses to drive the emulator core over any io stream (stdin/stdout
// for the native binary, JS callbacks for a future WASM target). One
// uniform frame in both directions:
//
//	[1 byte opcode][4 bytes big-endian length][payload]

package nes

import (
	"encoding/binary"
	"io"

	"github.com/danielgatis/go-headless-nes/internal/errs"
)

// Op is a one-byte frame opcode. The space is partitioned so future
// commands and events slot into reserved ranges without renumbering:
//
//	0x00        handshake
//	0x01-0x3F   core commands (client -> core)
//	0x40-0x7F   extension commands (reserved)
//	0x81-0xBF   core events (core -> client)
//	0xC0-0xFE   extension events (reserved)
//	0xFF        error
type Op byte

// The protocol opcodes, grouped by direction and purpose. Values are
// wire-stable; see the Op byte-range map above.
const (
	OpHandshake Op = 0x00

	// Commands: execution / memory / state.

	OpLoadROM   Op = 0x01
	OpRunFrame  Op = 0x02
	OpStep      Op = 0x03
	OpSetInput  Op = 0x04
	OpReset     Op = 0x05
	OpSaveState Op = 0x06
	OpLoadState Op = 0x07
	OpPeek      Op = 0x08
	OpPoke      Op = 0x09
	OpGetState  Op = 0x0A

	// Commands: debug (drive internal/debugger).

	OpAddBreak Op = 0x10
	OpDelBreak Op = 0x11
	OpAddWatch Op = 0x12
	OpDelWatch Op = 0x13
	OpDisasm   Op = 0x14
	OpReadMem  Op = 0x15
	OpSetTrace Op = 0x16

	// Commands: live patch (romhack / trainer).

	OpPatchPRG    Op = 0x20
	OpPatchCHR    Op = 0x21
	OpReadPRG     Op = 0x22
	OpReadCHR     Op = 0x23
	OpMapperWrite Op = 0x24
	OpGetMapper   Op = 0x25
	OpSetMapper   Op = 0x26

	// Events.

	OpVideo       Op = 0x81
	OpAudio       Op = 0x82
	OpSnapshot    Op = 0x83
	OpValue       Op = 0x84
	OpState       Op = 0x85
	OpStop        Op = 0x86
	OpDisasmText  Op = 0x87
	OpMemBlock    Op = 0x88
	OpTraceLine   Op = 0x89
	OpMapperState Op = 0x8A

	OpError Op = 0xFF
)

// ProtocolVersion is exchanged in the handshake so the two ends can
// detect a mismatch and evolve independently.
const ProtocolVersion byte = 1

// MaxPayload caps a frame's payload so a corrupt length header cannot
// force a huge allocation. The largest legitimate payloads are a video
// frame (61440 bytes) and a snapshot (~48 KB); 1 MiB leaves generous room.
const MaxPayload = 1 << 20

// headerSize is the fixed frame header: opcode + uint32 length.
const headerSize = 5

// Frame is one decoded wire message.
type Frame struct {
	Op      Op
	Payload []byte
}

// Encoder writes frames to an io.Writer. It is not safe for concurrent use.
type Encoder struct {
	w   io.Writer
	hdr [headerSize]byte
}

// NewEncoder returns an Encoder writing to w.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{w: w} }

// Write emits one frame: the opcode, the payload length, then the payload.
func (e *Encoder) Write(op Op, payload []byte) error {
	e.hdr[0] = byte(op)
	binary.BigEndian.PutUint32(e.hdr[1:], uint32(len(payload)))
	if _, err := e.w.Write(e.hdr[:]); err != nil {
		return errs.Wrap(err, "writing frame header")
	}
	if len(payload) > 0 {
		if _, err := e.w.Write(payload); err != nil {
			return errs.Wrap(err, "writing frame payload")
		}
	}
	return nil
}

// WriteHandshake emits the handshake frame carrying the protocol version.
func (e *Encoder) WriteHandshake(version byte) error {
	return e.Write(OpHandshake, []byte{version})
}

// Decoder reads frames from an io.Reader. It is not safe for concurrent use.
type Decoder struct {
	r   io.Reader
	hdr [headerSize]byte
}

// NewDecoder returns a Decoder reading from r.
func NewDecoder(r io.Reader) *Decoder { return &Decoder{r: r} }

// Read decodes the next frame. It returns io.EOF unwrapped when the stream
// ends cleanly at a frame boundary, so callers can treat that as a normal
// shutdown.
func (d *Decoder) Read() (Frame, error) {
	if _, err := io.ReadFull(d.r, d.hdr[:]); err != nil {
		if err == io.EOF {
			return Frame{}, io.EOF
		}
		return Frame{}, errs.Wrap(err, "reading frame header")
	}
	op := Op(d.hdr[0])
	n := binary.BigEndian.Uint32(d.hdr[1:])
	if n > MaxPayload {
		return Frame{}, errs.Errorf("frame payload too large: %d bytes", n)
	}
	var payload []byte
	if n > 0 {
		payload = make([]byte, n)
		if _, err := io.ReadFull(d.r, payload); err != nil {
			return Frame{}, errs.Wrap(err, "reading frame payload")
		}
	}
	return Frame{Op: op, Payload: payload}, nil
}

// ReadHandshake reads a frame and requires it to be a handshake, returning
// the peer's protocol version.
func (d *Decoder) ReadHandshake() (byte, error) {
	f, err := d.Read()
	if err != nil {
		return 0, err
	}
	if f.Op != OpHandshake {
		return 0, errs.Errorf("expected handshake, got opcode 0x%02X", byte(f.Op))
	}
	if len(f.Payload) < 1 {
		return 0, errs.New("handshake frame missing version byte")
	}
	return f.Payload[0], nil
}
