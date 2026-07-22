package test

import (
	"testing"

	"github.com/danielgatis/go-headless-nes/test/assets"
	"github.com/danielgatis/go-headless-nes/test/expect"
	"github.com/danielgatis/go-headless-nes/test/harness"
)

// S and P are shorthands for the two blargg result channels, keeping the
// test tables readable.
const (
	S = harness.MsgTypeSRAM
	P = harness.MsgTypePPUVRAM
)

// blarggCase is one row of a blargg-style ROM test: the ROM path, the
// channel it reports on, and the expected status and message.
type blarggCase struct {
	name       string
	rom        string
	msgType    harness.MsgType
	wantStatus harness.Status
	want       string
}

// runBlarggTable runs a table of blargg cases as parallel sub-tests, each
// asserting the ROM's final status and message. It is the single shared
// driver every blargg_*_test.go uses, so the suites stay identical in
// shape and only their tables differ.
func runBlarggTable(t *testing.T, cases []blarggCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rom, err := assets.ROMs.Open(tt.rom)
			expect.Require.NoError(t, err)

			test, err := harness.NewBlarggTest(rom, tt.msgType)
			expect.Require.NoError(t, err)

			expect.Require.NoError(t, test.Run())
			expect.Assert.Equal(t, tt.wantStatus, harness.GetStatus(test))
			expect.Assert.Equal(t, tt.want, harness.GetMessage(test, tt.msgType))
		})
	}
}

// frameCase is one row for the older blargg PPU ROMs that report by
// writing a result byte to PPU VRAM after a fixed number of rendered
// frames, rather than using the $6000 status protocol.
type frameCase struct {
	name        string
	rom         string
	renderCount int
	want        string
}

// runFrameTable runs a table of frame-count cases: each ROM renders
// renderCount frames, then its PPU-VRAM message is asserted.
func runFrameTable(t *testing.T, cases []frameCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rom, err := assets.ROMs.Open(tt.rom)
			expect.Require.NoError(t, err)

			test, err := harness.NewConsoleTest(rom, harness.ExitAfterFrameNum(tt.renderCount))
			expect.Require.NoError(t, err)

			expect.Require.NoError(t, test.Run())
			expect.Assert.Equal(t, tt.want, harness.GetMessage(test, P))
		})
	}
}
