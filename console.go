// Package nes is the public face of the emulator core. It offers the same
// primitives two ways:
//
//   - In-process: NewConsole boots a whole console (execution, video and
//     audio, memory, save states, a debugger and live patching) behind a
//     direct Go API. Each Console is independent, so N emulator instances
//     are just N values. A Console is not safe for concurrent use; drive
//     each one from a single goroutine.
//
//   - Over the wire: Encoder, Decoder and Server speak the binary control
//     protocol on any io stream (stdin/stdout for the cmd/nesd
//     binary, JS callbacks for a future WASM target). Server is a thin
//     adapter that maps command frames onto a Console.
//
// Presentation and orchestration policy (UI, rewind, save-slots,
// scripting) belongs to the consumer, which builds it from these
// primitives. Palette, VideoRGBA and AudioStream are the deliberate
// exceptions: they perform the two conversions every frontend repeats
// (color indices to RGBA, float32 samples to PCM) without deciding any
// policy beyond that.
package nes

import (
	"bytes"
	"io"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
	"github.com/danielgatis/go-headless-nes/internal/controller"
	"github.com/danielgatis/go-headless-nes/internal/debugger"
	"github.com/danielgatis/go-headless-nes/internal/errs"
	"github.com/danielgatis/go-headless-nes/internal/mapper"
	core "github.com/danielgatis/go-headless-nes/internal/nes"
	"github.com/danielgatis/go-headless-nes/internal/ppu"
	"github.com/danielgatis/go-headless-nes/internal/region"
	"github.com/danielgatis/go-headless-nes/internal/serial"
)

// Video dimensions: every frame is VideoWidth*VideoHeight NES color
// indices (0-63), one byte per pixel.
const (
	VideoWidth  = ppu.Width
	VideoHeight = ppu.Height
)

// Button bits for SetButtons (and the OpSetInput payload), in the order
// the joypad shift register reports them. Combine with bitwise or:
// ButtonA|ButtonRight.
const (
	ButtonA      = controller.A
	ButtonB      = controller.B
	ButtonSelect = controller.Select
	ButtonStart  = controller.Start
	ButtonUp     = controller.Up
	ButtonDown   = controller.Down
	ButtonLeft   = controller.Left
	ButtonRight  = controller.Right
)

// StopReason says why RunFrame or Step returned early. The values are
// wire-stable: OpStop's first payload byte is the reason.
type StopReason byte

// The reasons execution can stop early.
const (
	// StopNone means execution completed without hitting a stop condition.
	StopNone StopReason = iota
	// StopBreakpoint means execution reached a breakpoint PC.
	StopBreakpoint
	// StopWatchpoint means a watched address changed value.
	StopWatchpoint
)

// Stop describes a debugger halt.
type Stop struct {
	Reason StopReason
	Addr   uint16 // breakpoint PC or watched address
	Old    byte   // watchpoints: value before the change
	New    byte
}

// stopOf converts the internal debugger halt to the public type.
func stopOf(s debugger.Stop) Stop {
	return Stop{Reason: StopReason(s.Reason), Addr: s.Addr, Old: s.Old, New: s.New}
}

// State is the debug-observable machine state: CPU registers, cycle
// counters and PPU position.
type State struct {
	A, X, Y, SP, P byte
	PC             uint16
	Cycles         uint64 // CPU cycles since power-on
	Stall          uint16 // pending DMA stall cycles
	Scanline       int    // current PPU scanline
	Dot            int    // current PPU dot within the scanline
	Frame          uint64 // PPU frame counter
	MasterClock    uint64
}

// Console is one in-process emulator instance: a whole NES plus its
// debugger. It is not safe for concurrent use.
type Console struct {
	core  *core.NES
	debug *debugger.Debugger
	pads  [2]byte
	snap  core.Snapshot
	audio []float32
}

// NewConsole boots a console from an iNES ROM image.
func NewConsole(rom []byte) (*Console, error) {
	cart, err := cartridge.Load(bytes.NewReader(rom))
	if err != nil {
		return nil, errs.Wrap(err, "loading ROM")
	}
	c, err := core.New(cart)
	if err != nil {
		return nil, errs.Wrap(err, "creating console")
	}
	return &Console{core: c, debug: debugger.New(c), audio: make([]float32, 4096)}, nil
}

// SetButtons sets the held buttons for pad 0 or 1. The state latches at
// the start of the next RunFrame.
func (c *Console) SetButtons(pad int, buttons byte) { c.pads[pad] = buttons }

// RunFrame emulates until the next vblank, honoring breakpoints and
// watchpoints. After it returns, Video and Audio hold the frame's output.
func (c *Console) RunFrame() Stop {
	c.core.Controllers[0].SetButtons(c.pads[0])
	c.core.Controllers[1].SetButtons(c.pads[1])
	return stopOf(c.debug.RunFrame())
}

// Step executes a single CPU instruction.
func (c *Console) Step() Stop { return stopOf(c.debug.StepInstruction()) }

// Reset presses the console's reset button.
func (c *Console) Reset() { c.core.Reset() }

// SetRegion switches the console's TV system (NTSC, PAL or Dendy) at
// runtime, or re-detects the cartridge's own with RegionAuto. The switch
// is live: the emulator keeps running and simply adopts the new timing on
// its next cycle, without a reset, exactly as real hardware does. The
// region is a fixed property of the machine, so it is not part of a save
// state.
func (c *Console) SetRegion(r Region) { c.core.SetRegion(region.Region(r)) }

// Region reports the TV system currently in effect. It is always a
// concrete system (never RegionAuto): after RegionAuto it reflects what
// the cartridge resolved to.
func (c *Console) Region() Region { return Region(c.core.Region()) }

// FrameRate is how many emulated frames make one real second on the
// current TV system (NTSC ~60.0988, PAL and Dendy ~50.0070). A real-time
// frontend should run RunFrame this many times per second; a fixed-tick
// loop should round it. Driving PAL content at NTSC's 60 runs it ~20%
// fast, which is the usual "why is my PAL game too quick" bug.
func (c *Console) FrameRate() float64 { return c.core.FrameRate() }

// Video returns the current framebuffer: VideoWidth*VideoHeight NES color
// indices (0-63). The slice aliases the live PPU buffer, so read it
// before the next RunFrame, or copy it out.
func (c *Console) Video() []byte { return c.core.Framebuffer()[:] }

// Audio drains and returns the samples generated since the last call.
// The slice is reused by the next call; copy it out to keep it.
func (c *Console) Audio() []float32 {
	n := c.core.DrainSamples(c.audio)
	// Grow if a drain ever fills the buffer completely, so no samples are
	// silently dropped.
	for n == len(c.audio) {
		grown := make([]float32, len(c.audio)*2)
		copy(grown, c.audio)
		c.audio = grown
		n += c.core.DrainSamples(c.audio[n:])
	}
	return c.audio[:n]
}

// SaveState serializes the whole console to a blob that LoadState (or the
// OpLoadState command) restores deterministically.
func (c *Console) SaveState() []byte {
	c.core.Save(&c.snap)
	return c.snap.MarshalBinary()
}

// LoadState restores a snapshot produced by SaveState.
func (c *Console) LoadState(b []byte) error {
	if err := c.snap.UnmarshalBinary(b); err != nil {
		return err
	}
	c.core.Restore(&c.snap)
	c.debug.SyncWatches()
	return nil
}

// State reports the debug-observable machine state.
func (c *Console) State() State {
	r := c.core.CPU.Reg
	return State{
		A: r.A, X: r.X, Y: r.Y, SP: r.SP, P: r.P, PC: r.PC,
		Cycles:      c.core.CPU.Cycles,
		Stall:       c.core.CPU.Stall,
		Scanline:    int(c.core.PPU.Scanline),
		Dot:         int(c.core.PPU.Cycle),
		Frame:       c.core.PPU.Frame,
		MasterClock: c.core.MasterClock(),
	}
}

// Peek reads a CPU address without side effects (safe on PPU registers).
func (c *Console) Peek(addr uint16) byte { return c.core.Peek(addr) }

// Poke writes a CPU address through the bus, with normal side effects.
func (c *Console) Poke(addr uint16, v byte) { c.core.Write(addr, v) }

// ReadMem peeks n consecutive CPU addresses starting at addr.
func (c *Console) ReadMem(addr uint16, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = c.core.Peek(addr + uint16(i))
	}
	return out
}

// AddBreakpoint halts execution when the PC reaches addr.
func (c *Console) AddBreakpoint(addr uint16) { c.debug.AddBreakpoint(addr) }

// RemoveBreakpoint removes a breakpoint.
func (c *Console) RemoveBreakpoint(addr uint16) { c.debug.RemoveBreakpoint(addr) }

// AddWatchpoint halts execution when the value at addr changes.
func (c *Console) AddWatchpoint(addr uint16) { c.debug.AddWatchpoint(addr) }

// RemoveWatchpoint removes a watchpoint.
func (c *Console) RemoveWatchpoint(addr uint16) { c.debug.RemoveWatchpoint(addr) }

// Disasm disassembles n instructions starting at addr, one line each.
func (c *Console) Disasm(addr uint16, n int) []string {
	return c.debug.Disassemble(addr, n)
}

// SetTrace streams a nestest-format line to w for every instruction
// executed. Pass nil to stop tracing.
func (c *Console) SetTrace(w io.Writer) { c.debug.TraceTo = w }

// PatchPRG overwrites PRG ROM at off with data, live (romhack / trainer).
func (c *Console) PatchPRG(off int, data []byte) error { return c.patchROM(true, off, data) }

// PatchCHR overwrites CHR ROM at off with data, live.
func (c *Console) PatchCHR(off int, data []byte) error { return c.patchROM(false, off, data) }

// ReadPRG copies n bytes of PRG ROM starting at off.
func (c *Console) ReadPRG(off, n int) ([]byte, error) { return c.readROM(true, off, n) }

// ReadCHR copies n bytes of CHR ROM starting at off.
func (c *Console) ReadCHR(off, n int) ([]byte, error) { return c.readROM(false, off, n) }

// WriteMapper writes a mapper register through the PRG bus, bypassing the
// CPU (no cycles elapse).
func (c *Console) WriteMapper(addr uint16, v byte) { c.core.Mapper.WritePRG(addr, v) }

// MapperState serializes the mapper's register file and PRG/CHR RAM.
func (c *Console) MapperState() []byte {
	var ms mapper.State
	c.core.Mapper.Save(&ms)
	w := serial.NewWriter(nil)
	ms.Append(w)
	return w.B
}

// SetMapperState restores a blob produced by MapperState.
func (c *Console) SetMapperState(b []byte) error {
	var ms mapper.State
	r := serial.NewReader(b)
	ms.Read(r)
	if err := r.Err(); err != nil {
		return err
	}
	c.core.Mapper.Restore(&ms)
	return nil
}

func (c *Console) romBuf(prg bool) []byte {
	if prg {
		return c.core.Cart.PRG
	}
	return c.core.Cart.CHR
}

func (c *Console) patchROM(prg bool, off int, data []byte) error {
	buf := c.romBuf(prg)
	if buf == nil {
		return errs.New("cartridge has no ROM of that kind (CHR RAM?)")
	}
	if off < 0 || off+len(data) > len(buf) {
		return errs.Errorf("patch out of range: off %d + %d > %d", off, len(data), len(buf))
	}
	copy(buf[off:], data)
	return nil
}

func (c *Console) readROM(prg bool, off, n int) ([]byte, error) {
	buf := c.romBuf(prg)
	if buf == nil {
		return nil, errs.New("cartridge has no ROM of that kind")
	}
	if off < 0 || n < 0 || off+n > len(buf) {
		return nil, errs.Errorf("read out of range: off %d + %d > %d", off, n, len(buf))
	}
	out := make([]byte, n)
	copy(out, buf[off:])
	return out, nil
}
