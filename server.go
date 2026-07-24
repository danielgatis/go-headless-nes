package nes

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strings"

	"github.com/danielgatis/go-headless-nes/internal/errs"
)

// Server drives one Console over the binary protocol. It reads command
// frames, dispatches them against the console (and its debugger), and
// writes event frames back. One Server serves one stream to completion;
// it is not safe for concurrent use.
type Server struct {
	dec *Decoder
	enc *Encoder
	bw  *bufio.Writer

	console *Console
	pads    [2]byte // last SetInput, kept so it survives a LoadROM

	traceOn bool
	trace   io.Writer // opcode log (--trace), nil unless enabled
}

// NewServer builds a Server reading commands from r and writing events to
// w. Output is buffered and flushed after each command's events.
func NewServer(r io.Reader, w io.Writer) *Server {
	bw := bufio.NewWriter(w)
	return &Server{
		dec: NewDecoder(r),
		enc: NewEncoder(bw),
		bw:  bw,
	}
}

// SetTrace enables a human-readable opcode log to w (stderr for --trace).
func (s *Server) SetTrace(w io.Writer) { s.trace = w }

// Serve runs the handshake then the command loop until the stream ends
// cleanly (io.EOF) or a transport error occurs. Command-level failures are
// reported as OpError frames and do not stop the loop.
func (s *Server) Serve() error {
	if err := s.enc.WriteHandshake(ProtocolVersion); err != nil {
		return err
	}
	if err := s.bw.Flush(); err != nil {
		return err
	}
	peer, err := s.dec.ReadHandshake()
	if errors.Is(err, io.EOF) {
		return nil // client hung up before handshaking: clean shutdown
	}
	if err != nil {
		return errs.Wrap(err, "handshake")
	}
	if peer != ProtocolVersion {
		_ = s.emit(OpError, []byte(errs.Errorf("unsupported protocol version %d (server speaks %d)", peer, ProtocolVersion).Error()))
		_ = s.bw.Flush()
		return errs.Errorf("client protocol version %d != %d", peer, ProtocolVersion)
	}

	for {
		f, err := s.dec.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err // transport error: fatal
		}
		s.traceIn(f)
		if derr := s.dispatch(f); derr != nil {
			if err := s.emit(OpError, []byte(derr.Error())); err != nil {
				return err
			}
		}
		if err := s.bw.Flush(); err != nil {
			return err
		}
	}
}

// emit writes an event frame (and traces it if enabled).
func (s *Server) emit(op Op, payload []byte) error {
	s.traceOut(op, len(payload))
	return s.enc.Write(op, payload)
}

// dispatch handles one command frame. A returned error is a command-level
// failure reported to the client as OpError; the stream stays in sync.
func (s *Server) dispatch(f Frame) error {
	switch f.Op {
	case OpLoadROM:
		return s.loadROM(f.Payload)
	case OpRunFrame:
		return s.runFrame()
	case OpStep:
		return s.step()
	case OpSetInput:
		return s.setInput(f.Payload)
	case OpReset:
		return s.withConsole(func() error { s.console.Reset(); return nil })
	case OpSaveState:
		return s.withConsole(s.saveState)
	case OpLoadState:
		return s.loadState(f.Payload)
	case OpPeek:
		return s.peek(f.Payload)
	case OpPoke:
		return s.poke(f.Payload)
	case OpGetState:
		return s.withConsole(s.getState)
	case OpSetRegion:
		return s.setRegion(f.Payload)

	case OpAddBreak:
		return s.breakOp(f.Payload, true)
	case OpDelBreak:
		return s.breakOp(f.Payload, false)
	case OpAddWatch:
		return s.watchOp(f.Payload, true)
	case OpDelWatch:
		return s.watchOp(f.Payload, false)
	case OpDisasm:
		return s.disasm(f.Payload)
	case OpReadMem:
		return s.readMem(f.Payload)
	case OpSetTrace:
		return s.setTrace(f.Payload)

	case OpPatchPRG:
		return s.patchROM(f.Payload, true)
	case OpPatchCHR:
		return s.patchROM(f.Payload, false)
	case OpReadPRG:
		return s.readROM(f.Payload, true)
	case OpReadCHR:
		return s.readROM(f.Payload, false)
	case OpMapperWrite:
		return s.mapperWrite(f.Payload)
	case OpGetMapper:
		return s.withConsole(s.getMapper)
	case OpSetMapper:
		return s.setMapper(f.Payload)

	default:
		return errs.Errorf("unknown opcode 0x%02X", byte(f.Op))
	}
}

// withConsole runs fn only if a ROM is loaded.
func (s *Server) withConsole(fn func() error) error {
	if s.console == nil {
		return errs.New("no ROM loaded")
	}
	return fn()
}

func (s *Server) loadROM(payload []byte) error {
	console, err := NewConsole(payload)
	if err != nil {
		return err
	}
	s.console = console
	s.console.SetButtons(0, s.pads[0])
	s.console.SetButtons(1, s.pads[1])
	if s.traceOn {
		s.console.SetTrace(traceWriter{s})
	}
	return nil
}

func (s *Server) runFrame() error {
	return s.withConsole(func() error {
		stop := s.console.RunFrame()
		if err := s.emitVideoAudio(); err != nil {
			return err
		}
		if stop.Reason != StopNone {
			return s.emitStop(stop)
		}
		return nil
	})
}

func (s *Server) step() error {
	return s.withConsole(func() error {
		stop := s.console.Step()
		if err := s.emit(OpState, s.stateBlock()); err != nil {
			return err
		}
		if stop.Reason != StopNone {
			return s.emitStop(stop)
		}
		return nil
	})
}

func (s *Server) emitVideoAudio() error {
	// Both slices alias reusable console buffers; emit copies them onto the
	// wire before the console can touch them again.
	if err := s.emit(OpVideo, s.console.Video()); err != nil {
		return err
	}
	return s.emit(OpAudio, floatsToBytes(s.console.Audio()))
}

func (s *Server) emitStop(stop Stop) error {
	p := []byte{byte(stop.Reason), byte(stop.Addr >> 8), byte(stop.Addr), stop.Old, stop.New}
	return s.emit(OpStop, p)
}

func (s *Server) setInput(payload []byte) error {
	if len(payload) != 2 {
		return errs.Errorf("SetInput expects 2 bytes, got %d", len(payload))
	}
	s.pads[0] = payload[0]
	s.pads[1] = payload[1]
	if s.console != nil {
		s.console.SetButtons(0, payload[0])
		s.console.SetButtons(1, payload[1])
	}
	return nil
}

func (s *Server) setRegion(payload []byte) error {
	if len(payload) != 1 {
		return errs.Errorf("SetRegion expects 1 byte, got %d", len(payload))
	}
	if payload[0] > byte(RegionDendy) {
		return errs.Errorf("SetRegion: unknown region %d", payload[0])
	}
	return s.withConsole(func() error {
		s.console.SetRegion(Region(payload[0]))
		return nil
	})
}

func (s *Server) saveState() error {
	return s.emit(OpSnapshot, s.console.SaveState())
}

func (s *Server) loadState(payload []byte) error {
	return s.withConsole(func() error {
		if err := s.console.LoadState(payload); err != nil {
			return errs.Wrap(err, "LoadState")
		}
		return nil
	})
}

func (s *Server) getState() error { return s.emit(OpState, s.stateBlock()) }

func (s *Server) peek(payload []byte) error {
	return s.withConsole(func() error {
		addr, err := addr16(payload)
		if err != nil {
			return err
		}
		return s.emit(OpValue, []byte{s.console.Peek(addr)})
	})
}

func (s *Server) poke(payload []byte) error {
	return s.withConsole(func() error {
		if len(payload) != 3 {
			return errs.Errorf("Poke expects 3 bytes, got %d", len(payload))
		}
		s.console.Poke(uint16(payload[0])<<8|uint16(payload[1]), payload[2])
		return nil
	})
}

func (s *Server) breakOp(payload []byte, add bool) error {
	return s.withConsole(func() error {
		addr, err := addr16(payload)
		if err != nil {
			return err
		}
		if add {
			s.console.AddBreakpoint(addr)
		} else {
			s.console.RemoveBreakpoint(addr)
		}
		return nil
	})
}

func (s *Server) watchOp(payload []byte, add bool) error {
	return s.withConsole(func() error {
		addr, err := addr16(payload)
		if err != nil {
			return err
		}
		if add {
			s.console.AddWatchpoint(addr)
		} else {
			s.console.RemoveWatchpoint(addr)
		}
		return nil
	})
}

func (s *Server) disasm(payload []byte) error {
	return s.withConsole(func() error {
		if len(payload) != 3 {
			return errs.Errorf("Disasm expects addr(2)+n(1), got %d bytes", len(payload))
		}
		addr := uint16(payload[0])<<8 | uint16(payload[1])
		lines := s.console.Disasm(addr, int(payload[2]))
		return s.emit(OpDisasmText, []byte(strings.Join(lines, "\n")))
	})
}

func (s *Server) readMem(payload []byte) error {
	return s.withConsole(func() error {
		if len(payload) != 4 {
			return errs.Errorf("ReadMem expects addr(2)+len(2), got %d bytes", len(payload))
		}
		addr := uint16(payload[0])<<8 | uint16(payload[1])
		n := int(payload[2])<<8 | int(payload[3])
		return s.emit(OpMemBlock, s.console.ReadMem(addr, n))
	})
}

func (s *Server) setTrace(payload []byte) error {
	if len(payload) != 1 {
		return errs.Errorf("SetTrace expects 1 byte, got %d", len(payload))
	}
	s.traceOn = payload[0] != 0
	if s.console != nil {
		if s.traceOn {
			s.console.SetTrace(traceWriter{s})
		} else {
			s.console.SetTrace(nil)
		}
	}
	return nil
}

// --- live patch (romhack / trainer) ---

func (s *Server) patchROM(payload []byte, prg bool) error {
	return s.withConsole(func() error {
		if len(payload) < 4 {
			return errs.New("Patch expects offset(4)+data")
		}
		off := int(binary.BigEndian.Uint32(payload[:4]))
		data := payload[4:]
		if prg {
			return s.console.PatchPRG(off, data)
		}
		return s.console.PatchCHR(off, data)
	})
}

func (s *Server) readROM(payload []byte, prg bool) error {
	return s.withConsole(func() error {
		if len(payload) != 8 {
			return errs.Errorf("ReadROM expects offset(4)+len(4), got %d bytes", len(payload))
		}
		off := int(binary.BigEndian.Uint32(payload[:4]))
		n := int(binary.BigEndian.Uint32(payload[4:]))
		read := s.console.ReadPRG
		if !prg {
			read = s.console.ReadCHR
		}
		out, err := read(off, n)
		if err != nil {
			return err
		}
		return s.emit(OpMemBlock, out)
	})
}

func (s *Server) mapperWrite(payload []byte) error {
	return s.withConsole(func() error {
		if len(payload) != 3 {
			return errs.Errorf("MapperWrite expects addr(2)+val(1), got %d bytes", len(payload))
		}
		s.console.WriteMapper(uint16(payload[0])<<8|uint16(payload[1]), payload[2])
		return nil
	})
}

func (s *Server) getMapper() error {
	return s.emit(OpMapperState, s.console.MapperState())
}

func (s *Server) setMapper(payload []byte) error {
	return s.withConsole(func() error {
		if err := s.console.SetMapperState(payload); err != nil {
			return errs.Wrap(err, "SetMapper")
		}
		return nil
	})
}

// --- state block ---

// stateBlock encodes the debug-observable machine state (OpState payload):
// a version byte then CPU registers, cycle counters and PPU position.
func (s *Server) stateBlock() []byte {
	st := s.console.State()
	b := make([]byte, 0, 40)
	b = append(b, 1) // StateVersion
	b = append(b, st.A, st.X, st.Y, st.SP, st.P, byte(st.PC>>8), byte(st.PC))
	b = binary.BigEndian.AppendUint64(b, st.Cycles)
	b = binary.BigEndian.AppendUint16(b, st.Stall)
	b = binary.BigEndian.AppendUint16(b, uint16(st.Scanline))
	b = binary.BigEndian.AppendUint32(b, uint32(st.Dot))
	b = binary.BigEndian.AppendUint64(b, st.Frame)
	b = binary.BigEndian.AppendUint64(b, st.MasterClock)
	return b
}

// addr16 reads a 2-byte big-endian address.
func addr16(payload []byte) (uint16, error) {
	if len(payload) != 2 {
		return 0, errs.Errorf("expected 2-byte address, got %d bytes", len(payload))
	}
	return uint16(payload[0])<<8 | uint16(payload[1]), nil
}

// floatsToBytes packs samples as little-endian float32.
func floatsToBytes(samples []float32) []byte {
	out := make([]byte, len(samples)*4)
	for i, v := range samples {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}
