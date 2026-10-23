package pdf

import "unicode/utf8"

// replacementByte is written for any rune WinAnsi cannot represent.
const replacementByte = '?'

// asciiFirstPrintable and asciiLastPrintable bound the ASCII range WinAnsi
// shares unchanged with Unicode.
const (
	asciiFirstPrintable = 0x20
	asciiLastPrintable  = 0x7E
)

// latin1First and latin1Last bound the Latin-1 range (U+00A0..U+00FF) that
// WinAnsi also maps one-to-one.
const (
	latin1First = 0xA0
	latin1Last  = 0xFF
)

// cp1252High maps the Unicode runes WinAnsi places in 0x80..0x9F.
// Why a switch-free map: a lookup reads clearer than 27 cases, and it is only
// read (never iterated), so map ordering cannot affect output.
var cp1252High = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84,
	'…': 0x85, '†': 0x86, '‡': 0x87, 'ˆ': 0x88,
	'‰': 0x89, 'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C,
	'Ž': 0x8E, '‘': 0x91, '’': 0x92, '“': 0x93,
	'”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97,
	'˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B,
	'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F,
}

// toWinAnsi converts a UTF-8 string to WinAnsi (Windows-1252) bytes.
// Control characters, invalid UTF-8 and runes outside cp1252 become '?'.
func toWinAnsi(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		out = append(out, winAnsiByte(r))
	}
	return out
}

// winAnsiByte encodes one rune, falling back to replacementByte.
func winAnsiByte(r rune) byte {
	switch {
	case r == utf8.RuneError:
		return replacementByte
	case r >= asciiFirstPrintable && r <= asciiLastPrintable:
		return byte(r)
	case r >= latin1First && r <= latin1Last:
		return byte(r)
	}
	if b, ok := cp1252High[r]; ok {
		return b
	}
	return replacementByte
}

// escapeLiteral escapes encoded bytes for use inside a PDF literal string
// "( ... )": backslash, and both parentheses, are prefixed with a backslash.
func escapeLiteral(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == '\\' || c == '(' || c == ')' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return out
}
