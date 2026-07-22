package debugger_test

import (
	"strings"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/debugger"
	"github.com/danielgatis/go-headless-nes/internal/nes"
	"github.com/danielgatis/go-headless-nes/internal/testrom"
)

// The synthetic ROM lays out the same landmarks these tests rely on:
// $C000 is JMP $C5F5, and $C5F5 is a JSR whose first pushed byte ($C5)
// lands at $01FD. See internal/testrom.
func newDebugger(t *testing.T) *debugger.Debugger {
	t.Helper()
	console, err := nes.New(testrom.New(t))
	if err != nil {
		t.Fatal(err)
	}
	return debugger.New(console)
}

func TestBreakpointHalts(t *testing.T) {
	d := newDebugger(t)
	// Entering at $C000 jumps straight to $C5F5.
	d.Console().CPU.Reg.PC = 0xC000
	d.AddBreakpoint(0xC5F5)
	stop := d.RunFrame()
	if stop.Reason != debugger.StopBreakpoint || stop.Addr != 0xC5F5 {
		t.Fatalf("stop = %+v, want breakpoint at C5F5", stop)
	}
	if d.Console().CPU.Reg.PC != 0xC5F5 {
		t.Errorf("PC = %04X, want C5F5", d.Console().CPU.Reg.PC)
	}
}

func TestWatchpointDetectsChange(t *testing.T) {
	d := newDebugger(t)
	d.Console().CPU.Reg.PC = 0xC000
	// The first JSR pushes the return address high byte to $01FD
	// (SP starts at FD), changing it from RAM's power-on zero.
	d.AddWatchpoint(0x01FD)
	stop := d.RunFrame()
	if stop.Reason != debugger.StopWatchpoint || stop.Addr != 0x01FD {
		t.Fatalf("stop = %+v, want watchpoint at 01FD", stop)
	}
	if stop.Old != 0 || stop.New == 0 {
		t.Errorf("watch values = %02X -> %02X, want 00 -> nonzero", stop.Old, stop.New)
	}
}

func TestStepInstructionTraces(t *testing.T) {
	d := newDebugger(t)
	d.Console().CPU.Reg.PC = 0xC000
	var log strings.Builder
	d.TraceTo = &log
	d.StepInstruction()
	d.StepInstruction()
	lines := strings.Split(strings.TrimSpace(log.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("trace lines = %d, want 2", len(lines))
	}
	if !strings.HasPrefix(lines[0], "C000  4C F5 C5  JMP $C5F5") {
		t.Errorf("unexpected first trace line: %q", lines[0])
	}
}

func TestSyncWatchesRebaselines(t *testing.T) {
	d := newDebugger(t)
	d.AddWatchpoint(0x0010)
	d.Console().Write(0x0010, 0x42) // as a snapshot restore would
	d.SyncWatches()
	d.Console().CPU.Reg.PC = 0xC000
	if stop := d.StepInstruction(); stop.Reason != debugger.StopNone {
		t.Errorf("synced watch fired anyway: %+v", stop)
	}
}

func TestRunFrameCompletesWithoutStops(t *testing.T) {
	d := newDebugger(t)
	if stop := d.RunFrame(); stop.Reason != debugger.StopNone {
		t.Fatalf("unexpected stop: %+v", stop)
	}
	// RunFrame returns as the frame is emitted, at the start of the
	// post-render/vblank region (the reference sends the frame on scanline 240).
	if sl := d.Console().PPU.Scanline; sl < 240 {
		t.Errorf("scanline = %d, want >= 240 (frame emitted at vblank)", sl)
	}
}

func TestViewsFormat(t *testing.T) {
	d := newDebugger(t)
	if v := d.CPUView(); !strings.Contains(v, "PC:C004") {
		t.Errorf("CPU view missing PC: %q", v)
	}
	if v := d.PPUView(); !strings.Contains(v, "SL:") {
		t.Errorf("PPU view malformed: %q", v)
	}
	mem := d.MemoryView(0xFFFA, 1)
	if !strings.HasPrefix(mem, "FFFA:") || len(strings.Fields(mem)) != 17 {
		t.Errorf("memory view malformed: %q", mem)
	}
}

func TestDisassemble(t *testing.T) {
	d := newDebugger(t)
	lines := d.Disassemble(0xC000, 2)
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], "JMP $C5F5") {
		t.Errorf("first line = %q, want JMP with its operand", lines[0])
	}
	// JMP abs is 3 bytes: the next instruction starts at C003.
	if !strings.HasPrefix(lines[1], "C003") {
		t.Errorf("second line = %q, want C003 prefix", lines[1])
	}
}
