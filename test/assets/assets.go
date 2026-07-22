// Package assets embeds the ROM images the integration suite runs
// against. The images come from two git submodules pinned under this
// directory: christopherpow/nes-test-roms (roms/) and 100thCoin's
// AccuracyCoin (roms-accuracycoin/). Keeping the same on-disk layout as
// gabe565/gones lets the two emulators run identical ROMs from identical
// assets, so expected results stay directly comparable.
package assets

import "embed"

// ROMs is the christopherpow/nes-test-roms tree, addressed by paths like
// "roms/other/nestest.nes".
//
//go:embed roms
var ROMs embed.FS

// AccuracyCoin is the single AccuracyCoin (100thCoin) test ROM.
//
//go:embed roms-accuracycoin/AccuracyCoin.nes
var AccuracyCoin []byte
