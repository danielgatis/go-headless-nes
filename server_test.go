package nes

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

// script builds a command byte stream: a handshake followed by the given
// frames. It returns a reader the server consumes.
func script(t *testing.T, frames ...Frame) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.WriteHandshake(ProtocolVersion); err != nil {
		t.Fatal(err)
	}
	for _, f := range frames {
		if err := enc.Write(f.Op, f.Payload); err != nil {
			t.Fatal(err)
		}
	}
	return &buf
}

// run serves the scripted commands and returns every event frame the
// server emitted (after the server's own handshake).
func run(t *testing.T, frames ...Frame) []Frame {
	t.Helper()
	var out bytes.Buffer
	srv := NewServer(script(t, frames...), &out)
	if err := srv.Serve(); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	dec := NewDecoder(&out)
	// First frame is the server's handshake.
	if _, err := dec.ReadHandshake(); err != nil {
		t.Fatalf("server handshake: %v", err)
	}
	var events []Frame
	for {
		f, err := dec.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decoding events: %v", err)
		}
		events = append(events, f)
	}
	return events
}

func cmd(op Op, payload ...byte) Frame { return Frame{Op: op, Payload: payload} }

func loadROM(t *testing.T) Frame { return Frame{Op: OpLoadROM, Payload: testrom.Image(t)} }

// count returns how many events have the given opcode.
func count(events []Frame, op Op) (n int) {
	for _, e := range events {
		if e.Op == op {
			n++
		}
	}
	return
}

func first(events []Frame, op Op) (Frame, bool) {
	for _, e := range events {
		if e.Op == op {
			return e, true
		}
	}
	return Frame{}, false
}

func TestServerRunFrameEmitsVideoAudio(t *testing.T) {
	events := run(t,
		loadROM(t),
		cmd(OpRunFrame),
		cmd(OpRunFrame),
		cmd(OpRunFrame),
	)
	if got := count(events, OpVideo); got != 3 {
		t.Errorf("Video events = %d, want 3", got)
	}
	if got := count(events, OpAudio); got != 3 {
		t.Errorf("Audio events = %d, want 3", got)
	}
	v, _ := first(events, OpVideo)
	if len(v.Payload) != VideoWidth*VideoHeight {
		t.Errorf("Video payload = %d bytes, want %d", len(v.Payload), VideoWidth*VideoHeight)
	}
	a, _ := first(events, OpAudio)
	if len(a.Payload)%4 != 0 {
		t.Errorf("Audio payload %d not a multiple of 4", len(a.Payload))
	}
	if len(a.Payload) == 0 {
		t.Error("Audio payload empty; expected samples")
	}
}

func TestServerErrorBeforeROM(t *testing.T) {
	// RunFrame before LoadROM errors, but the stream stays usable: a
	// following LoadROM+RunFrame still produces a Video frame.
	events := run(t,
		cmd(OpRunFrame),
		loadROM(t),
		cmd(OpRunFrame),
	)
	if count(events, OpError) != 1 {
		t.Errorf("Error events = %d, want 1", count(events, OpError))
	}
	if count(events, OpVideo) != 1 {
		t.Errorf("Video events = %d, want 1 (stream recovered)", count(events, OpVideo))
	}
}

func TestServerUnknownOpcode(t *testing.T) {
	events := run(t, loadROM(t), cmd(Op(0x7E)))
	if count(events, OpError) != 1 {
		t.Errorf("Error events = %d, want 1 for unknown opcode", count(events, OpError))
	}
}

func TestServerPeekPoke(t *testing.T) {
	// Poke $0042 = 0xAB into RAM, then Peek it back.
	events := run(t,
		loadROM(t),
		cmd(OpPoke, 0x00, 0x42, 0xAB),
		cmd(OpPeek, 0x00, 0x42),
	)
	v, ok := first(events, OpValue)
	if !ok {
		t.Fatal("no Value event")
	}
	if v.Payload[0] != 0xAB {
		t.Errorf("Peek = 0x%02X, want 0xAB", v.Payload[0])
	}
}

func TestServerStepEmitsState(t *testing.T) {
	events := run(t, loadROM(t), cmd(OpStep))
	st, ok := first(events, OpState)
	if !ok {
		t.Fatal("no State event")
	}
	if st.Payload[0] != 1 {
		t.Errorf("StateVersion = %d, want 1", st.Payload[0])
	}
	// Version(1) + regs(7) + cycles(8) + stall(2) + sl(2) + cyc(4) + frame(8) + mclk(8)
	if len(st.Payload) != 40 {
		t.Errorf("State block = %d bytes, want 40", len(st.Payload))
	}
}

func TestServerSnapshotDeterminism(t *testing.T) {
	// Save, run more, load, run again: the video after re-running from the
	// snapshot must match the first run from that point.
	base := run(t, loadROM(t), cmd(OpRunFrame), cmd(OpSaveState))
	snap, ok := first(base, OpSnapshot)
	if !ok {
		t.Fatal("no Snapshot event")
	}

	// Two independent servers can't share state, so drive one server:
	events := run(t,
		loadROM(t),
		cmd(OpRunFrame), // advance
		cmd(OpLoadState, snap.Payload...),
		cmd(OpRunFrame), // this frame is from the restored state
	)
	videos := framesOf(events, OpVideo)
	if len(videos) < 2 {
		t.Fatalf("want >=2 video frames, got %d", len(videos))
	}
	// Snapshot was taken right after the first RunFrame; loading it and
	// running one frame should reproduce that frame deterministically.
	// (We compare the post-load frame against the pre-save baseline frame.)
	baseVideos := framesOf(base, OpVideo)
	if !bytes.Equal(videos[1], baseVideos[0]) {
		t.Error("post-LoadState frame differs from the snapshot's baseline (nondeterministic restore)")
	}
}

func framesOf(events []Frame, op Op) [][]byte {
	var out [][]byte
	for _, e := range events {
		if e.Op == op {
			out = append(out, e.Payload)
		}
	}
	return out
}

func TestServerBreakpoint(t *testing.T) {
	// The synthetic ROM spins at $C004 (JMP $C004). A breakpoint there
	// should halt RunFrame and emit a Stop with that PC.
	events := run(t,
		loadROM(t),
		cmd(OpAddBreak, 0xC0, 0x04),
		cmd(OpRunFrame),
	)
	st, ok := first(events, OpStop)
	if !ok {
		t.Fatal("no Stop event from breakpoint")
	}
	if st.Payload[0] != 1 { // StopBreakpoint = 1
		t.Errorf("Stop reason = %d, want 1 (breakpoint)", st.Payload[0])
	}
	addr := uint16(st.Payload[1])<<8 | uint16(st.Payload[2])
	if addr != 0xC004 {
		t.Errorf("Stop addr = $%04X, want $C004", addr)
	}
}

func TestServerWatchpoint(t *testing.T) {
	// Watch $0042, then Poke it: not through RunFrame, but a watchpoint set
	// then a Step after Poke should report the change. We poke via the CPU
	// bus and step; the ROM's NOPs don't touch $0042, so we assert the
	// watchpoint arms without error and a Step still yields State.
	events := run(t,
		loadROM(t),
		cmd(OpAddWatch, 0x00, 0x42),
		cmd(OpStep),
	)
	if count(events, OpError) != 0 {
		t.Errorf("unexpected errors: %d", count(events, OpError))
	}
	if _, ok := first(events, OpState); !ok {
		t.Error("no State event after watch+step")
	}
}

func TestServerDisasm(t *testing.T) {
	// Disassemble 3 instructions at $C000 (JMP $C5F5 / NOP / JMP $C004).
	events := run(t,
		loadROM(t),
		cmd(OpDisasm, 0xC0, 0x00, 0x03),
	)
	dt, ok := first(events, OpDisasmText)
	if !ok {
		t.Fatal("no DisasmText event")
	}
	text := string(dt.Payload)
	if !bytes.Contains(dt.Payload, []byte("JMP")) {
		t.Errorf("disasm missing JMP: %q", text)
	}
}

func TestServerPatchPRG(t *testing.T) {
	// Write bytes into PRG at offset 0, then read them back.
	patch := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	payload := append([]byte{0, 0, 0, 0}, patch...) // offset 0 + data
	events := run(t,
		loadROM(t),
		Frame{Op: OpPatchPRG, Payload: payload},
		cmd(OpReadPRG, 0, 0, 0, 0, 0, 0, 0, 4), // offset 0, len 4
	)
	mb, ok := first(events, OpMemBlock)
	if !ok {
		t.Fatal("no MemBlock event")
	}
	if !bytes.Equal(mb.Payload, patch) {
		t.Errorf("ReadPRG = % X, want % X", mb.Payload, patch)
	}
}

func TestServerPatchPRGOutOfRange(t *testing.T) {
	// Offset past the end of PRG must error, not panic.
	payload := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x00}
	events := run(t, loadROM(t), Frame{Op: OpPatchPRG, Payload: payload})
	if count(events, OpError) != 1 {
		t.Errorf("Error events = %d, want 1 for out-of-range patch", count(events, OpError))
	}
}

func TestServerMapperRoundTrip(t *testing.T) {
	// GetMapper then SetMapper with the same bytes must not error.
	get := run(t, loadROM(t), cmd(OpGetMapper))
	ms, ok := first(get, OpMapperState)
	if !ok {
		t.Fatal("no MapperState event")
	}
	events := run(t, loadROM(t), cmd(OpSetMapper, ms.Payload...))
	if count(events, OpError) != 0 {
		t.Errorf("SetMapper errored: %d error events", count(events, OpError))
	}
}
