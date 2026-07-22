package nes

import (
	"bytes"
	"os"
	"testing"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

func loadCart(t *testing.T, path string) *cartridge.Cartridge {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: test reads a ROM from a test-controlled path
	if err != nil {
		t.Skipf("ROM not found: %v", err)
	}
	cart, err := cartridge.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
	return cart
}

// blargg test ROMs write a status byte to $6000 (0x80 = running, 0 = done)
// with a signature at $6001-$6003, and an ASCII message from $6004.
func blarggStatus(c *NES) int {
	s := c.Peek(0x6000)
	if s == 0 {
		for i, b := range []byte{0xDE, 0xB0, 0x61} {
			if c.Peek(0x6001+uint16(i)) != b {
				return 0x80
			}
		}
	}
	return int(s)
}

func blarggMessage(c *NES) string {
	var msg []byte
	for a := uint16(0x6004); a < 0x8000; a++ {
		b := c.Peek(a)
		if b == 0 {
			break
		}
		msg = append(msg, b)
	}
	return string(msg)
}

// TestIntegrationBlarggCPU runs blargg's official instruction test through
// the fully wired console — the first end-to-end proof that CPU + PPU + APU +
// memory + board work together on the shared master clock.
func TestIntegrationBlarggCPU(t *testing.T) {
	cart := loadCart(t, "../../test/roms/instr_test-v5/official_only.nes")
	c, err := New(cart)
	if err != nil {
		t.Fatalf("console: %v", err)
	}

	for frame := 0; frame < 4000; frame++ {
		c.RunFrame()
		st := blarggStatus(c)
		if st == 0x81 {
			continue
		}
		if st != 0x80 {
			break
		}
	}
	msg := blarggMessage(c)
	t.Logf("blargg CPU status=%02X msg=%q", blarggStatus(c), msg)
	if blarggStatus(c) != 0 {
		t.Errorf("blargg CPU did not pass (status %02X): %s", blarggStatus(c), msg)
	}
}

const acStart = 0x08 // controller Start button bit

// TestIntegrationAccuracyCoin is the final gate: run the 141-test AccuracyCoin
// suite through the fully wired console and report the pass tally.
func TestIntegrationAccuracyCoin(t *testing.T) {
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

	if !started {
		t.Fatal("run-all never engaged")
	}
	if !finished {
		t.Fatal("AccuracyCoin did not finish (hang?)")
	}
	total := c.Peek(0x37)
	pass := c.Peek(0x38)
	t.Logf("AccuracyCoin (new core): %d/%d passed", pass, total)
	if total == 0 {
		t.Fatal("tally not populated")
	}
}
