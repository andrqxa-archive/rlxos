package ui

import (
	"avyos.dev/pkg/graphics"
	gfxuirender "avyos.dev/pkg/graphics/uirender"
)

func roundedRectOutsideDistance(px, py float64, r graphics.Rect, radius int) float64 {
	return gfxuirender.RoundedRectOutsideDistance(px, py, r, radius)
}

func pointInRoundedRectUI(x, y int, r graphics.Rect, radius int) bool {
	return gfxuirender.PointInRoundedRect(x, y, r, radius)
}

func pointInRoundedRectAtUI(px, py float64, r graphics.Rect, radius int) bool {
	return gfxuirender.PointInRoundedRectAt(px, py, r, radius)
}

func pointInRoundedRectAtUIClamped(px, py float64, r graphics.Rect, radius int) bool {
	return gfxuirender.PointInRoundedRectAtClamped(px, py, r, radius)
}

func roundedRectCoverageUI(x, y int, r graphics.Rect, radius int) float64 {
	return gfxuirender.RoundedRectCoverage(x, y, r, radius)
}

func clampRoundedRadiusUI(r graphics.Rect, radius int) int {
	return gfxuirender.ClampRoundedRadius(r, radius)
}

func roundedRectSignedDistanceUI(px, py float64, r graphics.Rect, radius int) float64 {
	return gfxuirender.RoundedRectSignedDistance(px, py, r, radius)
}
