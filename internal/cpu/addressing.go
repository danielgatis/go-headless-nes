package cpu

// Addressing-mode resolution. fetchOperand runs before the instruction and
// leaves the resolved operand in c.operand:
// an address for memory modes, an immediate value for Imm/Rel, or 0 for
// modes the instruction resolves itself (Other) or that take no operand.

// fetchOperand resolves the current instruction's addressing mode.
func (c *CPU) fetchOperand() uint16 {
	switch c.instAddrMode {
	case modeAcc, modeImp:
		c.dummyPCRead()
		return 0
	case modeImm, modeRel:
		return uint16(c.getImmediate())
	case modeZero:
		return uint16(c.getZeroAddr())
	case modeZeroX:
		return uint16(c.getZeroXAddr())
	case modeZeroY:
		return uint16(c.getZeroYAddr())
	case modeInd:
		return c.getIndAddr()
	case modeIndX:
		return c.getIndXAddr()
	case modeIndY:
		return c.getIndYAddr(false)
	case modeIndYW:
		return c.getIndYAddr(true)
	case modeAbs:
		return c.getAbsAddr()
	case modeAbsX:
		return c.getAbsXAddr(false)
	case modeAbsXW:
		return c.getAbsXAddr(true)
	case modeAbsY:
		return c.getAbsYAddr(false)
	case modeAbsYW:
		return c.getAbsYAddr(true)
	case modeOther:
		return 0 // handled specifically by the instruction
	default:
		return 0
	}
}

// getOperand returns the resolved operand (address or immediate value).
func (c *CPU) getOperand() uint16 { return c.operand }

// getOperandValue returns the operand's value: memory modes read through
// the effective address, register/immediate modes return it directly.
func (c *CPU) getOperandValue() byte {
	if c.instAddrMode >= modeZero {
		return c.read(c.getOperand())
	}
	return byte(c.getOperand())
}

func (c *CPU) getImmediate() byte { return c.readByte() }
func (c *CPU) getZeroAddr() byte  { return c.readByte() }
func (c *CPU) getAbsAddr() uint16 { return c.readPCWord() }
func (c *CPU) getIndAddr() uint16 { return c.readPCWord() }

func (c *CPU) getZeroXAddr() byte {
	v := c.readByte()
	c.read(uint16(v)) // dummy read
	return v + c.Reg.X
}

func (c *CPU) getZeroYAddr() byte {
	v := c.readByte()
	c.read(uint16(v)) // dummy read
	return v + c.Reg.Y
}

func (c *CPU) getAbsXAddr(dummyRead bool) uint16 {
	base := c.readPCWord()
	crossed := pageCrossed(base, c.Reg.X)
	if crossed || dummyRead {
		off := base + uint16(c.Reg.X)
		if crossed {
			off -= 0x100
		}
		c.read(off) // dummy read
	}
	return base + uint16(c.Reg.X)
}

func (c *CPU) getAbsYAddr(dummyRead bool) uint16 {
	base := c.readPCWord()
	crossed := pageCrossed(base, c.Reg.Y)
	if crossed || dummyRead {
		off := base + uint16(c.Reg.Y)
		if crossed {
			off -= 0x100
		}
		c.read(off) // dummy read
	}
	return base + uint16(c.Reg.Y)
}

// getInd reads the indirect pointer for JMP ($nnnn), reproducing the
// page-wrap bug: the high byte comes from the same page as the low byte.
func (c *CPU) getInd() uint16 {
	addr := c.getOperand()
	if addr&0xFF == 0xFF {
		lo := c.read(addr)
		hi := c.read(addr - 0xFF)
		return uint16(lo) | uint16(hi)<<8
	}
	return c.readWord(addr)
}

func (c *CPU) getIndXAddr() uint16 {
	zero := c.readByte()
	c.read(uint16(zero)) // dummy read
	zero += c.Reg.X
	if zero == 0xFF {
		lo := c.read(0xFF)
		hi := c.read(0x00)
		return uint16(lo) | uint16(hi)<<8
	}
	return c.readWord(uint16(zero))
}

func (c *CPU) getIndYAddr(dummyRead bool) uint16 {
	zero := c.readByte()
	var addr uint16
	if zero == 0xFF {
		lo := c.read(0xFF)
		hi := c.read(0x00)
		addr = uint16(lo) | uint16(hi)<<8
	} else {
		addr = c.readWord(uint16(zero))
	}
	crossed := pageCrossed(addr, c.Reg.Y)
	if crossed || dummyRead {
		off := addr + uint16(c.Reg.Y)
		if crossed {
			off -= 0x100
		}
		c.read(off) // dummy read
	}
	return addr + uint16(c.Reg.Y)
}
