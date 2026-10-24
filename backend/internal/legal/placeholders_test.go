package legal

import (
	"reflect"
	"strings"
	"testing"
)

func TestFillAndMissing(t *testing.T) {
	out, missing := Fill("{{ brand.name }} at {{registered_address}} ({{dpo_email}})", map[string]string{"brand.name": "Ride Co", "dpo_email": "dpo@ride.example"})
	if !strings.HasPrefix(out, "Ride Co at ") || !strings.Contains(out, "dpo@ride.example") {
		t.Fatalf("fill: %q", out)
	}
	if !reflect.DeepEqual(missing, []string{"registered_address"}) {
		t.Fatalf("missing: %v", missing)
	}
	if html := Markdown(out); !strings.Contains(html, "[registered_address]") {
		t.Fatalf("a gap shows its key: %s", html)
	}
}

// TestFillValueCannotAddMarkup: a value is text, even when it looks like Markdown or HTML.
func TestFillValueCannotAddMarkup(t *testing.T) {
	out, _ := Fill("Address: {{registered_address}}", map[string]string{"registered_address": "[x](javascript:alert(1)) **b** <script>"})
	html := Markdown(out)
	if strings.Contains(html, "<a ") || strings.Contains(html, "<strong>") || strings.Contains(html, "<script") {
		t.Fatalf("value added markup: %s", html)
	}
}

func TestUnknownPlaceholders(t *testing.T) {
	got := UnknownPlaceholders("{{brand.name}} {{dpo_mail}} {{ nope }} {{dpo_mail}}")
	if !reflect.DeepEqual(got, []string{"dpo_mail", "nope"}) {
		t.Fatalf("unknown: %v", got)
	}
}

func TestKindsAndParse(t *testing.T) {
	if k, ok := ParseKind("privacy-notice"); !ok || k != KindPrivacyNotice {
		t.Fatal("slug parses")
	}
	if Offered(AppRider, KindDriverAgreement) || !Offered(AppDriver, KindDriverAgreement) {
		t.Fatal("the driver agreement is for drivers only")
	}
	if got := PagePath(AppDriver, KindTerms, "sw"); got != "/legal/driver/terms?lang=sw" {
		t.Fatalf("path %s", got)
	}
}
