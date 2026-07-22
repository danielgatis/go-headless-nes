// Package debugger provides inspection tools built around the CPU's
// decode table: disassembly and execution tracing now, breakpoints and
// viewers as the debugger milestone lands.
package debugger

import (
	"fmt"
	"strings"

	"github.com/danielgatis/go-headless-nes/internal/cpu"
)

// Peeker reads memory without side effects. Tracing must never disturb
// the machine it is observing, so it cannot use the normal bus Read.
type Peeker interface {
	Peek(addr uint16) byte
}

// Trace formats the instruction at the CPU's PC in the classic nestest
// log format, one line per instruction:
//
//	C000  4C F5 C5  JMP $C5F5    A:00 X:00 Y:00 P:24 SP:FD PPU:  0, 21 CYC:7
//
// It must be called before the instruction executes: operand columns
// show memory as the instruction will see it.
func Trace(c *cpu.CPU, mem Peeker, ppuScanline, ppuDot int) string {
	pc := c.Reg.PC
	info := cpu.Decode(mem.Peek(pc))

	star := ' '
	if info.Unofficial {
		star = '*'
	}
	disasm := info.Name + operandText(c, mem, pc, info)

	return fmt.Sprintf("%04X  %-9s%c%-32sA:%02X X:%02X Y:%02X P:%02X SP:%02X PPU:%3d,%3d CYC:%d",
		pc, rawBytes(mem, pc, info.Size), star, disasm,
		c.Reg.A, c.Reg.X, c.Reg.Y, c.Reg.P, c.Reg.SP,
		ppuScanline, ppuDot, c.Cycles)
}

// operandText renders the operand column, including the effective
// address and current memory contents where the nestest format shows
// them. All address arithmetic reproduces the CPU's own quirks
// (zero-page wraparound, the JMP indirect page bug).
func operandText(c *cpu.CPU, mem Peeker, pc uint16, info cpu.Info) string {
	op1 := mem.Peek(pc + 1)
	op16 := uint16(mem.Peek(pc+2))<<8 | uint16(op1)

	switch info.Mode {
	case cpu.Implied:
		return ""
	case cpu.Accumulator:
		return " A"
	case cpu.Immediate:
		return fmt.Sprintf(" #$%02X", op1)
	case cpu.ZeroPage:
		return fmt.Sprintf(" $%02X = %02X", op1, mem.Peek(uint16(op1)))
	case cpu.ZeroPageX:
		addr := uint16(op1 + c.Reg.X)
		return fmt.Sprintf(" $%02X,X @ %02X = %02X", op1, addr, mem.Peek(addr))
	case cpu.ZeroPageY:
		addr := uint16(op1 + c.Reg.Y)
		return fmt.Sprintf(" $%02X,Y @ %02X = %02X", op1, addr, mem.Peek(addr))
	case cpu.Absolute:
		// Control flow targets are just addresses; everything else
		// shows what lives there.
		if info.Name == "JMP" || info.Name == "JSR" {
			return fmt.Sprintf(" $%04X", op16)
		}
		return fmt.Sprintf(" $%04X = %02X", op16, mem.Peek(op16))
	case cpu.AbsoluteX:
		addr := op16 + uint16(c.Reg.X)
		return fmt.Sprintf(" $%04X,X @ %04X = %02X", op16, addr, mem.Peek(addr))
	case cpu.AbsoluteY:
		addr := op16 + uint16(c.Reg.Y)
		return fmt.Sprintf(" $%04X,Y @ %04X = %02X", op16, addr, mem.Peek(addr))
	case cpu.Indirect:
		return fmt.Sprintf(" ($%04X) = %04X", op16, peek16PageWrap(mem, op16))
	case cpu.IndexedIndirect:
		ptr := op1 + c.Reg.X
		addr := peek16PageWrap(mem, uint16(ptr))
		return fmt.Sprintf(" ($%02X,X) @ %02X = %04X = %02X", op1, ptr, addr, mem.Peek(addr))
	case cpu.IndirectIndexed:
		base := peek16PageWrap(mem, uint16(op1))
		addr := base + uint16(c.Reg.Y)
		return fmt.Sprintf(" ($%02X),Y = %04X @ %04X = %02X", op1, base, addr, mem.Peek(addr))
	case cpu.Relative:
		return fmt.Sprintf(" $%04X", pc+2+uint16(int16(int8(op1))))
	}
	return ""
}

// rawBytes renders an instruction's bytes as space-separated hex.
func rawBytes(mem Peeker, pc uint16, size int) string {
	var sb strings.Builder
	for i := range uint16(size) {
		fmt.Fprintf(&sb, "%02X ", mem.Peek(pc+i))
	}
	return strings.TrimSuffix(sb.String(), " ")
}

// peek16PageWrap reads a pointer that cannot cross a page, matching the
// CPU's indirect addressing bug.
func peek16PageWrap(mem Peeker, addr uint16) uint16 {
	lo := uint16(mem.Peek(addr))
	hi := uint16(mem.Peek(addr&0xFF00 | uint16(byte(addr)+1)))
	return hi<<8 | lo
}
