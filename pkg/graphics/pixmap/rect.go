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

package pixmap

import "image"

func RectXYWH(x, y, w, h int) image.Rectangle {
	if w <= 0 || h <= 0 {
		return image.Rectangle{Min: image.Point{X: x, Y: y}, Max: image.Point{X: x, Y: y}}
	}
	return image.Rect(x, y, x+w, y+h)
}

func RectContainsXY(r image.Rectangle, x, y int) bool {
	return x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y
}

func RectContainsPoint(r image.Rectangle, p image.Point) bool {
	return RectContainsXY(r, p.X, p.Y)
}

func RectIntersects(a, b image.Rectangle) bool {
	return !a.Intersect(b).Empty()
}

func RectInsetLTRB(r image.Rectangle, top, right, bottom, left int) image.Rectangle {
	return image.Rectangle{
		Min: image.Point{X: r.Min.X + left, Y: r.Min.Y + top},
		Max: image.Point{X: r.Max.X - right, Y: r.Max.Y - bottom},
	}
}

func RectInsetAll(r image.Rectangle, amount int) image.Rectangle {
	return RectInsetLTRB(r, amount, amount, amount, amount)
}

func RectCenter(r image.Rectangle) image.Point {
	return image.Point{X: r.Min.X + r.Dx()/2, Y: r.Min.Y + r.Dy()/2}
}
