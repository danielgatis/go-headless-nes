// Package harness boots go-headless-nes consoles and drives ROM-based tests. It
// mirrors gabe565/gones' test harness, adapted to this emulator's nes.NES API:
// gones drives a console.Console with Step(bool)/CPU.StepErr and polls
// memory via Bus.ReadMem; this emulator's NES.Step() advances one instruction
// plus the rest of the machine, and memory is inspected side-effect-free
// through Bus.Peek.
package harness

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
	"github.com/danielgatis/go-headless-nes/internal/nes"
)

// errExit is the sentinel a callback returns to stop a run cleanly, the
// way the ROM's own completion protocol signals "done". It stands in for
// gones' console.ErrExit.
var errExit = errors.New("harness: exit requested")

// maxSteps caps a run so a ROM the emulator cannot complete fails instead of
// hanging the suite forever. gones' harness has no such guard because it
// only runs ROMs it passes; porting expected values verbatim means some
// ROMs may loop, so the cap is an emulator-specific safety net. It is far
// above the longest legitimate test (cpu_timing_test6 runs ~16 s of
// emulated time, well under this many instructions).
const maxSteps = 60_000_000

// cpuFrequency is the NTSC 2A03 clock in Hz. gones pulls this from its
// consts package to size the reset hold; this emulator has no such const, so it
// is defined locally to keep the reset-request behavior identical.
const cpuFrequency = 1789773

// StubConsole loads a ROM image and returns a bare console.
func StubConsole(r io.Reader) (*nes.NES, error) {
	cart, err := cartridge.Load(r)
	if err != nil {
		return nil, err
	}
	return nes.New(cart)
}

// Smoke loads a ROM and runs it for frames video frames, reporting any
// load/init error or a panic recovered mid-run. It is the basis of the
// smoke suite: no result is asserted beyond "boots and runs without
// crashing", so it covers ROMs that carry no automated pass/fail signal.
func Smoke(r io.Reader, frames int) (err error) {
	console, err := StubConsole(r)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic after boot: %v", r)
		}
	}()
	for range frames {
		console.RunFrame()
	}
	return nil
}

// Console is a console under test with an optional per-instruction
// callback and a pending reset countdown.
type Console struct {
	console *nes.NES
	resetIn int

	cb func(*Console) error
}

// NewConsoleTest wraps a freshly loaded console with a step callback.
func NewConsoleTest(r io.Reader, cb func(*Console) error) (*Console, error) {
	c, err := StubConsole(r)
	if err != nil {
		return nil, err
	}
	return &Console{console: c, cb: cb}, nil
}

// Run steps the console one instruction at a time, invoking the callback
// after each step until it signals success, returns another error
// (failure), or the step cap is hit (timeout).
func (c *Console) Run() error {
	for step := 0; step < maxSteps; step++ {
		c.console.Step()

		if c.cb != nil {
			if err := c.cb(c); err != nil {
				if errors.Is(err, errExit) {
					return nil
				}
				return err
			}
		}

		if c.resetIn != 0 {
			c.resetIn--
			if c.resetIn == 0 {
				c.console.Reset()
			}
		}
	}
	return errors.New("harness: ROM did not signal completion within the step cap")
}

// ExitAfterFrameNum stops a run once n full frames have been rendered.
// gones reads and clears PPU.RenderDone; this emulator's equivalent frame edge
// is PPU.TakeFrame, which the harness loop is the sole consumer of.
func ExitAfterFrameNum(n int) func(*Console) error {
	var frameCount int
	return func(c *Console) error {
		if c.console.PPU.TakeFrame() {
			frameCount++
			if frameCount > n {
				return errExit
			}
		}
		return nil
	}
}

// Status is a blargg test-ROM status code, read from $6000.
type Status int16

// The blargg $6000 status codes.
const (
	StatusPreRun  Status = -1
	StatusSuccess Status = 0
	StatusRunning Status = 0x80
	StatusReset   Status = 0x81
)

// MsgType selects where a blargg ROM writes its result message.
type MsgType uint8

// The blargg result-message channels.
const (
	MsgTypeSRAM MsgType = iota
	MsgTypePPUVRAM
)

var errInvalidMessageType = errors.New("harness: invalid message type")

// NewBlarggTest wraps a console with the completion callback matching the
// ROM's result-reporting channel.
func NewBlarggTest(r io.Reader, t MsgType) (*Console, error) {
	switch t {
	case MsgTypeSRAM:
		return NewConsoleTest(r, blarggSRAMMsgCallback)
	case MsgTypePPUVRAM:
		return NewConsoleTest(r, blarggPPUMsgCallback())
	}
	return nil, fmt.Errorf("%w: %d", errInvalidMessageType, t)
}

func blarggSRAMMsgCallback(c *Console) error {
	switch GetStatus(c) {
	case StatusPreRun, StatusRunning:
		return nil
	case StatusReset:
		if c.resetIn == 0 {
			c.resetIn = cpuFrequency / 10
		}
	default:
		return errExit
	}
	return nil
}

// GetStatus reads the blargg status byte at $6000, treating it as pre-run
// until the ROM has written its $DEB061 signature.
func GetStatus(c *Console) Status {
	status := Status(c.console.Peek(0x6000))
	if status == 0 {
		for i, b := range [3]byte{0xDE, 0xB0, 0x61} {
			if got := c.console.Peek(0x6001 + uint16(i)); got != b {
				return StatusPreRun
			}
		}
	}
	return status
}

// GetMessage reads the null-terminated result message from the ROM's
// reporting channel.
func GetMessage(c *Console, t MsgType) string {
	var msg []byte
	switch t {
	case MsgTypeSRAM:
		// gones reads Cartridge.SRAM[4:]; this emulator keeps PRG RAM inside
		// the mapper, so the message is read over the CPU bus from
		// $6004 (SRAM base $6000 + the 4-byte status/signature header).
		for addr := uint16(0x6004); addr < 0x8000; addr++ {
			msg = append(msg, c.console.Peek(addr))
		}
	case MsgTypePPUVRAM:
		msg = c.console.PPU.VRAM[:]
	}
	msg, _, found := bytes.Cut(msg, []byte{0})
	if !found {
		return ""
	}
	// Some ROMs (cpu_dummy_writes, cpu_exec_space) embed ANSI colour
	// codes and cursor moves as display hints. Strip them and any other
	// control bytes so the message is the plain human-readable text.
	msg = ansiEscape.ReplaceAll(msg, nil)
	msg = controlBytes.ReplaceAll(msg, nil)
	if t == MsgTypePPUVRAM {
		msg = regexp.MustCompile("  +").ReplaceAll(msg, []byte("\n"))
	}
	return string(bytes.TrimSpace(msg))
}

var (
	// ansiEscape matches CSI sequences like "\x1b[0;37m".
	ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	// controlBytes matches stray control characters (backspace, etc.)
	// left in the message, keeping tab and newline.
	controlBytes = regexp.MustCompile("[\x00-\x08\x0b\x0c\x0e-\x1f]")
)

func blarggPPUMsgCallback() func(*Console) error {
	var started bool
	return func(c *Console) error {
		if !started {
			if ready := c.console.Peek(0x7F1); ready != 0 {
				started = true
			}
			return nil
		}
		if ready := c.console.Peek(0x7F1); ready == 0 {
			return errExit
		}
		return nil
	}
}
