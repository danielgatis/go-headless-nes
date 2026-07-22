// This example is a minimal desktop frontend for go-headless-nes,
// built on Ebitengine: one window, keyboard input, audio out. It shows
// the intended shape of a consumer: the core emulates and converts
// formats, the frontend just moves data to the screen and sound card.
//
//	go run . path/to/game.nes
//
// Controls: arrows = d-pad, Z = B, X = A, Enter = Start, right Shift =
// Select, R = reset, F5 = save state, F9 = load state.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	nes "github.com/danielgatis/go-headless-nes"
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

type game struct {
	console   *nes.Console
	stream    *nes.AudioStream
	frame     *ebiten.Image
	pixels    []byte
	saveState []byte
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

	rom, err := os.ReadFile(os.Args[1])
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
	}

	player, err := audio.NewContext(nes.SampleRate).NewPlayer(g.stream)
	if err != nil {
		log.Fatal(err)
	}
	player.SetBufferSize(50 * time.Millisecond)
	player.Play()

	ebiten.SetWindowSize(nes.VideoWidth*scale, nes.VideoHeight*scale)
	ebiten.SetWindowTitle(filepath.Base(os.Args[1]) + " - go-headless-nes player")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
