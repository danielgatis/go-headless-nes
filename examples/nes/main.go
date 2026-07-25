// This example is a minimal desktop frontend for go-headless-nes,
// built on Ebitengine: one window, keyboard input, audio out. It shows
// the intended shape of a consumer: the core emulates and converts
// formats, the frontend just moves data to the screen and sound card.
//
//	go run . path/to/game.nes
//
// Controls: arrows = d-pad, Z = B, X = A, Enter = Start, right Shift =
// Select, R = reset, F5 = save state, F9 = load state, 1/2/3 = force
// NTSC/PAL/Dendy region, 0 = auto (re-detect from the header). A PAL game
// mis-flagged as NTSC runs ~20% fast until you press 2.
package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/danielgatis/go-headless-nes/nes"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const scale = 3

var keymap = map[ebiten.Key]byte{
	ebiten.KeyArrowUp:    nes.ButtonUp,
	ebiten.KeyArrowDown:  nes.ButtonDown,
	ebiten.KeyArrowLeft:  nes.ButtonLeft,
	ebiten.KeyArrowRight: nes.ButtonRight,
	ebiten.KeyZ:          nes.ButtonB,
	ebiten.KeyX:          nes.ButtonA,
	ebiten.KeyEnter:      nes.ButtonStart,
	ebiten.KeyShiftRight: nes.ButtonSelect,
}

// regionKeys maps a number key to the region it selects. 0 is Auto,
// which re-detects from the cartridge header.
var regionKeys = map[ebiten.Key]nes.Region{
	ebiten.Key0: nes.RegionAuto,
	ebiten.Key1: nes.RegionNTSC,
	ebiten.Key2: nes.RegionPAL,
	ebiten.Key3: nes.RegionDendy,
}

type game struct {
	console   *nes.Console
	stream    *nes.AudioStream
	frame     *ebiten.Image
	pixels    []byte
	saveState []byte
	title     string
	selected  nes.Region // what the user picked (RegionAuto until overridden)
}

func (g *game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.console.Reset()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		g.saveState = g.console.SaveState()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) && g.saveState != nil {
		if err := g.console.LoadState(g.saveState); err != nil {
			return err
		}
	}
	for key, r := range regionKeys {
		if inpututil.IsKeyJustPressed(key) {
			g.selected = r
			g.console.SetRegion(r)
			g.syncRegion()
		}
	}

	var buttons byte
	for key, bit := range keymap {
		if ebiten.IsKeyPressed(key) {
			buttons |= bit
		}
	}
	g.console.SetButtons(0, buttons)

	g.console.RunFrame()
	g.stream.Push(g.console.Audio())

	g.pixels = g.console.VideoRGBA(g.pixels)
	g.frame.WritePixels(g.pixels)
	return nil
}

// syncRegion drives Ebitengine's tick rate at the console's frame rate
// (60 on NTSC, 50 on PAL/Dendy) so one Update runs one emulated frame in
// real time. Without this the fixed 60 TPS would play PAL content ~20%
// fast. The title shows the user's pick and, when it is auto, what the
// cartridge resolved to.
func (g *game) syncRegion() {
	ebiten.SetTPS(int(math.Round(g.console.FrameRate())))
	tag := g.selected.String()
	if g.selected == nes.RegionAuto {
		tag = "auto: " + g.console.Region().String()
	}
	ebiten.SetWindowTitle(g.title + " [" + tag + "]")
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.DrawImage(g.frame, nil)
}

func (g *game) Layout(_, _ int) (int, int) {
	return nes.VideoWidth, nes.VideoHeight
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <rom.nes>\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	rom, err := os.ReadFile(os.Args[1]) //nolint:gosec // G304: reading the ROM path the user passed is this program's job
	if err != nil {
		log.Fatal(err)
	}
	console, err := nes.NewConsole(rom)
	if err != nil {
		log.Fatal(err)
	}

	g := &game{
		console: console,
		stream:  nes.NewAudioStream(0),
		frame:   ebiten.NewImage(nes.VideoWidth, nes.VideoHeight),
		title:   filepath.Base(os.Args[1]) + " - go-headless-nes player",
	}

	player, err := audio.NewContext(nes.SampleRate).NewPlayer(g.stream)
	if err != nil {
		log.Fatal(err)
	}
	player.SetBufferSize(50 * time.Millisecond)
	player.Play()

	ebiten.SetWindowSize(nes.VideoWidth*scale, nes.VideoHeight*scale)
	g.syncRegion() // seed tick rate and title from the auto-detected region
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
