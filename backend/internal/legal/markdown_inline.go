package legal

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	autoURLRe   = regexp.MustCompile(`^https?://[^\s<>"'()\[\]]+`)
	autoEmailRe = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`)
)

// urlTrailing is punctuation that ends a sentence rather than an auto-linked address.
const urlTrailing = ".,;:!?"

// safeSchemes are the only link targets produced; anything else (javascript:,
// data:, vbscript: ...) keeps its text and loses the link.
var safeSchemes = []string{"https://", "http://", "mailto:", "tel:"}

// inline renders one block's text: escapes everything, turning only the
// supported markers into tags.
func inline(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		wordStart := i == 0 || !isWordByte(s[i-1])
		if out, n := span(s[i:], wordStart); n > 0 {
			b.WriteString(out)
			i += n
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(html.EscapeString(s[i : i+size]))
		i += size
	}
	return b.String()
}

// span renders the marker at the start of s, or reports n = 0 when there is none.
func span(s string, wordStart bool) (string, int) {
	switch s[0] {
	case '\\':
		return backslash(s)
	case '`':
		return code(s)
	case '*':
		return emphasis(s)
	case '[':
		return link(s)
	}
	if wordStart {
		return autolink(s)
	}
	return "", 0
}

// backslash writes an escaped punctuation mark literally (\* is a star).
func backslash(s string) (string, int) {
	if len(s) > 1 && strings.ContainsRune("\\`*_[]{}()#+-.!<>|~", rune(s[1])) {
		return html.EscapeString(s[1:2]), 2
	}
	return "", 0
}

func code(s string) (string, int) {
	end := strings.IndexByte(s[1:], '`')
	if end <= 0 {
		return "", 0
	}
	return "<code>" + html.EscapeString(s[1:1+end]) + "</code>", end + 2
}

// emphasis is **bold** or *italic*; the text inside may not start or end with a space.
func emphasis(s string) (string, int) {
	marker, tag := "*", "em"
	if strings.HasPrefix(s, "**") {
		marker, tag = "**", "strong"
	}
	rest := s[len(marker):]
	end := strings.Index(rest, marker)
	if end <= 0 || rest[0] == ' ' || rest[end-1] == ' ' || (marker == "*" && rest[0] == '*') {
		return "", 0
	}
	return "<" + tag + ">" + inline(rest[:end]) + "</" + tag + ">", len(marker)*2 + end
}

// link is [text](target); an unsafe target keeps the text without the link.
func link(s string) (string, int) {
	rb := strings.IndexByte(s, ']')
	if rb < 1 || rb+1 >= len(s) || s[rb+1] != '(' {
		return "", 0
	}
	end := strings.IndexByte(s[rb+2:], ')')
	if end < 0 {
		return "", 0
	}
	text, target := s[1:rb], strings.TrimSpace(s[rb+2:rb+2+end])
	n := rb + 3 + end
	if !safeURL(target) {
		return inline(text), n
	}
	return anchor(target, inline(text)), n
}

// autolink links a bare web address or e-mail at the start of a word.
func autolink(s string) (string, int) {
	if m := autoURLRe.FindString(s); m != "" {
		m = strings.TrimRight(m, urlTrailing)
		return anchor(m, html.EscapeString(m)), len(m)
	}
	if m := autoEmailRe.FindString(s); m != "" {
		return anchor("mailto:"+m, html.EscapeString(m)), len(m)
	}
	return "", 0
}

func anchor(target, text string) string {
	return `<a href="` + html.EscapeString(target) + `" rel="noopener noreferrer">` + text + "</a>"
}

// safeURL accepts web, mail and phone links and same-site paths only.
func safeURL(u string) bool {
	if u == "" || strings.ContainsAny(u, " \t\r\n\"'<>`") {
		return false
	}
	if (strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//")) || strings.HasPrefix(u, "#") {
		return true
	}
	lower := strings.ToLower(u)
	for _, p := range safeSchemes {
		if strings.HasPrefix(lower, p) && len(u) > len(p) {
			return true
		}
	}
	return false
}

// isWordByte reports a byte that continues a word (so a link cannot start inside one).
func isWordByte(c byte) bool {
	return c == '_' || c == '.' || c == '-' || c == '@' || c == '/' || c >= 0x80 ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
