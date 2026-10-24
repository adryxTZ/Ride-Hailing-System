package legal

import (
	"strings"
	"testing"
)

func TestMarkdownBlocks(t *testing.T) {
	got := Markdown("## Who we are\n\nWe run **the service**.\nSecond line.\n\n- one\n- two\n  continued\n\n1. first\n2. second\n\n> Note\n\n---")
	for _, want := range []string{
		"<h2>Who we are</h2>",
		"<p>We run <strong>the service</strong>. Second line.</p>",
		"<ul>\n<li>one</li>\n<li>two continued</li>\n</ul>",
		"<ol>\n<li>first</li>\n<li>second</li>\n</ol>",
		"<blockquote><p>Note</p></blockquote>",
		"<hr>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func TestMarkdownLinks(t *testing.T) {
	got := Markdown("Write to help@example.com or see https://example.com/a. [Terms](/legal/rider/terms?lang=en) *now*")
	for _, want := range []string{
		`<a href="mailto:help@example.com" rel="noopener noreferrer">help@example.com</a>`,
		`<a href="https://example.com/a" rel="noopener noreferrer">https://example.com/a</a>.`,
		`<a href="/legal/rider/terms?lang=en" rel="noopener noreferrer">Terms</a>`,
		"<em>now</em>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

// TestMarkdownIsSafe: raw HTML, scripts, handlers and unsafe link schemes never become markup.
func TestMarkdownIsSafe(t *testing.T) {
	src := "<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>\n\n[click](javascript:alert(1)) [x](data:text/html,hi) [y](//evil.example)\n\n## <b>bold</b>\n\n`<i>`"
	got := Markdown(src)
	for _, bad := range []string{"<script", "<img", "<b>", "<i>", `href="javascript`, `href="data`, `href="//`} {
		if strings.Contains(got, bad) {
			t.Errorf("unsafe %q survived:\n%s", bad, got)
		}
	}
	if !strings.Contains(got, "&lt;script&gt;alert(1)&lt;/script&gt;") || !strings.Contains(got, "click") {
		t.Fatalf("text must stay readable:\n%s", got)
	}
}
