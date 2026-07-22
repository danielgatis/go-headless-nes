package mapper

import "github.com/danielgatis/go-headless-nes/internal/cartridge"

// cart builds a synthetic cartridge whose PRG bytes encode their 16 KiB
// bank number and CHR bytes their 8 KiB bank number, so tests can see
// which bank a read went to.
func cart(mapperID uint16, prg16, chr8 int) *cartridge.Cartridge {
	c := &cartridge.Cartridge{
		MapperID:  mapperID,
		Mirroring: cartridge.Vertical,
		PRG:       make([]byte, prg16*0x4000),
	}
	for i := range c.PRG {
		c.PRG[i] = byte(i / 0x4000)
	}
	if chr8 > 0 {
		c.CHR = make([]byte, chr8*0x2000)
		for i := range c.CHR {
			c.CHR[i] = byte(i / 0x2000)
		}
	}
	return c
}

// chrCart builds a cartridge whose CHR bytes encode their 1 KiB bank.
func chrCart(mapperID uint16, prg16, chr8 int) *cartridge.Cartridge {
	c := cart(mapperID, prg16, chr8)
	for i := range c.CHR {
		c.CHR[i] = byte(i / 0x400)
	}
	return c
}

// prg8Cart builds a cartridge whose PRG bytes encode their 8 KiB bank.
func prg8Cart(mapperID uint16, prg16 int) *cartridge.Cartridge {
	c := cart(mapperID, prg16, 1)
	for i := range c.PRG {
		c.PRG[i] = byte(i / 0x2000)
	}
	return c
}
