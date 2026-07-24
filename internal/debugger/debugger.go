package debugger

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/danielgatis/go-headless-nes/internal/cpu"
	"github.com/danielgatis/go-headless-nes/internal/nes"
)

// Debugger wraps a console with inspection and control. It owns no
// emulated state and injects nothing into the machine's hot paths:
// breakpoints and watchpoints are checked between instructions, and
// all memory access goes through side-effect-free peeks.
//
// This is also the reserved extension point for scripting: a future
// engine (e.g. Starlark) drives exactly this API, peek memory, read
// CPU state, step instructions or frames, and inject controller input
// through Console().Controllers.
type Debugger struct {
	console *nes.NES

	breakpoints map[uint16]struct{}
	watches     map[uint16]byte // last seen value per watched address
	watchAddrs  []uint16        // watched addresses, sorted: checks report deterministically

	// TraceTo receives one nestest-format line per executed
	// instruction while set.
	TraceTo io.Writer
}

// StopReason says why stepping returned early.
type StopReason int

// The reasons stepping can return early.
const (
	// StopNone means stepping completed without hitting a stop condition.
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

// New attaches a debugger to a console.
func New(console *nes.NES) *Debugger {
	return &Debugger{
		console:     console,
		breakpoints: make(map[uint16]struct{}),
		watches:     make(map[uint16]byte),
	}
}

// Console exposes the attached machine.
func (d *Debugger) Console() *nes.NES { return d.console }

// AddBreakpoint halts execution when PC reaches addr.
func (d *Debugger) AddBreakpoint(addr uint16) { d.breakpoints[addr] = struct{}{} }

// RemoveBreakpoint deletes a breakpoint.
func (d *Debugger) RemoveBreakpoint(addr uint16) { delete(d.breakpoints, addr) }

// AddWatchpoint halts execution when the value at addr changes.
// (A write of the same value is invisible to this scheme; catching it
// needs bus-level hooks, which the per-dot rework will add.)
func (d *Debugger) AddWatchpoint(addr uint16) {
	if _, ok := d.watches[addr]; !ok {
		i, _ := slices.BinarySearch(d.watchAddrs, addr)
		d.watchAddrs = slices.Insert(d.watchAddrs, i, addr)
	}
	d.watches[addr] = d.console.Peek(addr)
}

// RemoveWatchpoint deletes a watchpoint.
func (d *Debugger) RemoveWatchpoint(addr uint16) {
	if _, ok := d.watches[addr]; ok {
		delete(d.watches, addr)
		i, _ := slices.BinarySearch(d.watchAddrs, addr)
		d.watchAddrs = slices.Delete(d.watchAddrs, i, i+1)
	}
}

// SyncWatches re-reads every watched address into its baseline. Any
// state restore (rewind, load state) must call this: baselines carried
// over from the abandoned timeline would report a spurious change on
// the next step.
func (d *Debugger) SyncWatches() {
	for _, addr := range d.watchAddrs {
		d.watches[addr] = d.console.Peek(addr)
	}
}

// StepInstruction executes one instruction (tracing it if enabled) and
// reports any watchpoint change it caused. A stalled CPU (post-DMA)
// consumes its stall cycles without executing, so those steps are not
// traced, the instruction at PC has not run yet.
func (d *Debugger) StepInstruction() Stop {
	if d.TraceTo != nil && d.console.CPU.Stall == 0 {
		_, _ = fmt.Fprintln(d.TraceTo, d.Trace())
	}
	d.console.Step()
	for _, addr := range d.watchAddrs {
		old := d.watches[addr]
		if v := d.console.Peek(addr); v != old {
			d.watches[addr] = v
			return Stop{Reason: StopWatchpoint, Addr: addr, Old: old, New: v}
		}
	}
	return Stop{}
}

// RunFrame runs until the PPU completes a frame, a breakpoint is hit,
// or a watched value changes.
//
// Calling it while parked exactly on a breakpoint means "continue":
// one instruction executes before breakpoints re-arm, the standard
// step-off, so resuming cannot re-trigger the same hit forever.
func (d *Debugger) RunFrame() Stop {
	if _, parked := d.breakpoints[d.console.CPU.Reg.PC]; parked {
		if stop := d.StepInstruction(); stop.Reason != StopNone {
			return stop
		}
		if d.console.PPU.TakeFrame() {
			return Stop{}
		}
	}
	for {
		if _, hit := d.breakpoints[d.console.CPU.Reg.PC]; hit {
			return Stop{Reason: StopBreakpoint, Addr: d.console.CPU.Reg.PC}
		}
		if stop := d.StepInstruction(); stop.Reason != StopNone {
			return stop
		}
		if d.console.PPU.TakeFrame() {
			return Stop{}
		}
	}
}

// Trace formats the instruction at PC in nestest format.
func (d *Debugger) Trace() string {
	return Trace(d.console.CPU, d.console, int(d.console.PPU.Scanline), int(d.console.PPU.Cycle))
}

// CPUView formats the CPU state for an on-screen or terminal viewer.
func (d *Debugger) CPUView() string {
	r := d.console.CPU.Reg
	flags := ""
	for i, name := range [8]byte{'C', 'Z', 'I', 'D', 'B', 'U', 'V', 'N'} {
		if r.P&(1<<i) != 0 {
			flags = string(name) + flags
		} else {
			flags = strings.ToLower(string(name)) + flags
		}
	}
	return fmt.Sprintf("PC:%04X A:%02X X:%02X Y:%02X SP:%02X P:%s CYC:%d",
		r.PC, r.A, r.X, r.Y, r.SP, flags, d.console.CPU.Cycles)
}

// PPUView formats the PPU state.
func (d *Debugger) PPUView() string {
	p := d.console.PPU
	return fmt.Sprintf("SL:%3d DOT:%3d FRAME:%d STAT:%02X V:%04X T:%04X X:%d",
		p.Scanline, p.Cycle, p.Frame, p.StatusByte(), p.VideoRAMAddr, p.TmpVideoRAMAddr, p.XScroll)
}

// MemoryView hex-dumps rows of 16 bytes starting at addr, without
// disturbing the machine.
func (d *Debugger) MemoryView(addr uint16, rows int) string {
	var sb strings.Builder
	for range rows {
		fmt.Fprintf(&sb, "%04X:", addr)
		for i := range uint16(16) {
			fmt.Fprintf(&sb, " %02X", d.console.Peek(addr+i))
		}
		sb.WriteByte('\n')
		addr += 16
	}
	return sb.String()
}

// Disassemble decodes n instructions starting at addr, rendering full
// operands (with effective addresses computed from the current CPU
// registers, as the trace format does).
func (d *Debugger) Disassemble(addr uint16, n int) []string {
	out := make([]string, 0, n)
	for range n {
		info := cpu.Decode(d.console.Peek(addr))
		disasm := info.Name + operandText(d.console.CPU, d.console, addr, info)
		out = append(out, fmt.Sprintf("%04X  %-9s%s", addr, rawBytes(d.console, addr, info.Size), disasm))
		addr += uint16(info.Size)
	}
	return out
}
