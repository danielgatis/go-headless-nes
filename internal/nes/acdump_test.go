package nes

import "testing"

// Temporary diagnostic: dump which AccuracyCoin sub-tests fail (value&1==0).
var acResultNames = map[uint16]string{
	0x400: "Unimplemented", 0x401: "CPUInstr", 0x403: "RAMMirror", 0x404: "PPURegMirror",
	0x405: "ROMnotWritable", 0x406: "DummyReads", 0x407: "DummyWrites", 0x408: "OpenBus",
	0x409: "SLO_03", 0x40A: "SLO_07", 0x40B: "SLO_0F", 0x40C: "SLO_13", 0x40D: "SLO_17",
	0x40E: "SLO_1B", 0x40F: "SLO_1F", 0x410: "ANC_0B", 0x411: "ANC_2B", 0x412: "ASR_4B",
	0x413: "ARR_6B", 0x414: "ANE_8B", 0x415: "LXA_AB", 0x416: "AXS_CB", 0x417: "SBC_EB",
	0x419: "RLA_23", 0x41A: "RLA_27", 0x41B: "RLA_2F", 0x41C: "RLA_33", 0x41D: "RLA_37",
	0x41E: "RLA_3B", 0x41F: "RLA_3F", 0x420: "SRE_43", 0x47F: "SRE_47", 0x422: "SRE_4F",
	0x423: "SRE_53", 0x424: "SRE_57", 0x425: "SRE_5B", 0x426: "SRE_5F", 0x427: "RRA_63",
	0x428: "RRA_67", 0x429: "RRA_6F", 0x42A: "RRA_73", 0x42B: "RRA_77", 0x42C: "RRA_7B",
	0x42D: "RRA_7F", 0x42E: "SAX_83", 0x42F: "SAX_87", 0x430: "SAX_8F", 0x431: "SAX_97",
	0x432: "LAX_A3", 0x433: "LAX_A7", 0x434: "LAX_AF", 0x435: "LAX_B3", 0x436: "LAX_B7",
	0x437: "LAX_BF", 0x438: "DCP_C3", 0x439: "DCP_C7", 0x43A: "DCP_CF", 0x43B: "DCP_D3",
	0x43C: "DCP_D7", 0x43D: "DCP_DB", 0x43E: "DCP_DF", 0x43F: "ISC_E3", 0x440: "ISC_E7",
	0x441: "ISC_EF", 0x442: "ISC_F3", 0x443: "ISC_F7", 0x444: "ISC_FB", 0x445: "ISC_FF",
	0x446: "SHA_93", 0x447: "SHA_9F", 0x448: "SHS_9B", 0x449: "SHY_9C", 0x44A: "SHX_9E",
	0x44B: "LAE_BB", 0x44C: "DMA_Plus_2007R", 0x44D: "PC_Wraparound", 0x44E: "PPUOpenBus",
	0x44F: "DMA_Plus_2007W", 0x450: "VBlank_Beginning", 0x451: "VBlank_End", 0x452: "NMI_Control",
	0x453: "NMI_Timing", 0x454: "NMI_Suppression", 0x455: "NMI_VBL_End", 0x456: "NMI_Disabled_VBL_Start",
	0x457: "Sprite0Hit_Behavior", 0x458: "ArbitrarySpriteZero", 0x459: "SprOverflow_Behavior",
	0x45A: "MisalignedOAM_Behavior", 0x45B: "Address2004_Behavior", 0x45C: "APURegActivation",
	0x45D: "DMA_Plus_4015R", 0x45E: "DMA_Plus_4016R", 0x45F: "ControllerStrobing",
	0x460: "InstructionTiming", 0x461: "IFlagLatency", 0x462: "NmiAndBrk", 0x463: "NmiAndIrq",
	0x465: "APULengthCounter", 0x466: "APULengthTable", 0x467: "FrameCounterIRQ",
	0x468: "FrameCounter4Step", 0x469: "FrameCounter5Step", 0x46A: "DeltaModulationChannel",
	0x46B: "DMABusConflict", 0x46C: "DMA_Plus_OpenBus", 0x46D: "ImpliedDummyRead",
	0x46E: "AddrMode_AbsIndex", 0x46F: "AddrMode_ZPgIndex", 0x470: "AddrMode_Indirect",
	0x471: "AddrMode_IndIndeX", 0x472: "AddrMode_IndIndeY", 0x473: "AddrMode_Relative",
	0x474: "DecimalFlag", 0x475: "BFlag", 0x476: "PPUReadBuffer", 0x477: "DMCDMAPlusOAMDMA",
	0x478: "ImplicitDMAAbort", 0x479: "ExplicitDMAAbort", 0x47A: "ControllerClocking",
	0x47B: "OAM_Corruption", 0x47C: "JSREdgeCases", 0x47D: "AllNOPs", 0x47E: "PaletteRAMQuirks",
	0x480: "INC4014", 0x481: "AttributesAsTiles", 0x482: "tRegisterQuirks",
	0x483: "StaleBGShiftRegisters", 0x484: "Scanline0Sprites", 0x485: "CHRROMIsNotWritable",
	0x486: "RenderingFlagBehavior", 0x487: "BGSerialIn", 0x488: "DMA_Plus_2002R",
	0x489: "SuddenlyResizeSprite", 0x48A: "Rendering2007Read", 0x48B: "BranchDummyRead",
	0x48C: "2004_Stress", 0x48D: "2002FlagClearTiming", 0x48E: "2007_Stress",
	0x48F: "StaleSpriteShiftRegs", 0x490: "InternalDataBus", 0x491: "ALERead", 0x492: "HybridAddresses",
}

func TestAccuracyCoinDump(t *testing.T) {
	cart := loadCart(t, "../../test/roms-accuracycoin/AccuracyCoin.nes")
	c, err := New(cart)
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	started, finished := false, false
	for frame := 0; frame < 20000 && !finished; frame++ {
		switch {
		case !started:
			if frame >= 20 && frame%4 < 2 {
				c.Controllers[0].SetButtons(acStart)
			} else {
				c.Controllers[0].SetButtons(0)
			}
			if c.Peek(0x35) != 0 {
				started = true
				c.Controllers[0].SetButtons(0)
			}
		case c.Peek(0x35) == 0:
			for i := 0; i < 60; i++ {
				c.RunFrame()
			}
			finished = true
		}
		if finished {
			break
		}
		c.RunFrame()
	}
	if !finished {
		t.Fatal("did not finish")
	}
	t.Logf("tally: %d/%d", c.Peek(0x38), c.Peek(0x37))
	for addr := uint16(0x400); addr <= 0x492; addr++ {
		name, ok := acResultNames[addr]
		if !ok {
			continue
		}
		v := c.Peek(addr)
		if v&1 == 0 {
			t.Logf("FAIL $%04X %-24s = $%02X", addr, name, v)
		}
	}
}
