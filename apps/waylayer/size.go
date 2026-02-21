/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

const (
	defaultToplevelWidth  = 800
	defaultToplevelHeight = 600
	defaultPopupWidth     = 256
	defaultPopupHeight    = 192

	// Keep dimensions comfortably below native backend hard limits.
	// Some toolkits include extra client-side decoration extents on top of
	// content size and can exceed uint16 bounds if we clamp right at 65535.
	maxSurfaceDimension = 16384

	// Hard cap to avoid pathological allocations (64M pixels = 256MiB at 32bpp).
	maxSurfacePixels = 64 * 1024 * 1024
)

func clampSurfaceSize(width, height, fallbackW, fallbackH int) (int, int, bool) {
	origW, origH := width, height

	width = clampSurfaceDimension(width, fallbackW)
	height = clampSurfaceDimension(height, fallbackH)

	// Keep worst-case buffer sizes bounded.
	if int64(width)*int64(height) > maxSurfacePixels {
		maxH := int(maxSurfacePixels / int64(width))
		if maxH < 1 {
			maxH = 1
		}
		height = maxH
	}

	return width, height, width != origW || height != origH
}

func clampSurfaceDimension(value, fallback int) int {
	if fallback < 1 {
		fallback = 1
	}
	if fallback > maxSurfaceDimension {
		fallback = maxSurfaceDimension
	}

	if value <= 0 {
		value = fallback
	}
	if value > maxSurfaceDimension {
		value = maxSurfaceDimension
	}
	if value < 1 {
		value = 1
	}

	return value
}

func validBufferBounds(offset, width, height, stride, poolSize int) bool {
	if offset < 0 || width <= 0 || height <= 0 || stride <= 0 || poolSize <= 0 {
		return false
	}

	minStride := int64(width) * 4
	if int64(stride) < minStride {
		return false
	}

	end := int64(offset) + int64(stride)*int64(height)
	if end < int64(offset) || end > int64(poolSize) {
		return false
	}

	return true
}
