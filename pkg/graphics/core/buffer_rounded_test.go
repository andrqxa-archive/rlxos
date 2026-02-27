package core

import "testing"

func TestDrawRoundedRectBlendsTranslucentBorderOverFill(t *testing.T) {
	buf := NewBuffer(20, 20)

	fill := NewColor(240, 240, 240, 180)
	border := NewColor(27, 42, 74, 48)
	r := Rect{X: 2, Y: 2, W: 16, H: 16}

	// Lay down a translucent rounded fill first (matches glass panel usage).
	buf.FillRoundedRect(r, 6, fill)
	insideBefore := buf.GetPixel(r.X+r.W-1, r.Y+6)
	if insideBefore != fill {
		t.Fatalf("expected setup fill at sampled edge pixel, got %+v want %+v", insideBefore, fill)
	}

	buf.DrawRoundedRect(r, 6, border)

	got := buf.GetPixel(r.X+r.W-1, r.Y+6)
	want := border.Blend(fill)
	if got != want {
		t.Fatalf("translucent rounded border did not blend with fill: got %+v want %+v", got, want)
	}
}
