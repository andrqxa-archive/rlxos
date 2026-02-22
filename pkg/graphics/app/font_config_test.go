package app

import (
	"strings"
	"testing"

	"avyos.dev/pkg/graphics"
	"avyos.dev/pkg/ini"
	xfont "golang.org/x/image/font"
)

func TestPickConfiguredFontUISection(t *testing.T) {
	cfg, err := ini.Parse(strings.NewReader(`
[ui]
name = inter
size = 16
dpi = 96
hinting = vertical
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	spec, ok := pickConfiguredFont(cfg)
	if !ok {
		t.Fatalf("pickConfiguredFont() expected configured font")
	}
	if spec.Name != "inter" {
		t.Fatalf("name = %q, want inter", spec.Name)
	}
	if got := spec.RoleSizes[graphics.UIFontParagraph]; got != 16 {
		t.Fatalf("paragraph size = %v, want 16", got)
	}
	if got := spec.RoleSizes[graphics.UIFontSubheading]; got != 18 {
		t.Fatalf("subheading size = %v, want 18", got)
	}
	if got := spec.RoleSizes[graphics.UIFontHeading]; got != 20 {
		t.Fatalf("heading size = %v, want 20", got)
	}
	if got := spec.RoleSizes[graphics.UIFontTitle]; got != 24 {
		t.Fatalf("title size = %v, want 24", got)
	}
	if spec.BaseOpts.DPI != 96 {
		t.Fatalf("dpi = %v, want 96", spec.BaseOpts.DPI)
	}
	if spec.BaseOpts.Hinting != xfont.HintingVertical {
		t.Fatalf("hinting = %v, want %v", spec.BaseOpts.Hinting, xfont.HintingVertical)
	}
	if !spec.BaseOpts.HintingSet {
		t.Fatalf("HintingSet = false, want true")
	}
}

func TestPickConfiguredFontSkipsDisabledSection(t *testing.T) {
	cfg, err := ini.Parse(strings.NewReader(`
[ui]
enabled = false
name = inter

[default]
name = inter
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	spec, ok := pickConfiguredFont(cfg)
	if !ok {
		t.Fatalf("pickConfiguredFont() expected configured font")
	}
	if spec.Name != "inter" {
		t.Fatalf("name = %q, want inter", spec.Name)
	}
}

func TestRoleSizesFromExplicitKeys(t *testing.T) {
	cfg, err := ini.Parse(strings.NewReader(`
[ui]
name = inter
size = 14
paragraph_size = 15
subheading_size = 18
heading_size = 22
title_size = 30
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	spec, ok := pickConfiguredFont(cfg)
	if !ok {
		t.Fatalf("pickConfiguredFont() expected configured font")
	}
	if got := spec.RoleSizes[graphics.UIFontParagraph]; got != 15 {
		t.Fatalf("paragraph size = %v, want 15", got)
	}
	if got := spec.RoleSizes[graphics.UIFontSubheading]; got != 18 {
		t.Fatalf("subheading size = %v, want 18", got)
	}
	if got := spec.RoleSizes[graphics.UIFontHeading]; got != 22 {
		t.Fatalf("heading size = %v, want 22", got)
	}
	if got := spec.RoleSizes[graphics.UIFontTitle]; got != 30 {
		t.Fatalf("title size = %v, want 30", got)
	}
}
