package test

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/debugger"
	"github.com/danielgatis/go-headless-nes/test/assets"
	"github.com/danielgatis/go-headless-nes/test/expect"
	"github.com/danielgatis/go-headless-nes/test/harness"
)

// ppuColumn matches the "PPU:sl,dot" column of a nestest log line.
var ppuColumn = regexp.MustCompile(`PPU:\s*(\d+),\s*(\d+)`)

// nintendulatorToReferencePPU rewrites the log line's PPU column from the
// Nintendulator power-on alignment to this core's. The canonical
// nestest.log was captured on Nintendulator, whose CPU consumes a lump of
// 7 cycles at reset with the PPU at dot 3*7=21. This core reproduces the
// reference model instead: the CPU starts at cycle -1 / master clock 12
// with the PPU parked at (-1,340), and the eight clocked reset cycles
// leave the PPU 4 dots further along, at (0,25). Both machines then
// advance exactly 3 dots per CPU cycle (nestest never enables rendering,
// so the odd-frame dot skip is out of play), so the whole log differs by
// a constant +4 dots — everything else on the line must match exactly.
func nintendulatorToReferencePPU(line string) string {
	m := ppuColumn.FindStringSubmatchIndex(line)
	if m == nil {
		return line
	}
	sl, _ := strconv.Atoi(line[m[2]:m[3]])
	dot, _ := strconv.Atoi(line[m[4]:m[5]])
	total := sl*341 + dot + 4
	return line[:m[0]] + fmt.Sprintf("PPU:%3d,%3d", total/341, total%341) + line[m[1]:]
}

// Test_nestest runs nestest in its automated mode (entry $C000) and
// compares every executed instruction against the canonical log,
// including registers, the cycle counter and the PPU dot clock. gones
// calls c.Trace(); this emulator renders the same nestest log line through
// debugger.Trace over the CPU, bus and PPU position.
func Test_nestest(t *testing.T) {
	t.Parallel()

	nestest, err := assets.ROMs.Open("roms/other/nestest.nes")
	expect.Require.NoError(t, err)

	nestestLog, err := assets.ROMs.Open("roms/other/nestest.log")
	expect.Require.NoError(t, err)

	c, err := harness.StubConsole(nestest)
	expect.Require.NoError(t, err)

	c.CPU.Reg.PC = 0xC000

	scanner := bufio.NewScanner(nestestLog)
	var totalLines, checkedLines uint
	for scanner.Scan() {
		want := scanner.Text()
		if want == "" {
			continue
		}
		totalLines++
		checkedLines++

		actual := debugger.Trace(c.CPU, c, int(c.PPU.Scanline), int(c.PPU.Cycle))
		expect.Require.Equal(t, nintendulatorToReferencePPU(want), actual)

		c.Step()
	}
	expect.Require.NoError(t, scanner.Err())

	expect.Assert.Equal(t, totalLines, checkedLines)
	expect.Assert.EqualValues(t, 0, c.Peek(2),
		"official opcode failures report in $02; see nestest.txt",
	)
	expect.Assert.EqualValues(t, 0, c.Peek(3),
		"unofficial opcode failures report in $03; see nestest.txt",
	)
}
