package test

import (
	"bytes"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
	"github.com/danielgatis/go-headless-nes/internal/controller"
	"github.com/danielgatis/go-headless-nes/internal/nes"
	"github.com/danielgatis/go-headless-nes/test/assets"
	"github.com/danielgatis/go-headless-nes/test/expect"
)

// AccuracyCoin (100thCoin) is a single ROM of 141 sub-tests plus a few
// informational "DRAW" screens, covering CPU/PPU/APU/DMA/interrupt corner
// cases far stricter than blargg's suite. It is controller-driven: on the
// main menu, Start runs every test and tallies the results.
//
// Zero-page landmarks (from AccuracyCoin.asm):
//
//	$35 RunningAllTests   1 while the run-all routine executes, 0 on the
//	                      menu and again once results are tallied
//	$37 PostAllTestTally  after the run: total number of tests counted
//	$38 PostAllPassTally  after the run: number of tests that passed
const (
	acRunningAllTests  = 0x35
	acPostAllTestTally = 0x37
	acPostAllPassTally = 0x38
)

// accuracyCoinBaseline is the number of AccuracyCoin sub-tests this
// emulator currently passes: all of them. Any drop is a regression.
const accuracyCoinBaseline = 141

// TestAccuracyCoin boots AccuracyCoin, presses Start to run every test,
// waits for the run to finish, and reports the pass tally.
func TestAccuracyCoin(t *testing.T) {
	t.Parallel()

	cart, err := cartridge.Load(bytes.NewReader(assets.AccuracyCoin))
	expect.Require.NoError(t, err)
	console, err := nes.New(cart)
	expect.Require.NoError(t, err)

	const (
		firstPress = 20    // let boot/power-on tests reach the menu
		maxFrames  = 20000 // ~5.5 emulated minutes; a hang trips this
		settle     = 60    // let the results screen finish tallying
	)

	started, finished := false, false
	frame := 0
	for ; frame < maxFrames; frame++ {
		switch {
		case !started:
			// Pulse Start (two frames on, two off) until the run-all
			// routine engages, so we do not depend on the exact frame
			// the menu becomes ready.
			if frame >= firstPress && frame%4 < 2 {
				console.Controllers[0].SetButtons(controller.Start)
			} else {
				console.Controllers[0].SetButtons(0)
			}
			if console.Peek(acRunningAllTests) != 0 {
				started = true
				console.Controllers[0].SetButtons(0)
			}
		case console.Peek(acRunningAllTests) == 0:
			// Run-all finished; give the results screen time to tally.
			for range settle {
				console.RunFrame()
			}
			finished = true
		}
		if finished {
			break
		}
		console.RunFrame()
	}

	expect.Require.True(t, started, "run-all never engaged (Start press not registered?)")
	expect.Require.True(t, finished, "AccuracyCoin did not finish within %d frames", maxFrames)

	passed := console.Peek(acPostAllPassTally)
	total := console.Peek(acPostAllTestTally)
	t.Logf("AccuracyCoin: %d/%d sub-tests passed (baseline %d)", passed, total, accuracyCoinBaseline)

	expect.Require.NotZero(t, total, "tally not populated; landmark addresses wrong?")
	expect.Require.GreaterOrEqualf(t, int(passed), accuracyCoinBaseline,
		"pass count regressed below baseline %d", accuracyCoinBaseline)
}
