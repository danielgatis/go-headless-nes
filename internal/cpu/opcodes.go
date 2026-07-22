package cpu

// Opcode and addressing-mode tables.
// maps to an instruction method and an addressing mode; fetchOperand
// (cpu.go) resolves the mode into c.operand before the instruction runs.

// addrMode is a 6502 addressing mode, mirroring the NesAddrMode enum.
// Order matters: modes >= modeZero take a memory operand (getOperandValue
// reads through memory), the earlier ones are register/immediate.
type addrMode byte

const (
	modeNone addrMode = iota
	modeAcc
	modeImp
	modeImm
	modeRel
	modeZero
	modeZeroX
	modeZeroY
	modeInd
	modeIndX
	modeIndY  // ($nn),Y read (dummy read only on page cross)
	modeIndYW // ($nn),Y write (always dummy read)
	modeAbs
	modeAbsX  // abs,X read
	modeAbsXW // abs,X write (always dummy read)
	modeAbsY  // abs,Y read
	modeAbsYW // abs,Y write (always dummy read)
	modeOther // handled specially by the instruction
)

// instr is a CPU instruction method.
type instr func(*CPU)

// opTable maps opcode -> instruction handler.
var opTable = [256]instr{
	(*CPU).brk,    // 0x00 brk
	(*CPU).ora,    // 0x01 ora
	(*CPU).hlt,    // 0x02 hlt
	(*CPU).slo,    // 0x03 slo
	(*CPU).nop,    // 0x04 nop
	(*CPU).ora,    // 0x05 ora
	(*CPU).aslMem, // 0x06 aslMem
	(*CPU).slo,    // 0x07 slo
	(*CPU).php,    // 0x08 php
	(*CPU).ora,    // 0x09 ora
	(*CPU).aslAcc, // 0x0A aslAcc
	(*CPU).aac,    // 0x0B aac
	(*CPU).nop,    // 0x0C nop
	(*CPU).ora,    // 0x0D ora
	(*CPU).aslMem, // 0x0E aslMem
	(*CPU).slo,    // 0x0F slo
	(*CPU).bpl,    // 0x10 bpl
	(*CPU).ora,    // 0x11 ora
	(*CPU).hlt,    // 0x12 hlt
	(*CPU).slo,    // 0x13 slo
	(*CPU).nop,    // 0x14 nop
	(*CPU).ora,    // 0x15 ora
	(*CPU).aslMem, // 0x16 aslMem
	(*CPU).slo,    // 0x17 slo
	(*CPU).clc,    // 0x18 clc
	(*CPU).ora,    // 0x19 ora
	(*CPU).nop,    // 0x1A nop
	(*CPU).slo,    // 0x1B slo
	(*CPU).nop,    // 0x1C nop
	(*CPU).ora,    // 0x1D ora
	(*CPU).aslMem, // 0x1E aslMem
	(*CPU).slo,    // 0x1F slo
	(*CPU).jsr,    // 0x20 jsr
	(*CPU).and,    // 0x21 and
	(*CPU).hlt,    // 0x22 hlt
	(*CPU).rla,    // 0x23 rla
	(*CPU).bit,    // 0x24 bit
	(*CPU).and,    // 0x25 and
	(*CPU).rolMem, // 0x26 rolMem
	(*CPU).rla,    // 0x27 rla
	(*CPU).plp,    // 0x28 plp
	(*CPU).and,    // 0x29 and
	(*CPU).rolAcc, // 0x2A rolAcc
	(*CPU).aac,    // 0x2B aac
	(*CPU).bit,    // 0x2C bit
	(*CPU).and,    // 0x2D and
	(*CPU).rolMem, // 0x2E rolMem
	(*CPU).rla,    // 0x2F rla
	(*CPU).bmi,    // 0x30 bmi
	(*CPU).and,    // 0x31 and
	(*CPU).hlt,    // 0x32 hlt
	(*CPU).rla,    // 0x33 rla
	(*CPU).nop,    // 0x34 nop
	(*CPU).and,    // 0x35 and
	(*CPU).rolMem, // 0x36 rolMem
	(*CPU).rla,    // 0x37 rla
	(*CPU).sec,    // 0x38 sec
	(*CPU).and,    // 0x39 and
	(*CPU).nop,    // 0x3A nop
	(*CPU).rla,    // 0x3B rla
	(*CPU).nop,    // 0x3C nop
	(*CPU).and,    // 0x3D and
	(*CPU).rolMem, // 0x3E rolMem
	(*CPU).rla,    // 0x3F rla
	(*CPU).rti,    // 0x40 rti
	(*CPU).eor,    // 0x41 eor
	(*CPU).hlt,    // 0x42 hlt
	(*CPU).sre,    // 0x43 sre
	(*CPU).nop,    // 0x44 nop
	(*CPU).eor,    // 0x45 eor
	(*CPU).lsrMem, // 0x46 lsrMem
	(*CPU).sre,    // 0x47 sre
	(*CPU).pha,    // 0x48 pha
	(*CPU).eor,    // 0x49 eor
	(*CPU).lsrAcc, // 0x4A lsrAcc
	(*CPU).asr,    // 0x4B asr
	(*CPU).jmpAbs, // 0x4C jmpAbs
	(*CPU).eor,    // 0x4D eor
	(*CPU).lsrMem, // 0x4E lsrMem
	(*CPU).sre,    // 0x4F sre
	(*CPU).bvc,    // 0x50 bvc
	(*CPU).eor,    // 0x51 eor
	(*CPU).hlt,    // 0x52 hlt
	(*CPU).sre,    // 0x53 sre
	(*CPU).nop,    // 0x54 nop
	(*CPU).eor,    // 0x55 eor
	(*CPU).lsrMem, // 0x56 lsrMem
	(*CPU).sre,    // 0x57 sre
	(*CPU).cli,    // 0x58 cli
	(*CPU).eor,    // 0x59 eor
	(*CPU).nop,    // 0x5A nop
	(*CPU).sre,    // 0x5B sre
	(*CPU).nop,    // 0x5C nop
	(*CPU).eor,    // 0x5D eor
	(*CPU).lsrMem, // 0x5E lsrMem
	(*CPU).sre,    // 0x5F sre
	(*CPU).rts,    // 0x60 rts
	(*CPU).adc,    // 0x61 adc
	(*CPU).hlt,    // 0x62 hlt
	(*CPU).rra,    // 0x63 rra
	(*CPU).nop,    // 0x64 nop
	(*CPU).adc,    // 0x65 adc
	(*CPU).rorMem, // 0x66 rorMem
	(*CPU).rra,    // 0x67 rra
	(*CPU).pla,    // 0x68 pla
	(*CPU).adc,    // 0x69 adc
	(*CPU).rorAcc, // 0x6A rorAcc
	(*CPU).arr,    // 0x6B arr
	(*CPU).jmpInd, // 0x6C jmpInd
	(*CPU).adc,    // 0x6D adc
	(*CPU).rorMem, // 0x6E rorMem
	(*CPU).rra,    // 0x6F rra
	(*CPU).bvs,    // 0x70 bvs
	(*CPU).adc,    // 0x71 adc
	(*CPU).hlt,    // 0x72 hlt
	(*CPU).rra,    // 0x73 rra
	(*CPU).nop,    // 0x74 nop
	(*CPU).adc,    // 0x75 adc
	(*CPU).rorMem, // 0x76 rorMem
	(*CPU).rra,    // 0x77 rra
	(*CPU).sei,    // 0x78 sei
	(*CPU).adc,    // 0x79 adc
	(*CPU).nop,    // 0x7A nop
	(*CPU).rra,    // 0x7B rra
	(*CPU).nop,    // 0x7C nop
	(*CPU).adc,    // 0x7D adc
	(*CPU).rorMem, // 0x7E rorMem
	(*CPU).rra,    // 0x7F rra
	(*CPU).nop,    // 0x80 nop
	(*CPU).sta,    // 0x81 sta
	(*CPU).nop,    // 0x82 nop
	(*CPU).sax,    // 0x83 sax
	(*CPU).sty,    // 0x84 sty
	(*CPU).sta,    // 0x85 sta
	(*CPU).stx,    // 0x86 stx
	(*CPU).sax,    // 0x87 sax
	(*CPU).dey,    // 0x88 dey
	(*CPU).nop,    // 0x89 nop
	(*CPU).txa,    // 0x8A txa
	(*CPU).ane,    // 0x8B ane
	(*CPU).sty,    // 0x8C sty
	(*CPU).sta,    // 0x8D sta
	(*CPU).stx,    // 0x8E stx
	(*CPU).sax,    // 0x8F sax
	(*CPU).bcc,    // 0x90 bcc
	(*CPU).sta,    // 0x91 sta
	(*CPU).hlt,    // 0x92 hlt
	(*CPU).shaz,   // 0x93 shaz
	(*CPU).sty,    // 0x94 sty
	(*CPU).sta,    // 0x95 sta
	(*CPU).stx,    // 0x96 stx
	(*CPU).sax,    // 0x97 sax
	(*CPU).tya,    // 0x98 tya
	(*CPU).sta,    // 0x99 sta
	(*CPU).txs,    // 0x9A txs
	(*CPU).tas,    // 0x9B tas
	(*CPU).shy,    // 0x9C shy
	(*CPU).sta,    // 0x9D sta
	(*CPU).shx,    // 0x9E shx
	(*CPU).shaa,   // 0x9F shaa
	(*CPU).ldy,    // 0xA0 ldy
	(*CPU).lda,    // 0xA1 lda
	(*CPU).ldx,    // 0xA2 ldx
	(*CPU).lax,    // 0xA3 lax
	(*CPU).ldy,    // 0xA4 ldy
	(*CPU).lda,    // 0xA5 lda
	(*CPU).ldx,    // 0xA6 ldx
	(*CPU).lax,    // 0xA7 lax
	(*CPU).tay,    // 0xA8 tay
	(*CPU).lda,    // 0xA9 lda
	(*CPU).tax,    // 0xAA tax
	(*CPU).atx,    // 0xAB atx
	(*CPU).ldy,    // 0xAC ldy
	(*CPU).lda,    // 0xAD lda
	(*CPU).ldx,    // 0xAE ldx
	(*CPU).lax,    // 0xAF lax
	(*CPU).bcs,    // 0xB0 bcs
	(*CPU).lda,    // 0xB1 lda
	(*CPU).hlt,    // 0xB2 hlt
	(*CPU).lax,    // 0xB3 lax
	(*CPU).ldy,    // 0xB4 ldy
	(*CPU).lda,    // 0xB5 lda
	(*CPU).ldx,    // 0xB6 ldx
	(*CPU).lax,    // 0xB7 lax
	(*CPU).clv,    // 0xB8 clv
	(*CPU).lda,    // 0xB9 lda
	(*CPU).tsx,    // 0xBA tsx
	(*CPU).las,    // 0xBB las
	(*CPU).ldy,    // 0xBC ldy
	(*CPU).lda,    // 0xBD lda
	(*CPU).ldx,    // 0xBE ldx
	(*CPU).lax,    // 0xBF lax
	(*CPU).cpy,    // 0xC0 cpy
	(*CPU).cpa,    // 0xC1 cpa
	(*CPU).nop,    // 0xC2 nop
	(*CPU).dcp,    // 0xC3 dcp
	(*CPU).cpy,    // 0xC4 cpy
	(*CPU).cpa,    // 0xC5 cpa
	(*CPU).dec,    // 0xC6 dec
	(*CPU).dcp,    // 0xC7 dcp
	(*CPU).iny,    // 0xC8 iny
	(*CPU).cpa,    // 0xC9 cpa
	(*CPU).dex,    // 0xCA dex
	(*CPU).axs,    // 0xCB axs
	(*CPU).cpy,    // 0xCC cpy
	(*CPU).cpa,    // 0xCD cpa
	(*CPU).dec,    // 0xCE dec
	(*CPU).dcp,    // 0xCF dcp
	(*CPU).bne,    // 0xD0 bne
	(*CPU).cpa,    // 0xD1 cpa
	(*CPU).hlt,    // 0xD2 hlt
	(*CPU).dcp,    // 0xD3 dcp
	(*CPU).nop,    // 0xD4 nop
	(*CPU).cpa,    // 0xD5 cpa
	(*CPU).dec,    // 0xD6 dec
	(*CPU).dcp,    // 0xD7 dcp
	(*CPU).cld,    // 0xD8 cld
	(*CPU).cpa,    // 0xD9 cpa
	(*CPU).nop,    // 0xDA nop
	(*CPU).dcp,    // 0xDB dcp
	(*CPU).nop,    // 0xDC nop
	(*CPU).cpa,    // 0xDD cpa
	(*CPU).dec,    // 0xDE dec
	(*CPU).dcp,    // 0xDF dcp
	(*CPU).cpx,    // 0xE0 cpx
	(*CPU).sbc,    // 0xE1 sbc
	(*CPU).nop,    // 0xE2 nop
	(*CPU).isb,    // 0xE3 isb
	(*CPU).cpx,    // 0xE4 cpx
	(*CPU).sbc,    // 0xE5 sbc
	(*CPU).inc,    // 0xE6 inc
	(*CPU).isb,    // 0xE7 isb
	(*CPU).inx,    // 0xE8 inx
	(*CPU).sbc,    // 0xE9 sbc
	(*CPU).nop,    // 0xEA nop
	(*CPU).sbc,    // 0xEB sbc
	(*CPU).cpx,    // 0xEC cpx
	(*CPU).sbc,    // 0xED sbc
	(*CPU).inc,    // 0xEE inc
	(*CPU).isb,    // 0xEF isb
	(*CPU).beq,    // 0xF0 beq
	(*CPU).sbc,    // 0xF1 sbc
	(*CPU).nop,    // 0xF2 nop
	(*CPU).isb,    // 0xF3 isb
	(*CPU).nop,    // 0xF4 nop
	(*CPU).sbc,    // 0xF5 sbc
	(*CPU).inc,    // 0xF6 inc
	(*CPU).isb,    // 0xF7 isb
	(*CPU).sed,    // 0xF8 sed
	(*CPU).sbc,    // 0xF9 sbc
	(*CPU).nop,    // 0xFA nop
	(*CPU).isb,    // 0xFB isb
	(*CPU).nop,    // 0xFC nop
	(*CPU).sbc,    // 0xFD sbc
	(*CPU).inc,    // 0xFE inc
	(*CPU).isb,    // 0xFF isb
}

// addrModes maps opcode -> addressing mode, transcribed row-for-row.
var addrModes = [256]addrMode{
	modeImp,   // 0x00
	modeIndX,  // 0x01
	modeNone,  // 0x02
	modeIndX,  // 0x03
	modeZero,  // 0x04
	modeZero,  // 0x05
	modeZero,  // 0x06
	modeZero,  // 0x07
	modeImp,   // 0x08
	modeImm,   // 0x09
	modeAcc,   // 0x0A
	modeImm,   // 0x0B
	modeAbs,   // 0x0C
	modeAbs,   // 0x0D
	modeAbs,   // 0x0E
	modeAbs,   // 0x0F
	modeRel,   // 0x10
	modeIndY,  // 0x11
	modeNone,  // 0x12
	modeIndYW, // 0x13
	modeZeroX, // 0x14
	modeZeroX, // 0x15
	modeZeroX, // 0x16
	modeZeroX, // 0x17
	modeImp,   // 0x18
	modeAbsY,  // 0x19
	modeImp,   // 0x1A
	modeAbsYW, // 0x1B
	modeAbsX,  // 0x1C
	modeAbsX,  // 0x1D
	modeAbsXW, // 0x1E
	modeAbsXW, // 0x1F
	modeOther, // 0x20
	modeIndX,  // 0x21
	modeNone,  // 0x22
	modeIndX,  // 0x23
	modeZero,  // 0x24
	modeZero,  // 0x25
	modeZero,  // 0x26
	modeZero,  // 0x27
	modeImp,   // 0x28
	modeImm,   // 0x29
	modeAcc,   // 0x2A
	modeImm,   // 0x2B
	modeAbs,   // 0x2C
	modeAbs,   // 0x2D
	modeAbs,   // 0x2E
	modeAbs,   // 0x2F
	modeRel,   // 0x30
	modeIndY,  // 0x31
	modeNone,  // 0x32
	modeIndYW, // 0x33
	modeZeroX, // 0x34
	modeZeroX, // 0x35
	modeZeroX, // 0x36
	modeZeroX, // 0x37
	modeImp,   // 0x38
	modeAbsY,  // 0x39
	modeImp,   // 0x3A
	modeAbsYW, // 0x3B
	modeAbsX,  // 0x3C
	modeAbsX,  // 0x3D
	modeAbsXW, // 0x3E
	modeAbsXW, // 0x3F
	modeImp,   // 0x40
	modeIndX,  // 0x41
	modeNone,  // 0x42
	modeIndX,  // 0x43
	modeZero,  // 0x44
	modeZero,  // 0x45
	modeZero,  // 0x46
	modeZero,  // 0x47
	modeImp,   // 0x48
	modeImm,   // 0x49
	modeAcc,   // 0x4A
	modeImm,   // 0x4B
	modeAbs,   // 0x4C
	modeAbs,   // 0x4D
	modeAbs,   // 0x4E
	modeAbs,   // 0x4F
	modeRel,   // 0x50
	modeIndY,  // 0x51
	modeNone,  // 0x52
	modeIndYW, // 0x53
	modeZeroX, // 0x54
	modeZeroX, // 0x55
	modeZeroX, // 0x56
	modeZeroX, // 0x57
	modeImp,   // 0x58
	modeAbsY,  // 0x59
	modeImp,   // 0x5A
	modeAbsYW, // 0x5B
	modeAbsX,  // 0x5C
	modeAbsX,  // 0x5D
	modeAbsXW, // 0x5E
	modeAbsXW, // 0x5F
	modeImp,   // 0x60
	modeIndX,  // 0x61
	modeNone,  // 0x62
	modeIndX,  // 0x63
	modeZero,  // 0x64
	modeZero,  // 0x65
	modeZero,  // 0x66
	modeZero,  // 0x67
	modeImp,   // 0x68
	modeImm,   // 0x69
	modeAcc,   // 0x6A
	modeImm,   // 0x6B
	modeInd,   // 0x6C
	modeAbs,   // 0x6D
	modeAbs,   // 0x6E
	modeAbs,   // 0x6F
	modeRel,   // 0x70
	modeIndY,  // 0x71
	modeNone,  // 0x72
	modeIndYW, // 0x73
	modeZeroX, // 0x74
	modeZeroX, // 0x75
	modeZeroX, // 0x76
	modeZeroX, // 0x77
	modeImp,   // 0x78
	modeAbsY,  // 0x79
	modeImp,   // 0x7A
	modeAbsYW, // 0x7B
	modeAbsX,  // 0x7C
	modeAbsX,  // 0x7D
	modeAbsXW, // 0x7E
	modeAbsXW, // 0x7F
	modeImm,   // 0x80
	modeIndX,  // 0x81
	modeImm,   // 0x82
	modeIndX,  // 0x83
	modeZero,  // 0x84
	modeZero,  // 0x85
	modeZero,  // 0x86
	modeZero,  // 0x87
	modeImp,   // 0x88
	modeImm,   // 0x89
	modeImp,   // 0x8A
	modeImm,   // 0x8B
	modeAbs,   // 0x8C
	modeAbs,   // 0x8D
	modeAbs,   // 0x8E
	modeAbs,   // 0x8F
	modeRel,   // 0x90
	modeIndYW, // 0x91
	modeNone,  // 0x92
	modeOther, // 0x93
	modeZeroX, // 0x94
	modeZeroX, // 0x95
	modeZeroY, // 0x96
	modeZeroY, // 0x97
	modeImp,   // 0x98
	modeAbsYW, // 0x99
	modeImp,   // 0x9A
	modeOther, // 0x9B
	modeOther, // 0x9C
	modeAbsXW, // 0x9D
	modeOther, // 0x9E
	modeOther, // 0x9F
	modeImm,   // 0xA0
	modeIndX,  // 0xA1
	modeImm,   // 0xA2
	modeIndX,  // 0xA3
	modeZero,  // 0xA4
	modeZero,  // 0xA5
	modeZero,  // 0xA6
	modeZero,  // 0xA7
	modeImp,   // 0xA8
	modeImm,   // 0xA9
	modeImp,   // 0xAA
	modeImm,   // 0xAB
	modeAbs,   // 0xAC
	modeAbs,   // 0xAD
	modeAbs,   // 0xAE
	modeAbs,   // 0xAF
	modeRel,   // 0xB0
	modeIndY,  // 0xB1
	modeNone,  // 0xB2
	modeIndY,  // 0xB3
	modeZeroX, // 0xB4
	modeZeroX, // 0xB5
	modeZeroY, // 0xB6
	modeZeroY, // 0xB7
	modeImp,   // 0xB8
	modeAbsY,  // 0xB9
	modeImp,   // 0xBA
	modeAbsY,  // 0xBB
	modeAbsX,  // 0xBC
	modeAbsX,  // 0xBD
	modeAbsY,  // 0xBE
	modeAbsY,  // 0xBF
	modeImm,   // 0xC0
	modeIndX,  // 0xC1
	modeImm,   // 0xC2
	modeIndX,  // 0xC3
	modeZero,  // 0xC4
	modeZero,  // 0xC5
	modeZero,  // 0xC6
	modeZero,  // 0xC7
	modeImp,   // 0xC8
	modeImm,   // 0xC9
	modeImp,   // 0xCA
	modeImm,   // 0xCB
	modeAbs,   // 0xCC
	modeAbs,   // 0xCD
	modeAbs,   // 0xCE
	modeAbs,   // 0xCF
	modeRel,   // 0xD0
	modeIndY,  // 0xD1
	modeNone,  // 0xD2
	modeIndYW, // 0xD3
	modeZeroX, // 0xD4
	modeZeroX, // 0xD5
	modeZeroX, // 0xD6
	modeZeroX, // 0xD7
	modeImp,   // 0xD8
	modeAbsY,  // 0xD9
	modeImp,   // 0xDA
	modeAbsYW, // 0xDB
	modeAbsX,  // 0xDC
	modeAbsX,  // 0xDD
	modeAbsXW, // 0xDE
	modeAbsXW, // 0xDF
	modeImm,   // 0xE0
	modeIndX,  // 0xE1
	modeImm,   // 0xE2
	modeIndX,  // 0xE3
	modeZero,  // 0xE4
	modeZero,  // 0xE5
	modeZero,  // 0xE6
	modeZero,  // 0xE7
	modeImp,   // 0xE8
	modeImm,   // 0xE9
	modeImp,   // 0xEA
	modeImm,   // 0xEB
	modeAbs,   // 0xEC
	modeAbs,   // 0xED
	modeAbs,   // 0xEE
	modeAbs,   // 0xEF
	modeRel,   // 0xF0
	modeIndY,  // 0xF1
	modeNone,  // 0xF2
	modeIndYW, // 0xF3
	modeZeroX, // 0xF4
	modeZeroX, // 0xF5
	modeZeroX, // 0xF6
	modeZeroX, // 0xF7
	modeImp,   // 0xF8
	modeAbsY,  // 0xF9
	modeImp,   // 0xFA
	modeAbsYW, // 0xFB
	modeAbsX,  // 0xFC
	modeAbsX,  // 0xFD
	modeAbsXW, // 0xFE
	modeAbsXW, // 0xFF
}
