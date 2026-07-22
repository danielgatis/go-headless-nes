package cpu

// Instruction implementations, a direct translation of the op-code
// table. Each is a no-arg method matching an entry
// in opTable; the resolved operand lives in c.operand (see addressing.go).

func (c *CPU) setPS(v byte) { c.Reg.P = v&0xCF | FlagU }

// --- Arithmetic / logic helpers ---

func (c *CPU) add(value byte) {
	carry := byte(0)
	if c.checkFlag(FlagC) {
		carry = 1
	}
	result := uint16(c.Reg.A) + uint16(value) + uint16(carry)

	c.clearFlags(FlagC | FlagN | FlagV | FlagZ)
	c.setZeroNeg(byte(result))
	if ^(c.Reg.A^value)&(c.Reg.A^byte(result))&0x80 != 0 {
		c.setFlags(FlagV)
	}
	if result > 0xFF {
		c.setFlags(FlagC)
	}
	c.setA(byte(result))
}

func (c *CPU) compare(reg, value byte) {
	c.clearFlags(FlagC | FlagN | FlagZ)
	result := reg - value
	if reg >= value {
		c.setFlags(FlagC)
	}
	if reg == value {
		c.setFlags(FlagZ)
	}
	if result&0x80 == 0x80 {
		c.setFlags(FlagN)
	}
}

func (c *CPU) doASL(value byte) byte {
	c.clearFlags(FlagC | FlagN | FlagZ)
	if value&0x80 != 0 {
		c.setFlags(FlagC)
	}
	result := value << 1
	c.setZeroNeg(result)
	return result
}

func (c *CPU) doLSR(value byte) byte {
	c.clearFlags(FlagC | FlagN | FlagZ)
	if value&0x01 != 0 {
		c.setFlags(FlagC)
	}
	result := value >> 1
	c.setZeroNeg(result)
	return result
}

func (c *CPU) doROL(value byte) byte {
	carry := c.checkFlag(FlagC)
	c.clearFlags(FlagC | FlagN | FlagZ)
	if value&0x80 != 0 {
		c.setFlags(FlagC)
	}
	result := value << 1
	if carry {
		result |= 0x01
	}
	c.setZeroNeg(result)
	return result
}

func (c *CPU) doROR(value byte) byte {
	carry := c.checkFlag(FlagC)
	c.clearFlags(FlagC | FlagN | FlagZ)
	if value&0x01 != 0 {
		c.setFlags(FlagC)
	}
	result := value >> 1
	if carry {
		result |= 0x80
	}
	c.setZeroNeg(result)
	return result
}

// --- Logical ops ---

func (c *CPU) and() { c.setA(c.Reg.A & c.getOperandValue()) }
func (c *CPU) eor() { c.setA(c.Reg.A ^ c.getOperandValue()) }
func (c *CPU) ora() { c.setA(c.Reg.A | c.getOperandValue()) }

func (c *CPU) adc() { c.add(c.getOperandValue()) }
func (c *CPU) sbc() { c.add(c.getOperandValue() ^ 0xFF) }

func (c *CPU) cpa() { c.compare(c.Reg.A, c.getOperandValue()) }
func (c *CPU) cpx() { c.compare(c.Reg.X, c.getOperandValue()) }
func (c *CPU) cpy() { c.compare(c.Reg.Y, c.getOperandValue()) }

func (c *CPU) bit() {
	value := c.getOperandValue()
	c.clearFlags(FlagZ | FlagV | FlagN)
	if c.Reg.A&value == 0 {
		c.setFlags(FlagZ)
	}
	if value&0x40 != 0 {
		c.setFlags(FlagV)
	}
	if value&0x80 != 0 {
		c.setFlags(FlagN)
	}
}

// --- Loads / stores / transfers ---

func (c *CPU) lda() { c.setA(c.getOperandValue()) }
func (c *CPU) ldx() { c.setX(c.getOperandValue()) }
func (c *CPU) ldy() { c.setY(c.getOperandValue()) }

func (c *CPU) sta() { c.write(c.getOperand(), c.Reg.A) }
func (c *CPU) stx() { c.write(c.getOperand(), c.Reg.X) }
func (c *CPU) sty() { c.write(c.getOperand(), c.Reg.Y) }

func (c *CPU) tax() { c.setX(c.Reg.A) }
func (c *CPU) tay() { c.setY(c.Reg.A) }
func (c *CPU) tsx() { c.setX(c.Reg.SP) }
func (c *CPU) txa() { c.setA(c.Reg.X) }
func (c *CPU) txs() { c.Reg.SP = c.Reg.X }
func (c *CPU) tya() { c.setA(c.Reg.Y) }

func (c *CPU) inx() { c.setX(c.Reg.X + 1) }
func (c *CPU) iny() { c.setY(c.Reg.Y + 1) }
func (c *CPU) dex() { c.setX(c.Reg.X - 1) }
func (c *CPU) dey() { c.setY(c.Reg.Y - 1) }

// --- Stack ops ---

func (c *CPU) pha() { c.push(c.Reg.A) }

func (c *CPU) php() {
	c.push(c.Reg.P | FlagB | FlagU)
}

func (c *CPU) pla() {
	c.dummyStackRead()
	c.setA(c.pop())
}

func (c *CPU) plp() {
	c.dummyStackRead()
	c.setPS(c.pop())
}

// --- Read-modify-write memory ops ---

func (c *CPU) inc() {
	addr := c.getOperand()
	c.clearFlags(FlagN | FlagZ)
	value := c.read(addr)
	c.write(addr, value) // dummy write (original value)
	value++
	c.setZeroNeg(value)
	c.writeRMWSecond(addr, value)
}

func (c *CPU) dec() {
	addr := c.getOperand()
	c.clearFlags(FlagN | FlagZ)
	value := c.read(addr)
	c.write(addr, value) // dummy write
	value--
	c.setZeroNeg(value)
	c.writeRMWSecond(addr, value)
}

func (c *CPU) aslMem() {
	addr := c.getOperand()
	value := c.read(addr)
	c.write(addr, value) // dummy write
	c.writeRMWSecond(addr, c.doASL(value))
}

func (c *CPU) lsrMem() {
	addr := c.getOperand()
	value := c.read(addr)
	c.write(addr, value)
	c.writeRMWSecond(addr, c.doLSR(value))
}

func (c *CPU) rolMem() {
	addr := c.getOperand()
	value := c.read(addr)
	c.write(addr, value)
	c.writeRMWSecond(addr, c.doROL(value))
}

func (c *CPU) rorMem() {
	addr := c.getOperand()
	value := c.read(addr)
	c.write(addr, value)
	c.writeRMWSecond(addr, c.doROR(value))
}

func (c *CPU) aslAcc() { c.setA(c.doASL(c.Reg.A)) }
func (c *CPU) lsrAcc() { c.setA(c.doLSR(c.Reg.A)) }
func (c *CPU) rolAcc() { c.setA(c.doROL(c.Reg.A)) }
func (c *CPU) rorAcc() { c.setA(c.doROR(c.Reg.A)) }

// --- Jumps / calls ---

func (c *CPU) jmpAbs() { c.Reg.PC = c.getOperand() }
func (c *CPU) jmpInd() { c.Reg.PC = c.getInd() }

func (c *CPU) jsr() {
	lo := c.readByte()
	c.dummyStackRead()
	c.push16(c.Reg.PC)
	addr := uint16(c.readByte())<<8 | uint16(lo)
	c.Reg.PC = addr
}

func (c *CPU) rts() {
	c.dummyStackRead()
	addr := c.popWord()
	c.Reg.PC = addr
	c.dummyPCRead()
	c.Reg.PC = addr + 1
}

func (c *CPU) rti() {
	c.dummyStackRead()
	c.setPS(c.pop())
	c.Reg.PC = c.popWord()
}

// --- Branches ---

func (c *CPU) branchRelative(branch bool) {
	offset := int8(c.getOperand())
	if branch {
		// "a taken non-page-crossing branch ignores IRQ/NMI during its last
		// clock, so the next instruction executes before the IRQ."
		c.runIRQ = c.prevRunIRQ

		c.dummyPCRead()

		if pageCrossedSigned(c.Reg.PC, offset) {
			c.runIRQ = c.runIRQ || c.prevRunIRQ
			c.read(c.Reg.PC&0xFF00 | (c.Reg.PC+uint16(int16(offset)))&0xFF)
		}
		c.Reg.PC += uint16(int16(offset))
	}
}

func (c *CPU) bcc() { c.branchRelative(!c.checkFlag(FlagC)) }
func (c *CPU) bcs() { c.branchRelative(c.checkFlag(FlagC)) }
func (c *CPU) beq() { c.branchRelative(c.checkFlag(FlagZ)) }
func (c *CPU) bmi() { c.branchRelative(c.checkFlag(FlagN)) }
func (c *CPU) bne() { c.branchRelative(!c.checkFlag(FlagZ)) }
func (c *CPU) bpl() { c.branchRelative(!c.checkFlag(FlagN)) }
func (c *CPU) bvc() { c.branchRelative(!c.checkFlag(FlagV)) }
func (c *CPU) bvs() { c.branchRelative(c.checkFlag(FlagV)) }

// --- Flag ops ---

func (c *CPU) clc() { c.clearFlags(FlagC) }
func (c *CPU) cld() { c.clearFlags(FlagD) }
func (c *CPU) cli() { c.clearFlags(FlagI) }
func (c *CPU) clv() { c.clearFlags(FlagV) }
func (c *CPU) sec() { c.setFlags(FlagC) }
func (c *CPU) sed() { c.setFlags(FlagD) }
func (c *CPU) sei() { c.setFlags(FlagI) }

// --- BRK / NOP ---

func (c *CPU) brk() {
	c.push16(c.Reg.PC + 1)

	flags := c.Reg.P | FlagB | FlagU
	if c.needNMI {
		c.needNMI = false
		c.push(flags)
		c.setFlags(FlagI)
		c.Reg.PC = c.readWord(VecNMI)
	} else {
		c.push(flags)
		c.setFlags(FlagI)
		c.Reg.PC = c.readWord(VecIRQ)
	}

	// Don't start an NMI right after a BRK: the first instruction of the
	// IRQ handler must run first (nmi_and_brk test).
	c.prevNeedNMI = false
}

func (c *CPU) nop() {
	// Take as many cycles as the addressing mode implies.
	c.getOperandValue()
}

// --- Unofficial ops ---

func (c *CPU) slo() {
	value := c.getOperandValue()
	c.write(c.getOperand(), value) // dummy write
	shifted := c.doASL(value)
	c.setA(c.Reg.A | shifted)
	c.writeRMWSecond(c.getOperand(), shifted)
}

func (c *CPU) sre() {
	value := c.getOperandValue()
	c.write(c.getOperand(), value)
	shifted := c.doLSR(value)
	c.setA(c.Reg.A ^ shifted)
	c.writeRMWSecond(c.getOperand(), shifted)
}

func (c *CPU) rla() {
	value := c.getOperandValue()
	c.write(c.getOperand(), value)
	shifted := c.doROL(value)
	c.setA(c.Reg.A & shifted)
	c.writeRMWSecond(c.getOperand(), shifted)
}

func (c *CPU) rra() {
	value := c.getOperandValue()
	c.write(c.getOperand(), value)
	shifted := c.doROR(value)
	c.add(shifted)
	c.writeRMWSecond(c.getOperand(), shifted)
}

func (c *CPU) sax() { c.write(c.getOperand(), c.Reg.A&c.Reg.X) }

func (c *CPU) lax() {
	value := c.getOperandValue()
	c.setX(value)
	c.setA(value)
}

func (c *CPU) dcp() {
	value := c.getOperandValue()
	c.write(c.getOperand(), value)
	value--
	c.compare(c.Reg.A, value)
	c.writeRMWSecond(c.getOperand(), value)
}

func (c *CPU) isb() {
	value := c.getOperandValue()
	c.write(c.getOperand(), value)
	value++
	c.add(value ^ 0xFF)
	c.writeRMWSecond(c.getOperand(), value)
}

func (c *CPU) aac() {
	c.setA(c.Reg.A & c.getOperandValue())
	c.clearFlags(FlagC)
	if c.checkFlag(FlagN) {
		c.setFlags(FlagC)
	}
}

func (c *CPU) asr() {
	c.clearFlags(FlagC)
	c.setA(c.Reg.A & c.getOperandValue())
	if c.Reg.A&0x01 != 0 {
		c.setFlags(FlagC)
	}
	c.setA(c.Reg.A >> 1)
}

func (c *CPU) arr() {
	v := (c.Reg.A & c.getOperandValue()) >> 1
	if c.checkFlag(FlagC) {
		v |= 0x80
	}
	c.setA(v)
	c.clearFlags(FlagC | FlagV)
	if c.Reg.A&0x40 != 0 {
		c.setFlags(FlagC)
	}
	carry := byte(0)
	if c.checkFlag(FlagC) {
		carry = 1
	}
	if carry^((c.Reg.A>>5)&0x01) != 0 {
		c.setFlags(FlagV)
	}
}

func (c *CPU) atx() {
	value := c.getOperandValue()
	c.setA(value)
	c.setX(c.Reg.A)
	c.setA(c.Reg.A)
}

func (c *CPU) axs() {
	opValue := c.getOperandValue()
	value := (c.Reg.A & c.Reg.X) - opValue
	c.clearFlags(FlagC)
	if (c.Reg.A & c.Reg.X) >= opValue {
		c.setFlags(FlagC)
	}
	c.setX(value)
}

// syaSxaAxa is the unstable SHY/SHX/SHA/TAS store.
func (c *CPU) syaSxaAxa(baseAddr uint16, indexReg, valueReg byte) {
	crossed := pageCrossed(baseAddr, indexReg)

	cyc := c.Cycles
	off := baseAddr + uint16(indexReg)
	if crossed {
		off -= 0x100
	}
	c.read(off) // dummy read

	hadDMA := c.Cycles-cyc > 1

	operand := baseAddr + uint16(indexReg)
	addrHigh := byte(operand >> 8)
	addrLow := byte(operand)
	if crossed {
		addrHigh &= valueReg
	}

	var value byte
	if hadDMA {
		value = valueReg
	} else {
		value = valueReg & (byte(baseAddr>>8) + 1)
	}
	c.write(uint16(addrHigh)<<8|uint16(addrLow), value)
}

func (c *CPU) shy()  { c.syaSxaAxa(c.readPCWord(), c.Reg.X, c.Reg.Y) }
func (c *CPU) shx()  { c.syaSxaAxa(c.readPCWord(), c.Reg.Y, c.Reg.X) }
func (c *CPU) shaa() { c.syaSxaAxa(c.readPCWord(), c.Reg.Y, c.Reg.X&c.Reg.A) }

func (c *CPU) shaz() {
	zero := c.readByte()
	var baseAddr uint16
	if zero == 0xFF {
		lo := c.read(0xFF)
		hi := c.read(0x00)
		baseAddr = uint16(lo) | uint16(hi)<<8
	} else {
		baseAddr = c.readWord(uint16(zero))
	}
	c.syaSxaAxa(baseAddr, c.Reg.Y, c.Reg.X&c.Reg.A)
}

func (c *CPU) tas() {
	c.shaa()
	c.Reg.SP = c.Reg.X & c.Reg.A
}

func (c *CPU) ane() {
	imm := c.getOperandValue()
	c.setA((c.Reg.A | 0xEE) & c.Reg.X & imm)
}

func (c *CPU) las() {
	value := c.getOperandValue()
	c.setA(value & c.Reg.SP)
	c.setX(c.Reg.A)
	c.Reg.SP = c.Reg.A
}

// hlt freezes the CPU (invalid opcode). hardware re-executes it forever by
// decrementing PC; the crashed flag prevents repeated diagnostics.
func (c *CPU) hlt() {
	c.Reg.PC--
	c.prevRunIRQ = false
	c.prevNeedNMI = false
	c.crashed = true
	c.Jammed = true
}
