package legal

import (
	"regexp"
	"strconv"
	"strings"
)

// Markdown renders the subset legal texts use to HTML: headings (#..####),
// paragraphs, bulleted and numbered lists, quotes, rules, and inline bold,
// italic, code and links. It is safe by construction: every character of the
// source is escaped and only these tags are produced, so raw HTML, scripts and
// event handlers in a text reach the reader as plain text, never as markup.
func Markdown(src string) string {
	r := &mdRenderer{}
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		r.line(line)
	}
	r.flushAll()
	return r.out.String()
}

// maxHeading is the deepest heading level produced (##### and deeper become h4).
const maxHeading = 4

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	ruleRe    = regexp.MustCompile(`^(?:-\s*){3,}$|^(?:\*\s*){3,}$|^(?:_\s*){3,}$`)
	bulletRe  = regexp.MustCompile(`^[-*+]\s+(.*)$`)
	numberRe  = regexp.MustCompile(`^\d{1,3}[.)]\s+(.*)$`)
)

// mdRenderer collects the open block (paragraph, list or quote) line by line.
type mdRenderer struct {
	out   strings.Builder
	para  []string // lines of the open paragraph
	list  string   // "ul", "ol" or "" when no list is open
	item  []string // lines of the open list item
	quote []string // lines of the open quote
}

// line feeds one source line.
func (r *mdRenderer) line(raw string) {
	t := strings.TrimSpace(raw)
	switch {
	case t == "":
		r.flushAll()
	case headingRe.MatchString(t):
		r.flushAll()
		m := headingRe.FindStringSubmatch(t)
		tag := "h" + strconv.Itoa(min(len(m[1]), maxHeading))
		r.out.WriteString("<" + tag + ">" + inline(m[2]) + "</" + tag + ">\n")
	case ruleRe.MatchString(t):
		r.flushAll()
		r.out.WriteString("<hr>\n")
	case strings.HasPrefix(t, ">"):
		r.flushPara()
		r.flushList()
		r.quote = append(r.quote, strings.TrimSpace(strings.TrimPrefix(t, ">")))
	default:
		r.text(t)
	}
}

// text handles list items, their continuation lines and paragraph lines.
func (r *mdRenderer) text(t string) {
	if m := bulletRe.FindStringSubmatch(t); m != nil {
		r.startItem("ul", m[1])
		return
	}
	if m := numberRe.FindStringSubmatch(t); m != nil {
		r.startItem("ol", m[1])
		return
	}
	if r.item != nil {
		r.item = append(r.item, t) // a continuation of the item, until a blank line
		return
	}
	r.flushList()
	r.flushQuote()
	r.para = append(r.para, t)
}

// startItem opens a list item, starting a new list when the kind changes.
func (r *mdRenderer) startItem(kind, first string) {
	r.flushPara()
	r.flushQuote()
	if r.list != kind {
		r.flushList()
		r.list = kind
		r.out.WriteString("<" + kind + ">\n")
	}
	r.flushItem()
	r.item = []string{first}
}

func (r *mdRenderer) flushAll() {
	r.flushPara()
	r.flushList()
	r.flushQuote()
}

func (r *mdRenderer) flushPara() {
	if len(r.para) > 0 {
		r.out.WriteString("<p>" + inline(strings.Join(r.para, " ")) + "</p>\n")
		r.para = nil
	}
}

func (r *mdRenderer) flushItem() {
	if r.item != nil {
		r.out.WriteString("<li>" + inline(strings.Join(r.item, " ")) + "</li>\n")
		r.item = nil
	}
}

func (r *mdRenderer) flushList() {
	r.flushItem()
	if r.list != "" {
		r.out.WriteString("</" + r.list + ">\n")
		r.list = ""
	}
}

func (r *mdRenderer) flushQuote() {
	if len(r.quote) > 0 {
		r.out.WriteString("<blockquote><p>" + inline(strings.Join(r.quote, " ")) + "</p></blockquote>\n")
		r.quote = nil
	}
}
