package gsm7bit

import "unicode"

func init() {
	for index, r := range reverseLookup {
		forwardLookup[r] = byte(index)
	}
	for r, b := range forwardEscapes {
		reverseEscapes[b] = r
	}
}

const esc, cr byte = 0x1B, 0x0D

var forwardLookup = map[rune]byte{}

var reverseLookup = [256]rune{
	0x40, 0xA3, 0x24, 0xA5, 0xE8, 0xE9, 0xF9, 0xEC, 0xF2, 0xC7, 0x0A, 0xD8, 0xF8, 0x0D, 0xC5, 0xE5,
	0x0394, 0x005F, 0x03A6, 0x0393, 0x039B, 0x03A9, 0x03A0, 0x03A8, 0x03A3, 0x0398, 0x039E, 0x00A0,
	0xC6, 0xE6, 0xDF, 0xC9, 0x20, 0x21, 0x22, 0x23, 0xA4, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x2B,
	0x2C, 0x2D, 0x2E, 0x2F, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3A, 0x3B,
	0x3C, 0x3D, 0x3E, 0x3F, 0xA1, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x4B,
	0x4C, 0x4D, 0x4E, 0x4F, 0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5A, 0xC4,
	0xD6, 0xD1, 0xDC, 0xA7, 0xBF, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x6B,
	0x6C, 0x6D, 0x6E, 0x6F, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0xE4,
	0xF6, 0xF1, 0xFC, 0xE0,
}

var forwardEscapes = map[rune]byte{
	0x0C: 0x0A, 0x5B: 0x3C, 0x5C: 0x2F, 0x5D: 0x3E, 0x5E: 0x14, 0x7B: 0x28, 0x7C: 0x40, 0x7D: 0x29, 0x7E: 0x3D,
	0x20AC: 0x65,
}

var reverseEscapes = map[byte]rune{}

var DefaultAlphabet = &unicode.RangeTable{R16: []unicode.Range16{
	{0x000A, 0x000A, 1}, {0x000C, 0x000D, 1}, {0x0020, 0x005F, 1}, {0x0061, 0x007E, 1}, {0x00A0, 0x00A1, 1},
	{0x00A3, 0x00A5, 1}, {0x00A7, 0x00A7, 1}, {0x00BF, 0x00BF, 1}, {0x00C4, 0x00C6, 1}, {0x00C9, 0x00C9, 1},
	{0x00D1, 0x00D1, 1}, {0x00D6, 0x00D6, 1}, {0x00D8, 0x00D8, 1}, {0x00DC, 0x00DC, 1}, {0x00DF, 0x00E0, 1},
	{0x00E4, 0x00E9, 1}, {0x00EC, 0x00EC, 1}, {0x00F1, 0x00F2, 1}, {0x00F6, 0x00F6, 1}, {0x00F8, 0x00F9, 1},
	{0x00FC, 0x00FC, 1}, {0x0393, 0x0394, 1}, {0x0398, 0x0398, 1}, {0x039B, 0x039B, 1}, {0x039E, 0x039E, 1},
	{0x03A0, 0x03A0, 1}, {0x03A3, 0x03A3, 1}, {0x03A6, 0x03A6, 1}, {0x03A8, 0x03A9, 1}, {0x20AC, 0x20AC, 1},
}}
