package test

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/danielgatis/go-headless-nes/test/assets"
	"github.com/danielgatis/go-headless-nes/test/expect"
	"github.com/danielgatis/go-headless-nes/test/harness"
)

// smokeFrames is how long each ROM is exercised. 120 frames (~2 s) is
// enough for boot, mapper setup, and the first stretch of the main loop
// to run, where a broken mapper or PPU path tends to crash.
const smokeFrames = 120

// smokeSkip lists ROMs the smoke suite cannot run: mapper 28 (Action 53
// multicart) is not implemented, so the console rejects them at New.
var smokeSkip = map[string]string{
	"roms/other/test28.nes":           "mapper 28 (Action 53) not implemented",
	"roms/other/Streemerz_bundle.nes": "mapper 28 (Action 53) not implemented",
}

// TestSmoke boots every test ROM and runs it for a couple of seconds,
// asserting only that it loads and runs without crashing. It is the
// coverage floor for the ROMs that carry no automated pass/fail result
// (demos, visual tools, audio tests) and a cheap regression net for the
// rest: a mapper or PPU change that panics on boot trips here even when
// no targeted test exists.
func TestSmoke(t *testing.T) {
	t.Parallel()

	var roms []string
	err := fs.WalkDir(assets.ROMs, "roms", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".nes") {
			roms = append(roms, p)
		}
		return nil
	})
	expect.Require.NoError(t, err)
	sort.Strings(roms)

	for _, rp := range roms {
		t.Run(rp, func(t *testing.T) {
			t.Parallel()
			if reason, skip := smokeSkip[rp]; skip {
				t.Skip(reason)
			}
			f, err := assets.ROMs.Open(rp)
			expect.Require.NoError(t, err)
			defer func() { _ = f.Close() }()
			expect.Require.NoError(t, harness.Smoke(f, smokeFrames))
		})
	}
}
