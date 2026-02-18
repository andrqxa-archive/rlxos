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

package core

// Point represents a 2D point.
type Point struct {
	X, Y int
}

// Rect represents a rectangle with position and size.
type Rect struct {
	X, Y int
	W, H int
}

// Contains returns true if the point is inside the rectangle.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}

// ContainsXY returns true if the coordinates are inside the rectangle.
func (r Rect) ContainsXY(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Intersects returns true if this rectangle intersects with another.
func (r Rect) Intersects(other Rect) bool {
	return r.X < other.X+other.W && r.X+r.W > other.X &&
		r.Y < other.Y+other.H && r.Y+r.H > other.Y
}

// Intersection returns the intersection of two rectangles.
func (r Rect) Intersection(other Rect) Rect {
	x := max(r.X, other.X)
	y := max(r.Y, other.Y)
	x2 := min(r.X+r.W, other.X+other.W)
	y2 := min(r.Y+r.H, other.Y+other.H)

	if x >= x2 || y >= y2 {
		return Rect{}
	}
	return Rect{X: x, Y: y, W: x2 - x, H: y2 - y}
}

// Union returns the smallest rectangle containing both rectangles.
func (r Rect) Union(other Rect) Rect {
	if r.W == 0 || r.H == 0 {
		return other
	}
	if other.W == 0 || other.H == 0 {
		return r
	}

	x := min(r.X, other.X)
	y := min(r.Y, other.Y)
	x2 := max(r.X+r.W, other.X+other.W)
	y2 := max(r.Y+r.H, other.Y+other.H)

	return Rect{X: x, Y: y, W: x2 - x, H: y2 - y}
}

// Inset returns a rectangle inset by the given amounts.
func (r Rect) Inset(top, right, bottom, left int) Rect {
	return Rect{
		X: r.X + left,
		Y: r.Y + top,
		W: r.W - left - right,
		H: r.H - top - bottom,
	}
}

// InsetAll returns a rectangle inset by the same amount on all sides.
func (r Rect) InsetAll(amount int) Rect {
	return r.Inset(amount, amount, amount, amount)
}

// IsEmpty returns true if the rectangle has zero area.
func (r Rect) IsEmpty() bool {
	return r.W <= 0 || r.H <= 0
}

// Size returns the size of the rectangle as a Point.
func (r Rect) Size() Point {
	return Point{X: r.W, Y: r.H}
}

// Position returns the position of the rectangle as a Point.
func (r Rect) Position() Point {
	return Point{X: r.X, Y: r.Y}
}

// Center returns the center point of the rectangle.
func (r Rect) Center() Point {
	return Point{X: r.X + r.W/2, Y: r.Y + r.H/2}
}

// NewRect creates a new rectangle.
func NewRect(x, y, w, h int) Rect {
	return Rect{X: x, Y: y, W: w, H: h}
}
