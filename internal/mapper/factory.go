// Package mapper emulates cartridge boards: the address decoding, bank
// switching, on-board memory and expansion hardware between the console
// buses and the ROM. Each mapper models the actual cartridge hardware,
// not per-game behavior.
package mapper

import (
	"github.com/danielgatis/go-headless-nes/internal/errs"

	"github.com/danielgatis/go-headless-nes/internal/cartridge"
)

// Mapper is the cartridge board seen from the two console buses.
//
// PRG addresses are CPU addresses ($4020-$FFFF reach the cartridge
// port, though most boards only decode $6000 and up); CHR addresses
// are PPU pattern-table addresses ($0000-$1FFF). Since every CPU and
// PPU access flows through these methods, boards observe the buses
// exactly as hardware does, bank registers, CHR latches (MMC2) and
// address-line IRQs all hang off them.
type Mapper interface {
	ReadPRG(addr uint16) byte
	WritePRG(addr uint16, v byte)
	ReadCHR(addr uint16) byte
	WriteCHR(addr uint16, v byte)

	// Mirroring reports the current nametable arrangement. Fixed by
	// solder pads on simple boards, register-controlled on others.
	Mirroring() cartridge.Mirroring

	// Tick advances board hardware by one CPU cycle (VRC-style cycle
	// IRQ counters). Most boards do nothing.
	Tick()

	// Scanline is the PPU's end-of-visible-line notification (dot 260
	// with rendering enabled), which is when MMC3-style counters see
	// their A12 rise. Boards without scanline counters ignore it.
	Scanline()

	// IRQ reports the level of the cartridge's IRQ line.
	IRQ() bool

	// FiltersConsecutiveWrites reports whether the board ignores the
	// second of two writes on back-to-back CPU cycles, as the MMC1's
	// serial port does. Read-modify-write instructions generate
	// exactly that pattern.
	FiltersConsecutiveWrites() bool

	// SetOpenBus hands the board the current data-bus value so an
	// undecoded cartridge-port read returns the real floating value.
	// The bus calls this before each ReadPRG.
	SetOpenBus(v byte)

	// Save and Restore copy the board's mutable state to and from a
	// fixed-size State, so rewind snapshots need no serialization,
	// reflection, or allocation. ROM is immutable and never copied.
	Save(*State)
	Restore(*State)
}

// State is snapshot storage for any supported mapper. It is sized for
// the largest board we emulate; simple boards use a subset. Each
// mapper packs its registers into Regs explicitly.
type State struct {
	Regs   [96]byte
	PRGRAM [32768]byte
	CHRRAM [8192]byte
}

// New builds the mapper for a parsed cartridge.
func New(c *cartridge.Cartridge) (Mapper, error) {
	switch c.MapperID {
	case 0:
		return NewNROM(c), nil
	case 1:
		return NewMMC1(c), nil
	case 2:
		return NewUxROM(c), nil
	case 3:
		return NewCNROM(c), nil
	case 4:
		return NewMMC3(c), nil
	case 5:
		return NewMMC5(c), nil
	case 7:
		return NewAxROM(c), nil
	case 9:
		return NewMMC2(c), nil
	case 10:
		return NewMMC4(c), nil
	case 11:
		return NewColorDreams(c), nil
	case 13:
		return NewCpROM(c), nil
	case 15:
		return NewMapper15(c), nil
	case 16, 153, 157, 159:
		return NewBandaiFCG(c), nil
	case 18:
		return NewJalecoSS88006(c), nil
	case 19, 210:
		return NewNamco163(c), nil
	case 21, 22, 23, 25, 27, 183:
		return NewVRC24(c), nil
	case 24, 26:
		return NewVRC6(c), nil
	case 30:
		return NewUNROM512(c), nil
	case 32:
		return NewIremG101(c), nil
	case 33:
		return NewTaitoTC0190(c), nil
	case 34:
		return NewMapper34(c), nil
	case 37:
		return NewMMC3Multi37(c), nil
	case 38:
		return NewPCI556(c), nil
	case 44:
		return NewMMC3Multi44(c), nil
	case 45:
		return NewMMC3Multi45(c), nil
	case 47:
		return NewMMC3Multi47(c), nil
	case 48:
		return NewTaitoTC0690(c), nil
	case 49:
		return NewMMC3Multi49(c), nil
	case 52:
		return NewMMC3Multi52(c), nil
	case 64:
		return NewRambo1(c), nil
	case 65:
		return NewIremH3001(c), nil
	case 66:
		return NewGxROM(c), nil
	case 67:
		return NewSunsoft3(c), nil
	case 69:
		return NewSunsoftFME7(c), nil
	case 70, 152:
		return NewBandai74161(c), nil
	case 71:
		return NewCamerica(c), nil
	case 72, 92:
		return NewJalecoJF17(c), nil
	case 73:
		return NewVRC3(c), nil
	case 75, 151:
		// 151 is the VS-System iNES number for the same VRC1 board.
		return NewVRC1(c), nil
	case 76:
		return NewDxROM76(c), nil
	case 77:
		return NewIremLROG017(c), nil
	case 78:
		return NewJalecoJF16(c), nil
	case 79, 113, 146:
		// 113 is the multicart revision of the NINA-03/06 board; 146 is
		// the same board as 79 under a different maker's ID.
		return NewNINA03(c), nil
	case 80, 207:
		return NewTaitoX1005(c), nil
	case 82, 552:
		return NewTaitoX1017(c), nil
	case 85:
		return NewVRC7(c), nil
	case 86:
		return NewJalecoJF13(c), nil
	case 87:
		return NewJaleco87(c), nil
	case 88:
		return NewDxROM88(c), nil
	case 89:
		return NewSunsoft89(c), nil
	case 93:
		return NewSunsoft93(c), nil
	case 94:
		return NewUN1ROM(c), nil
	case 95:
		return NewDxROM95(c), nil
	case 97:
		return NewIremTamS1(c), nil
	case 101:
		return NewJaleco101(c), nil
	case 105:
		return NewMMC1Event(c), nil
	case 107:
		return NewMapper107(c), nil
	case 112:
		return NewMapper112(c), nil
	case 118:
		return NewTxSROM(c), nil
	case 119:
		return NewTQROM(c), nil
	case 140:
		return NewJaleco140(c), nil
	case 154:
		return NewDxROM154(c), nil
	case 155:
		// MMC1A: the PRG-RAM disable bit is not connected.
		m := NewMMC1(c)
		m.ramAlwaysOn = true
		return m, nil
	case 158:
		return NewRambo158(c), nil
	case 180:
		return NewUNROM180(c), nil
	case 184:
		return NewSunsoft184(c), nil
	case 185:
		return NewCNROMProtect(c), nil
	case 206:
		return NewDxROM(c), nil
	case 232:
		return NewQuattro(c), nil
	case 268:
		return NewMMC3Coolboy(c), nil
	case 74, 191, 192, 194, 195:
		return NewMMC3ChrRAM(c), nil
	case 115:
		return NewMMC3115(c), nil
	case 165:
		return NewMMC3165(c), nil
	case 12:
		return NewMMC312(c), nil
	case 182:
		return NewMMC3182(c), nil
	case 197:
		return NewMMC3197(c), nil
	case 205:
		return NewMMC3205(c), nil
	case 250:
		return NewMMC3250(c), nil
	case 196:
		return NewMMC3196(c), nil
	case 245:
		return NewMMC3245(c), nil
	case 254:
		return NewMMC3254(c), nil
	case 238:
		return NewMMC3238(c), nil
	case 114:
		return NewMMC3114(c), nil
	case 123:
		return NewMMC3123(c), nil
	case 134:
		return NewMMC3134(c), nil
	case 249:
		return NewMMC3249(c), nil
	case 187:
		return NewMMC3187(c), nil
	case 189:
		return NewMMC3189(c), nil
	case 224:
		return NewMMC3224(c), nil
	case 28:
		return NewAction53(c), nil
	case 68:
		return NewSunsoft4(c), nil
	case 137, 138, 139, 141:
		return NewSachen8259(c), nil
	case 54, 201:
		return NewNovelDiamond(c), nil
	case 57:
		return NewMapper57(c), nil
	case 58:
		return NewMapper58(c), nil
	case 61:
		return NewMapper61(c), nil
	case 62:
		return NewMapper62(c), nil
	case 200:
		return NewMapper200(c), nil
	case 202:
		return NewMapper202(c), nil
	case 203:
		return NewMapper203(c), nil
	case 213:
		return NewMapper213(c), nil
	case 240:
		return NewMapper240(c), nil
	case 242:
		return NewMapper242(c), nil
	case 56, 142:
		return NewKaiser202(c), nil
	case 171:
		return NewKaiser7058(c), nil
	case 175:
		return NewKaiser7022(c), nil
	case 162:
		return NewWaixing162(c), nil
	case 164:
		return NewWaixing164(c), nil
	case 178:
		return NewWaixing178(c), nil
	case 252:
		return NewWaixing252(c), nil
	case 36:
		return NewTxc22000(c), nil
	case 132:
		return NewTxc22211A(c), nil
	case 172:
		return NewTxc22211B(c), nil
	case 173:
		return NewTxc22211C(c), nil
	case 6, 8, 17:
		return NewFrontFareast(c), nil
	case 166, 167:
		return NewSubor166(c), nil
	case 177:
		return NewHenggedianzi177(c), nil
	case 179:
		return NewHenggedianzi179(c), nil
	case 46:
		return NewColorDreams46(c), nil
	case 50:
		return NewMapper50(c), nil
	case 170:
		return NewMapper170(c), nil
	case 174:
		return NewMapper174(c), nil
	case 204:
		return NewMapper204(c), nil
	case 216:
		return NewMapper216(c), nil
	case 144:
		return NewColorDreams144(c), nil
	case 225:
		return NewMapper225(c), nil
	case 226:
		return NewMapper226(c), nil
	case 227:
		return NewMapper227(c), nil
	case 229:
		return NewMapper229(c), nil
	case 231:
		return NewMapper231(c), nil
	case 234:
		return NewMapper234(c), nil
	case 241:
		return NewMapper241(c), nil
	case 244:
		return NewMapper244(c), nil
	case 190:
		return NewMagicKidGooGoo(c), nil
	case 193:
		return NewNtdecTc112(c), nil
	case 221:
		return NewMapper221(c), nil
	case 228:
		return NewActionEnterprises(c), nil
	case 39:
		return NewMapper39(c), nil
	case 40:
		return NewMapper40(c), nil
	case 42:
		return NewMapper42(c), nil
	case 59:
		return NewUnlD1038(c), nil
	case 106:
		return NewMapper106(c), nil
	case 41:
		return NewCaltron41(c), nil
	case 51:
		return NewBmc51(c), nil
	case 108:
		return NewBb(c), nil
	case 255:
		return NewBmc255(c), nil
	case 91:
		return NewMapper91(c), nil
	case 235:
		return NewBmc235(c), nil
	case 133:
		return NewSachen133(c), nil
	case 136:
		return NewSachen136(c), nil
	case 143:
		return NewSachen143(c), nil
	case 145:
		return NewSachen145(c), nil
	case 147:
		return NewSachen147(c), nil
	case 148:
		return NewSachen148(c), nil
	case 149:
		return NewSachen149(c), nil
	case 150, 243:
		return NewSachen74LS374N(c), nil
	case 156:
		return NewDaouInfosys(c), nil
	case 265:
		return NewT262(c), nil
	case 283:
		return NewGs2004(c), nil
	case 285:
		return NewA65AS(c), nil
	case 288:
		return NewGkcx1(c), nil
	case 304:
		return NewSmb2j(c), nil
	case 329:
		return NewEdu2000(c), nil
	case 529:
		return NewT230(c), nil
	case 259:
		return NewMMC3BmcF15(c), nil
	case 263:
		return NewMMC3Kof97(c), nil
	case 308, 309:
		return NewLh51(c), nil
	case 327, 328:
		return NewRt01(c), nil
	case 271, 274:
		return NewBmc80013B(c), nil
	case 331:
		return NewBmc12in1(c), nil
	case 35:
		return NewMapper35(c), nil
	case 117:
		return NewMapper117(c), nil
	case 222:
		return NewMapper222(c), nil
	case 60:
		return NewMapper60(c), nil
	case 230:
		return NewMapper230(c), nil
	case 233:
		return NewMapper233(c), nil
	case 313:
		return NewResetTxrom(c), nil
	case 199:
		return NewMMC3199(c), nil
	case 262:
		return NewMMC3StreetHeroes(c), nil
	case 325:
		return NewMMC3MaliSB(c), nil
	case 305:
		return NewKaiser7031(c), nil
	case 346:
		return NewKaiser7012(c), nil
	case 306:
		return NewKaiser7016(c), nil
	case 120:
		return NewMapper120(c), nil
	case 212:
		return NewMapper212(c), nil
	case 214:
		return NewMapper214(c), nil
	case 246:
		return NewMapper246(c), nil
	case 261:
		return NewBmc810544CA1(c), nil
	case 290:
		return NewBmcNtd03(c), nil
	case 299:
		return NewBmc11160(c), nil
	case 300:
		return NewBmc190in1(c), nil
	case 336:
		return NewBmcK3046(c), nil
	case 349:
		return NewBmcG146(c), nil
	case 104:
		return NewGoldenFive(c), nil
	case 125:
		return NewLh32(c), nil
	case 332:
		return NewSuper40in1Ws(c), nil
	case 348:
		return NewBmc830118C(c), nil
	case 521:
		return NewDreamTech01(c), nil
	case 53:
		return NewSupervision(c), nil
	case 258:
		return NewUnl158B(c), nil
	case 286:
		return NewBs5(c), nil
	case 289:
		return NewBmc60311C(c), nil
	case 312:
		return NewKaiser7013B(c), nil
	case 319:
		return NewHp898f(c), nil
	case 320:
		return NewBmc830425C4391T(c), nil
	case 366:
		return NewBmcGn45(c), nil
	case 29:
		return NewSealieComputing(c), nil
	case 301:
		return NewBmc8157(c), nil
	case 314:
		return NewBmc64in1NoRepeat(c), nil
	case 324:
		return NewFaridUnrom(c), nil
	case 43:
		return NewMapper43(c), nil
	case 287:
		return NewMMC3Bmc411120C(c), nil
	case 519:
		return NewEh8813A(c), nil
	case 522:
		return NewLh10(c), nil
	case 298:
		return NewTf1201(c), nil
	case 302:
		return NewKaiser7057(c), nil
	case 303:
		return NewKaiser7017(c), nil
	case 530:
		return NewAx5705(c), nil
	case 31:
		return NewNsfCart31(c), nil
	case 264:
		return NewYoko(c), nil
	case 266:
		return NewCityFighter(c), nil
	case 487:
		return NewMapper487(c), nil
	case 208:
		return NewMMC3208(c), nil
	case 63:
		return NewBmc63(c), nil
	case 236:
		return NewBmc70in1(c), nil
	case 96:
		return NewOekaKids(c), nil
	case 518:
		return NewDance2000(c), nil
	case 103:
		return NewMapper103(c), nil
	case 198:
		return NewMMC3198(c), nil
	case 307:
		return NewKaiser7037(c), nil
	case 219:
		return NewMMC3219(c), nil
	case 333:
		return NewBmc8in1(c), nil
	case 217:
		return NewMMC3217(c), nil
	case 215:
		return NewMMC3215(c), nil
	case 126:
		return NewMMC3126(c), nil
	case 260:
		return NewBmcHpxx(c), nil
	case 253:
		return NewMapper253(c), nil
	case 83:
		return NewMapper83(c), nil
	case 121:
		return NewMMC3121(c), nil
	case 14:
		return NewMMC314(c), nil
	case 323:
		return NewFaridSlrom(c), nil
	case 116:
		return NewMapper116(c), nil
	case 218:
		return NewMagicFloor218(c), nil
	case 90, 209, 211:
		return NewJYCompany(c), nil
	case 111:
		return NewGTROM(c), nil
	case 163:
		return NewNanjing(c), nil
	case 168:
		return NewRacermate(c), nil
	case 176:
		return NewFk23C(c), nil
	case 188:
		return NewBandaiKaraoke(c), nil
	case 284:
		return NewDripGame(c), nil
	case 292:
		return NewDragonFighter(c), nil
	case 513:
		return NewSachen9602(c), nil

	// Boards the reference emulator supports that this one recognizes but does not
	// implement: each is console-level hardware, not a cartridge board (see
	// docs/MAPPERS.md). They get a distinct error so callers can tell
	// "known but unimplemented" apart from an unknown mapper number.
	case 99, // VS System: dual-system, coin/DIP inputs, security PPU
		682: // Rainbow: flash + FPGA expansion audio/video + WiFi hardware
		return nil, errs.Errorf("mapper %d recognized but not supported yet", c.MapperID)
	}
	return nil, errs.Errorf("unsupported mapper %d", c.MapperID)
}
