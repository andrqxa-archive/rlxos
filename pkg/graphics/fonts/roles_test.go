package font

import "testing"

func TestUIFontRoleNormalization(t *testing.T) {
	if got := normalizeUIFontRole("parag"); got != UIFontParagraph {
		t.Fatalf("normalizeUIFontRole(parag) = %q", got)
	}
	if got := normalizeUIFontRole("h1"); got != UIFontTitle {
		t.Fatalf("normalizeUIFontRole(h1) = %q", got)
	}
	if got := normalizeUIFontRole("subtitle"); got != UIFontSubheading {
		t.Fatalf("normalizeUIFontRole(subtitle) = %q", got)
	}
}

func TestUIFontFallbackToParagraph(t *testing.T) {
	origParagraph := UIFont(UIFontParagraph)
	t.Cleanup(func() {
		SetUIFont(UIFontParagraph, origParagraph)
	})

	f := &Font{Width: 11, Height: 22}
	SetUIFont(UIFontParagraph, f)
	if got := UIFont("unknown-role"); got != f {
		t.Fatalf("UIFont(unknown-role) did not fallback to paragraph")
	}
}
