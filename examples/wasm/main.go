// This example runs the go-headless-nes core in the browser, compiled to
// WebAssembly. The Go side is still just a consumer of the core: it holds
// a Console, steps a frame, and converts video and audio to formats the
// page can hand to a canvas and to Web Audio. Everything that is policy
// (windowing, the render loop, key mapping, audio scheduling) lives in
// index.html and is written in JavaScript.
//
// Build it with build.sh, then serve the directory over HTTP.
//
// The bridge is deliberately tiny. JavaScript calls four globals:
//
//	nesLoadROM(uint8Array)      -> "" on success, or an error string
//	nesRunFrame()               -> nothing; advances one frame
//	nesSetButtons(pad, mask)    -> nothing; latches controller state
//	nesVideoRGBA(uint8Array)    -> fills a VideoWidth*VideoHeight*4 buffer
//	nesAudio()                  -> Float32Array of samples for the frame
//	nesReset()                  -> nothing
//	nesSetRegion(code)          -> nothing; 0 auto, 1 NTSC, 2 PAL, 3 Dendy
//	nesRegion()                 -> string: the resolved region name
//	nesFrameRate()              -> number: emulated frames per second
//
// Passing the pixel buffer in from JS lets us reuse one allocation per
// frame instead of copying a fresh slice across the boundary each time.
package main

import (
	"syscall/js"

	"github.com/danielgatis/go-headless-nes/nes"
)

// console is the single emulated machine this page drives. WASM is
// single-threaded here, so a package-level value is safe.
var (
	console *nes.Console
	pixels  []byte
)

// loadROM builds a fresh Console from a JS Uint8Array of iNES bytes. It
// returns an empty string on success or the error text on failure, so JS
// can decide what to show the user.
func loadROM(_ js.Value, args []js.Value) any {
	rom := make([]byte, args[0].Get("length").Int())
	js.CopyBytesToGo(rom, args[0])

	c, err := nes.NewConsole(rom)
	if err != nil {
		return err.Error()
	}
	console = c
	pixels = make([]byte, nes.VideoWidth*nes.VideoHeight*4)
	return ""
}

// runFrame advances the emulation by exactly one frame. The page calls
// this once per requestAnimationFrame tick.
func runFrame(_ js.Value, _ []js.Value) any {
	if console != nil {
		console.RunFrame()
	}
	return nil
}

// setButtons latches the controller state for a pad. mask is an OR of the
// nes.Button* bits, which the page mirrors in JavaScript.
func setButtons(_ js.Value, args []js.Value) any {
	if console != nil {
		console.SetButtons(args[0].Int(), byte(args[1].Int()))
	}
	return nil
}

// videoRGBA converts the current framebuffer to RGBA and copies it into
// the JS Uint8Array the page passes in (typically backed by the canvas
// ImageData). Reusing that buffer avoids an allocation every frame.
func videoRGBA(_ js.Value, args []js.Value) any {
	if console == nil {
		return nil
	}
	pixels = console.VideoRGBA(pixels)
	js.CopyBytesToJS(args[0], pixels)
	return nil
}

// audio returns the frame's samples as a Float32Array. The page feeds
// these into a Web Audio buffer at nes.SampleRate.
func audio(_ js.Value, _ []js.Value) any {
	if console == nil {
		return js.Global().Get("Float32Array").New(0)
	}
	samples := console.Audio()
	out := js.Global().Get("Float32Array").New(len(samples))
	// A small typed-array copy: build a JS view over the Go floats via a
	// byte copy would need an ArrayBuffer, so set element-wise. Frames are
	// short (~735 samples), so this stays cheap.
	for i, s := range samples {
		out.SetIndex(i, s)
	}
	return out
}

func reset(_ js.Value, _ []js.Value) any {
	if console != nil {
		console.Reset()
	}
	return nil
}

// setRegion overrides the TV system: 0 auto (re-detect), 1 NTSC, 2 PAL,
// 3 Dendy. The switch is live; no reset.
func setRegion(_ js.Value, args []js.Value) any {
	if console != nil {
		console.SetRegion(nes.Region(args[0].Int()))
	}
	return nil
}

// region reports the resolved TV system name, so the page can label it.
func region(_ js.Value, _ []js.Value) any {
	if console == nil {
		return ""
	}
	return console.Region().String()
}

// frameRate reports how many emulated frames make one second, so the page
// can pace its loop at the region's real speed instead of a fixed 60.
func frameRate(_ js.Value, _ []js.Value) any {
	if console == nil {
		return 60.0988
	}
	return console.FrameRate()
}

func main() {
	// Publish the constants the page needs so index.html has no magic
	// numbers of its own.
	js.Global().Set("nesVideoWidth", nes.VideoWidth)
	js.Global().Set("nesVideoHeight", nes.VideoHeight)
	js.Global().Set("nesSampleRate", nes.SampleRate)

	js.Global().Set("nesLoadROM", js.FuncOf(loadROM))
	js.Global().Set("nesRunFrame", js.FuncOf(runFrame))
	js.Global().Set("nesSetButtons", js.FuncOf(setButtons))
	js.Global().Set("nesVideoRGBA", js.FuncOf(videoRGBA))
	js.Global().Set("nesAudio", js.FuncOf(audio))
	js.Global().Set("nesReset", js.FuncOf(reset))
	js.Global().Set("nesSetRegion", js.FuncOf(setRegion))
	js.Global().Set("nesRegion", js.FuncOf(region))
	js.Global().Set("nesFrameRate", js.FuncOf(frameRate))

	// Report the button bit layout so JS keeps a single source of truth.
	buttons := js.Global().Get("Object").New()
	buttons.Set("A", nes.ButtonA)
	buttons.Set("B", nes.ButtonB)
	buttons.Set("Select", nes.ButtonSelect)
	buttons.Set("Start", nes.ButtonStart)
	buttons.Set("Up", nes.ButtonUp)
	buttons.Set("Down", nes.ButtonDown)
	buttons.Set("Left", nes.ButtonLeft)
	buttons.Set("Right", nes.ButtonRight)
	js.Global().Set("nesButtons", buttons)

	// Signal readiness, then park forever so the exported funcs stay live.
	js.Global().Call("nesReady")
	select {}
}
