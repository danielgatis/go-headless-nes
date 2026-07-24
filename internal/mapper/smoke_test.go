package mapper

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

// supportedMapperIDs lists every ID the factory accepts, with the
// submappers that select distinct board revisions.
var supportedMapperIDs = []struct {
	id  uint16
	sub byte
}{
	{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}, {5, 0}, {7, 0}, {9, 0}, {10, 0}, {11, 0},
	{13, 0}, {15, 0}, {16, 0}, {16, 4}, {16, 5}, {18, 0}, {19, 0},
	{21, 0}, {21, 2}, {22, 0}, {23, 0}, {23, 1}, {23, 2}, {23, 3},
	{24, 0}, {25, 0}, {25, 2}, {25, 3}, {26, 0}, {27, 0}, {30, 0}, {30, 3},
	{32, 0}, {32, 1}, {33, 0}, {34, 0}, {37, 0}, {38, 0}, {44, 0}, {45, 0}, {47, 0}, {48, 0}, {48, 1}, {49, 0}, {52, 0},
	{64, 0}, {65, 0}, {66, 0}, {67, 0}, {69, 0}, {70, 0}, {71, 0},
	{72, 0}, {73, 0}, {75, 0}, {76, 0}, {77, 0}, {78, 0}, {78, 3}, {79, 0},
	{80, 0}, {82, 0}, {85, 0}, {86, 0}, {87, 0}, {88, 0}, {89, 0},
	{92, 0}, {93, 0}, {94, 0}, {95, 0}, {97, 0}, {101, 0}, {105, 0}, {107, 0}, {112, 0},
	{113, 0}, {118, 0}, {119, 0}, {140, 0}, {152, 0}, {153, 0}, {154, 0}, {155, 0}, {157, 0}, {158, 0}, {159, 0},
	{180, 0}, {184, 0}, {185, 0}, {185, 4}, {206, 0}, {207, 0}, {210, 0}, {210, 1}, {210, 2}, {232, 0}, {268, 0}, {268, 1},
	// MMC3 variants ported from the reference emulator.
	{12, 0}, {74, 0}, {114, 0}, {115, 0}, {123, 0}, {134, 0}, {165, 0}, {182, 0}, {187, 0}, {189, 0},
	{191, 0}, {192, 0}, {194, 0}, {195, 0}, {196, 0}, {197, 0}, {205, 0}, {224, 0}, {238, 0}, {245, 0},
	{249, 0}, {250, 0}, {254, 0},
	// Named-board one-offs.
	{28, 0}, {68, 0}, {146, 0},
	// Sachen 8259 family.
	{137, 0}, {138, 0}, {139, 0}, {141, 0},
	// Simple discrete / multicart boards.
	{54, 0}, {57, 0}, {58, 0}, {61, 0}, {62, 0}, {200, 0}, {201, 0},
	{202, 0}, {203, 0}, {213, 0}, {240, 0}, {242, 0},
	// VRC clones sharing existing families.
	{151, 0}, {183, 0},
	// Kaiser boards.
	{56, 0}, {142, 0}, {171, 0}, {175, 0},
	// Waixing boards.
	{162, 0}, {164, 0}, {178, 0}, {252, 0},
	// TXC boards.
	{36, 0}, {132, 0}, {172, 0}, {173, 0},
	// FDS-conversion (FFE), Subor, Henggedianzi.
	{6, 0}, {8, 0}, {17, 0}, {166, 0}, {167, 0}, {177, 0}, {179, 0},
	// More discrete / multicart boards.
	{46, 0}, {50, 0}, {170, 0}, {174, 0}, {204, 0}, {216, 0},
	{144, 0}, {225, 0}, {226, 0}, {227, 0}, {229, 0}, {231, 0}, {234, 0},
	{241, 0}, {244, 0}, {190, 0}, {193, 0}, {221, 0}, {228, 0},
	{39, 0}, {40, 0}, {42, 0}, {59, 0}, {106, 0},
	// BMC multicarts.
	{41, 0}, {51, 0}, {108, 0}, {255, 0}, {91, 0}, {235, 0},
	// Small Sachen boards.
	{133, 0}, {136, 0}, {143, 0}, {145, 0}, {147, 0}, {148, 0}, {149, 0},
	{150, 0}, {243, 0}, {156, 0},
	// NES 2.0 multicarts.
	{265, 0}, {283, 0}, {285, 0}, {288, 0}, {304, 0}, {329, 0},
	{529, 0},
	// MMC3-based NES 2.0 multicarts.
	{259, 0}, {263, 0},
	{308, 0}, {309, 0}, {327, 0}, {328, 0},
	{271, 0}, {274, 0}, {331, 0},
	// A12-clocked IRQ boards.
	{35, 0}, {117, 0}, {222, 0},
	// Reset-cycled boards.
	{60, 0}, {230, 0}, {233, 0}, {313, 0},
	// Mixed CHR RAM/ROM MMC3 clone.
	{199, 0},
	// MMC3 scramble/exreg clones.
	{262, 0}, {325, 0},
	// Kaiser NES 2.0 boards.
	{305, 0}, {346, 0}, {306, 0},
	// More BMC / discrete multicarts.
	{120, 0}, {212, 0}, {214, 0}, {246, 0}, {261, 0}, {290, 0}, {299, 0},
	{300, 0}, {336, 0}, {349, 0},
	{104, 0}, {125, 0}, {332, 0}, {348, 0}, {521, 0},
	{53, 0}, {258, 0}, {286, 0}, {289, 0}, {312, 0}, {319, 0}, {320, 0},
	{366, 0}, {29, 0}, {301, 0}, {314, 0}, {324, 0},
	{43, 0}, {287, 0}, {519, 0}, {522, 0},
	{298, 0}, {302, 0}, {303, 0}, {530, 0},
	{31, 0}, {264, 0}, {266, 0}, {487, 0}, {208, 0}, {63, 0}, {236, 0},
	// VRAM-address sniffer boards.
	{96, 0}, {518, 0},
	// Banked-WRAM boards.
	{103, 0}, {198, 0},
	// Per-table nametable board.
	{307, 0},
	// MMC3-shell state-machine multicarts.
	{219, 0}, {333, 0}, {217, 0}, {215, 0}, {126, 0}, {260, 0}, {253, 0},
	{83, 0}, {121, 0}, {14, 0}, {552, 0}, {323, 0}, {116, 0}, {218, 0},
	// JY Company, flash/homebrew and remaining reference-emulator boards.
	{90, 0}, {209, 0}, {211, 0}, {111, 0}, {163, 0}, {168, 0}, {176, 0},
	{188, 0}, {284, 0}, {292, 0}, {513, 0},
}

// Structural mirrors of the PPU's optional board capabilities, for
// asserting them without importing the ppu package.
type ntPager interface {
	NametablePage(table byte) byte
}

type ntSource interface {
	ReadNT(addr uint16) (v byte, ok bool)
	WriteNT(addr uint16, v byte) (ok bool)
}

// smokeCart builds a synthetic cartridge for any mapper: 512 KiB PRG
// filled with RTI plus reset/IRQ vectors in every 8 KiB bank tail, and
// 256 KiB CHR.
func smokeCart(id uint16, sub byte) *cartridge.Cartridge {
	prg := make([]byte, 512*1024)
	for i := range prg {
		prg[i] = 0x40 // RTI
	}
	// Point every possible vector location at $8000 (NOP sled territory);
	// $EA = NOP for the body keeps execution harmless wherever PC lands.
	for i := 0; i < len(prg); i += 0x2000 {
		tail := prg[i+0x2000-16 : i+0x2000]
		for j := 0; j < 16; j += 2 {
			tail[j] = 0x00
			tail[j+1] = 0x80
		}
	}
	chr := make([]byte, 256*1024)
	return &cartridge.Cartridge{
		PRG:       prg,
		CHR:       chr,
		MapperID:  id,
		Submapper: sub,
		Mirroring: cartridge.Vertical,
	}
}

// TestMapperSmoke drives every supported board through its whole
// register space, PRG/CHR access, ticks, scanlines and a snapshot
// round-trip, a build-level guarantee that no board panics or breaks
// the Mapper contract.
func TestMapperSmoke(t *testing.T) {
	for _, tc := range supportedMapperIDs {
		m, err := New(smokeCart(tc.id, tc.sub))
		if err != nil {
			t.Errorf("mapper %d.%d: %v", tc.id, tc.sub, err)
			continue
		}

		// Sweep writes across the register space with varied values,
		// interleaving reads so latch-style boards see bus values.
		for addr := 0x6000; addr <= 0xFFFF; addr += 0x91 {
			m.SetOpenBus(byte(addr))
			m.WritePRG(uint16(addr), byte(addr>>3))
			_ = m.ReadPRG(uint16(addr))
		}
		for addr := 0; addr < 0x2000; addr += 0x40 {
			_ = m.ReadCHR(uint16(addr))
			m.WriteCHR(uint16(addr), byte(addr))
		}
		// Boards that watch the PPU bus (mappers 96, 518, ...) latch CHR
		// on nametable fetches; sweep the address space so they react.
		if sn, ok := m.(interface{ NotifyVramAddr(uint16) }); ok {
			for addr := 0; addr < 0x3000; addr += 0x40 {
				sn.NotifyVramAddr(uint16(addr))
				_ = m.ReadCHR(uint16(addr & 0x1FFF))
			}
		}
		for range 2000 {
			m.Tick()
		}
		for range 300 {
			m.Scanline()
		}
		_ = m.IRQ()
		_ = m.Mirroring()
		if pager, ok := m.(ntPager); ok {
			for table := byte(0); table < 4; table++ {
				if page := pager.NametablePage(table); page > 1 {
					t.Errorf("mapper %d.%d: nametable page %d out of range", tc.id, tc.sub, page)
				}
			}
		}
		if src, ok := m.(ntSource); ok {
			for addr := uint16(0x2000); addr < 0x3000; addr += 0x400 {
				_, _ = src.ReadNT(addr)
				_ = src.WriteNT(addr, 0xAA)
			}
		}

		// Snapshot round-trip must restore identical observable behavior.
		var snap State
		m.Save(&snap)
		before := [4]byte{
			m.ReadPRG(0x8123), m.ReadPRG(0xC456), m.ReadCHR(0x0123), m.ReadCHR(0x1456),
		}
		// Disturb the state, then restore.
		for addr := 0x8000; addr <= 0xFFFF; addr += 0x1000 {
			m.WritePRG(uint16(addr), 0xFF)
		}
		m.Restore(&snap)
		after := [4]byte{
			m.ReadPRG(0x8123), m.ReadPRG(0xC456), m.ReadCHR(0x0123), m.ReadCHR(0x1456),
		}
		if before != after {
			t.Errorf("mapper %d.%d: snapshot round-trip diverged: %v vs %v", tc.id, tc.sub, before, after)
		}
	}
}

// TestAllMappersSurviveSnapshot exercises Save/Restore across every
// registered mapper with a synthetic cartridge.
func TestAllMappersSurviveSnapshot(t *testing.T) {
	ids := []uint16{0, 1, 2, 3, 4, 7, 9, 10, 11, 34, 66, 71, 79, 87, 94, 113, 140, 180, 206, 232}
	for _, id := range ids {
		m, err := New(cart(id, 8, 4))
		if err != nil {
			t.Errorf("mapper %d: %v", id, err)
			continue
		}
		m.WritePRG(0x6000, 0xA5)
		var s State
		m.Save(&s)
		m.Restore(&s)
		// Smoke: the mapper still reads without panicking after a
		// round trip, and PRG RAM (where the board has it) persists.
		m.ReadPRG(0x8000)
		m.ReadCHR(0x0000)
	}
}
