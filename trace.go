package nes

import "fmt"

// Tracing renders opcodes as human-readable lines on the trace writer
// (stderr under --trace), since the binary wire form is not eyeball-
// friendly. The seam is the dispatch, where opcodes are already decoded —
// independent of the transport, so it works identically for a WASM binding.

// traceIn logs an inbound command frame.
func (s *Server) traceIn(f Frame) {
	if s.trace == nil {
		return
	}
	_, _ = fmt.Fprintf(s.trace, "-> %s (%dB)\n", opName(f.Op), len(f.Payload))
}

// traceOut logs an outbound event frame.
func (s *Server) traceOut(op Op, n int) {
	if s.trace == nil {
		return
	}
	_, _ = fmt.Fprintf(s.trace, "<- %s (%dB)\n", opName(op), n)
}

// traceWriter adapts the debugger's TraceTo io.Writer into OpTraceLine
// events: each nestest line the debugger produces becomes one event frame.
type traceWriter struct{ s *Server }

func (w traceWriter) Write(p []byte) (int, error) {
	// The debugger writes one line (with a trailing newline) per call.
	line := p
	if n := len(line); n > 0 && line[n-1] == '\n' {
		line = line[:n-1]
	}
	if err := w.s.enc.Write(OpTraceLine, line); err != nil {
		return 0, err
	}
	return len(p), nil
}

func opName(op Op) string {
	switch op {
	case OpHandshake:
		return "Handshake"
	case OpLoadROM:
		return "LoadROM"
	case OpRunFrame:
		return "RunFrame"
	case OpStep:
		return "Step"
	case OpSetInput:
		return "SetInput"
	case OpReset:
		return "Reset"
	case OpSaveState:
		return "SaveState"
	case OpLoadState:
		return "LoadState"
	case OpPeek:
		return "Peek"
	case OpPoke:
		return "Poke"
	case OpGetState:
		return "GetState"
	case OpAddBreak:
		return "AddBreak"
	case OpDelBreak:
		return "DelBreak"
	case OpAddWatch:
		return "AddWatch"
	case OpDelWatch:
		return "DelWatch"
	case OpDisasm:
		return "Disasm"
	case OpReadMem:
		return "ReadMem"
	case OpSetTrace:
		return "SetTrace"
	case OpPatchPRG:
		return "PatchPRG"
	case OpPatchCHR:
		return "PatchCHR"
	case OpReadPRG:
		return "ReadPRG"
	case OpReadCHR:
		return "ReadCHR"
	case OpMapperWrite:
		return "MapperWrite"
	case OpGetMapper:
		return "GetMapper"
	case OpSetMapper:
		return "SetMapper"
	case OpVideo:
		return "Video"
	case OpAudio:
		return "Audio"
	case OpSnapshot:
		return "Snapshot"
	case OpValue:
		return "Value"
	case OpState:
		return "State"
	case OpStop:
		return "Stop"
	case OpDisasmText:
		return "DisasmText"
	case OpMemBlock:
		return "MemBlock"
	case OpTraceLine:
		return "TraceLine"
	case OpMapperState:
		return "MapperState"
	case OpError:
		return "Error"
	default:
		return fmt.Sprintf("Op(0x%02X)", byte(op))
	}
}
