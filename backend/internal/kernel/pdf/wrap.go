package pdf

import "strings"

// Wrap breaks s into lines no wider than maxWidth points at size in font,
// greedily packing whole words. Explicit newlines start a new line (an empty
// paragraph yields an empty line); runs of other whitespace collapse to one
// space; a single word wider than maxWidth is split between characters.
func Wrap(size float64, font Font, s string, maxWidth float64) []string {
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		lines = append(lines, wrapParagraph(size, font, para, maxWidth)...)
	}
	return lines
}

// wrapParagraph wraps one newline-free paragraph.
func wrapParagraph(size float64, font Font, para string, maxWidth float64) []string {
	words := strings.Fields(para)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if TextWidth(size, font, candidate) <= maxWidth {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		pieces := splitLongWord(size, font, word, maxWidth)
		lines = append(lines, pieces[:len(pieces)-1]...)
		current = pieces[len(pieces)-1]
	}
	return append(lines, current)
}

// splitLongWord cuts word into pieces that each fit maxWidth; a word that
// already fits is returned whole. Every piece holds at least one rune so
// the loop always progresses, even when maxWidth is smaller than a glyph.
func splitLongWord(size float64, font Font, word string, maxWidth float64) []string {
	if TextWidth(size, font, word) <= maxWidth {
		return []string{word}
	}
	var pieces []string
	var piece []rune
	for _, r := range word {
		next := string(append(piece, r))
		if len(piece) > 0 && TextWidth(size, font, next) > maxWidth {
			pieces = append(pieces, string(piece))
			piece = piece[:0]
		}
		piece = append(piece, r)
	}
	return append(pieces, string(piece))
}
