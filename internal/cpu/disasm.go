package cpu

// Disassembly metadata for the debugger/tracer. The execution core uses
// the function-pointer opTable, which carries no mnemonic or size, so
// this file provides a parallel exported surface (Mode, Info, Decode)
// derived from the same opcode byte.

// Mode is an exported addressing mode for disassembly. It mirrors the
// classic 6502 modes the tracer switches on.
type Mode byte

// The 6502 addressing modes, in the tracer's canonical order.
const (
	Implied Mode = iota
	Accumulator
	Immediate
	ZeroPage
	ZeroPageX
	ZeroPageY
	Absolute
	AbsoluteX
	AbsoluteY
	Indirect
	IndexedIndirect // ($nn,X)
	IndirectIndexed // ($nn),Y
	Relative
)

// operandBytes is the number of operand bytes for an exported Mode.
var operandBytes = [...]int{
	Implied: 0, Accumulator: 0,
	Immediate: 1, ZeroPage: 1, ZeroPageX: 1, ZeroPageY: 1,
	Relative: 1, IndexedIndirect: 1, IndirectIndexed: 1,
	Absolute: 2, AbsoluteX: 2, AbsoluteY: 2, Indirect: 2,
}

// Info describes an opcode for disassemblers and debuggers.
type Info struct {
	Name       string
	Mode       Mode
	Size       int // total instruction length in bytes
	Unofficial bool
}

// Decode returns the decode-table entry for an opcode byte.
func Decode(code byte) Info {
	name, unofficial := opNames[code], opUnofficial[code]
	m := exportMode(addrModes[code])
	// modeOther opcodes (JSR, the $9x SHA/SHX/SHY/TAS group) resolve their
	// own operands, so they carry no export mode; give them their real
	// visible mode for disassembly.
	if addrModes[code] == modeOther {
		switch code {
		case 0x20: // JSR
			m = Absolute
		case 0x93, 0x9B, 0x9F: // SHAZ (indirect,Y), TAS/SHAA (abs,Y)
			m = IndirectIndexed
			if code != 0x93 {
				m = AbsoluteY
			}
		case 0x9C: // SHY abs,X
			m = AbsoluteX
		case 0x9E: // SHX abs,Y
			m = AbsoluteY
		}
	}
	return Info{
		Name:       name,
		Mode:       m,
		Size:       1 + operandBytes[m],
		Unofficial: unofficial,
	}
}

// exportMode maps an internal addrMode to the exported disassembly Mode.
// Read/write variants collapse to the same visible mode.
func exportMode(m addrMode) Mode {
	switch m {
	case modeAcc:
		return Accumulator
	case modeImm:
		return Immediate
	case modeRel:
		return Relative
	case modeZero:
		return ZeroPage
	case modeZeroX:
		return ZeroPageX
	case modeZeroY:
		return ZeroPageY
	case modeInd:
		return Indirect
	case modeIndX:
		return IndexedIndirect
	case modeIndY, modeIndYW:
		return IndirectIndexed
	case modeAbs:
		return Absolute
	case modeAbsX, modeAbsXW:
		return AbsoluteX
	case modeAbsY, modeAbsYW:
		return AbsoluteY
	default: // modeImp, modeNone, modeOther
		return Implied
	}
}

// opNames is the mnemonic for each opcode (illegal ops named per nestest's
// disassembly conventions).
var opNames = [256]string{
	"BRK", "ORA", "KIL", "SLO", "NOP", "ORA", "ASL", "SLO", "PHP", "ORA", "ASL", "ANC", "NOP", "ORA", "ASL", "SLO",
	"BPL", "ORA", "KIL", "SLO", "NOP", "ORA", "ASL", "SLO", "CLC", "ORA", "NOP", "SLO", "NOP", "ORA", "ASL", "SLO",
	"JSR", "AND", "KIL", "RLA", "BIT", "AND", "ROL", "RLA", "PLP", "AND", "ROL", "ANC", "BIT", "AND", "ROL", "RLA",
	"BMI", "AND", "KIL", "RLA", "NOP", "AND", "ROL", "RLA", "SEC", "AND", "NOP", "RLA", "NOP", "AND", "ROL", "RLA",
	"RTI", "EOR", "KIL", "SRE", "NOP", "EOR", "LSR", "SRE", "PHA", "EOR", "LSR", "ALR", "JMP", "EOR", "LSR", "SRE",
	"BVC", "EOR", "KIL", "SRE", "NOP", "EOR", "LSR", "SRE", "CLI", "EOR", "NOP", "SRE", "NOP", "EOR", "LSR", "SRE",
	"RTS", "ADC", "KIL", "RRA", "NOP", "ADC", "ROR", "RRA", "PLA", "ADC", "ROR", "ARR", "JMP", "ADC", "ROR", "RRA",
	"BVS", "ADC", "KIL", "RRA", "NOP", "ADC", "ROR", "RRA", "SEI", "ADC", "NOP", "RRA", "NOP", "ADC", "ROR", "RRA",
	"NOP", "STA", "NOP", "SAX", "STY", "STA", "STX", "SAX", "DEY", "NOP", "TXA", "XAA", "STY", "STA", "STX", "SAX",
	"BCC", "STA", "KIL", "AHX", "STY", "STA", "STX", "SAX", "TYA", "STA", "TXS", "TAS", "SHY", "STA", "SHX", "AHX",
	"LDY", "LDA", "LDX", "LAX", "LDY", "LDA", "LDX", "LAX", "TAY", "LDA", "TAX", "LAX", "LDY", "LDA", "LDX", "LAX",
	"BCS", "LDA", "KIL", "LAX", "LDY", "LDA", "LDX", "LAX", "CLV", "LDA", "TSX", "LAS", "LDY", "LDA", "LDX", "LAX",
	"CPY", "CMP", "NOP", "DCP", "CPY", "CMP", "DEC", "DCP", "INY", "CMP", "DEX", "AXS", "CPY", "CMP", "DEC", "DCP",
	"BNE", "CMP", "KIL", "DCP", "NOP", "CMP", "DEC", "DCP", "CLD", "CMP", "NOP", "DCP", "NOP", "CMP", "DEC", "DCP",
	"CPX", "SBC", "NOP", "ISB", "CPX", "SBC", "INC", "ISB", "INX", "SBC", "NOP", "SBC", "CPX", "SBC", "INC", "ISB",
	"BEQ", "SBC", "KIL", "ISB", "NOP", "SBC", "INC", "ISB", "SED", "SBC", "NOP", "ISB", "NOP", "SBC", "INC", "ISB",
}

// opUnofficial marks illegal opcodes for the disassembler's '*' prefix,
// matching nestest's reference log exactly (only $EA is the official NOP;
// $EB is an unofficial SBC alias; all *_illegal columns are unofficial).
var opUnofficial = [256]bool{
	0x02: true, 0x03: true, 0x04: true, 0x07: true, 0x0B: true, 0x0C: true, 0x0F: true,
	0x12: true, 0x13: true, 0x14: true, 0x17: true, 0x1A: true, 0x1B: true, 0x1C: true, 0x1F: true,
	0x22: true, 0x23: true, 0x27: true, 0x2B: true, 0x2F: true,
	0x32: true, 0x33: true, 0x34: true, 0x37: true, 0x3A: true, 0x3B: true, 0x3C: true, 0x3F: true,
	0x42: true, 0x43: true, 0x44: true, 0x47: true, 0x4B: true, 0x4F: true,
	0x52: true, 0x53: true, 0x54: true, 0x57: true, 0x5A: true, 0x5B: true, 0x5C: true, 0x5F: true,
	0x62: true, 0x63: true, 0x64: true, 0x67: true, 0x6B: true, 0x6F: true,
	0x72: true, 0x73: true, 0x74: true, 0x77: true, 0x7A: true, 0x7B: true, 0x7C: true, 0x7F: true,
	0x80: true, 0x82: true, 0x83: true, 0x87: true, 0x89: true, 0x8B: true, 0x8F: true,
	0x92: true, 0x93: true, 0x97: true, 0x9B: true, 0x9C: true, 0x9E: true, 0x9F: true,
	0xA3: true, 0xA7: true, 0xAB: true, 0xAF: true,
	0xB2: true, 0xB3: true, 0xB7: true, 0xBB: true, 0xBF: true,
	0xC2: true, 0xC3: true, 0xC7: true, 0xCB: true, 0xCF: true,
	0xD2: true, 0xD3: true, 0xD4: true, 0xD7: true, 0xDA: true, 0xDB: true, 0xDC: true, 0xDF: true,
	0xE2: true, 0xE3: true, 0xE7: true, 0xEB: true, 0xEF: true,
	0xF2: true, 0xF3: true, 0xF4: true, 0xF7: true, 0xFA: true, 0xFB: true, 0xFC: true, 0xFF: true,
}
