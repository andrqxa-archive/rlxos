package svg

import (
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/vector"
)

func svgAttr(n svgNode, key string) string {
	for _, a := range n.Attrs {
		if strings.EqualFold(a.Name.Local, key) {
			return a.Value
		}
	}
	return ""
}

func svgParseFloatList(s string) []float64 {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		if v, err := strconv.ParseFloat(p, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func svgParsePoints(s string) [][2]float64 {
	vals := svgParseFloatList(s)
	if len(vals) < 2 {
		return nil
	}
	n := len(vals) / 2
	out := make([][2]float64, 0, n)
	for i := 0; i+1 < len(vals); i += 2 {
		out = append(out, [2]float64{vals[i], vals[i+1]})
	}
	return out
}

func svgParseDashArray(s string) []float64 {
	v := strings.TrimSpace(strings.ToLower(s))
	if v == "" || v == "none" {
		return nil
	}
	nums := svgParseFloatList(v)
	out := make([]float64, 0, len(nums))
	for _, n := range nums {
		if n > 0 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	if len(out)%2 == 1 {
		dup := make([]float64, len(out))
		copy(dup, out)
		out = append(out, dup...)
	}
	return out
}

func svgParseLength(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "px")
	return svgParseNumber(s, 0)
}

func svgParseNumber(s string, def float64) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return def
	}
	if strings.HasSuffix(s, "%") {
		f, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil {
			return def
		}
		return f / 100.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return f
}

func svgParseOffset(s string) float64 {
	o := svgParseNumber(s, 0)
	if o < 0 {
		o = 0
	}
	if o > 1 {
		o = 1
	}
	return o
}

func svgClamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func svgParseColor(s string) (Color, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "none" {
		return ColorTransparent, false
	}
	if strings.HasPrefix(s, "#") {
		h := strings.TrimPrefix(s, "#")
		switch len(h) {
		case 4:
			r, _ := strconv.ParseUint(strings.Repeat(string(h[0]), 2), 16, 8)
			g, _ := strconv.ParseUint(strings.Repeat(string(h[1]), 2), 16, 8)
			b, _ := strconv.ParseUint(strings.Repeat(string(h[2]), 2), 16, 8)
			a, _ := strconv.ParseUint(strings.Repeat(string(h[3]), 2), 16, 8)
			return Color{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, true
		case 3:
			r, _ := strconv.ParseUint(strings.Repeat(string(h[0]), 2), 16, 8)
			g, _ := strconv.ParseUint(strings.Repeat(string(h[1]), 2), 16, 8)
			b, _ := strconv.ParseUint(strings.Repeat(string(h[2]), 2), 16, 8)
			return Color{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, true
		case 8:
			v, err := strconv.ParseUint(h, 16, 32)
			if err != nil {
				return ColorTransparent, false
			}
			return Color{R: uint8(v >> 24), G: uint8((v >> 16) & 0xFF), B: uint8((v >> 8) & 0xFF), A: uint8(v & 0xFF)}, true
		case 6:
			v, err := strconv.ParseUint(h, 16, 32)
			if err != nil {
				return ColorTransparent, false
			}
			return Color{R: uint8(v >> 16), G: uint8((v >> 8) & 0xFF), B: uint8(v & 0xFF), A: 255}, true
		}
	}
	if strings.HasPrefix(s, "rgb(") && strings.HasSuffix(s, ")") {
		parts := strings.Split(strings.TrimSpace(s[4:len(s)-1]), ",")
		if len(parts) == 3 {
			r := uint8(svgColorComp(parts[0]))
			g := uint8(svgColorComp(parts[1]))
			b := uint8(svgColorComp(parts[2]))
			return Color{R: r, G: g, B: b, A: 255}, true
		}
	}
	if strings.HasPrefix(s, "rgba(") && strings.HasSuffix(s, ")") {
		parts := strings.Split(strings.TrimSpace(s[5:len(s)-1]), ",")
		if len(parts) == 4 {
			r := uint8(svgColorComp(parts[0]))
			g := uint8(svgColorComp(parts[1]))
			b := uint8(svgColorComp(parts[2]))
			a := uint8(svgClamp01(svgParseNumber(parts[3], 1))*255 + 0.5)
			return Color{R: r, G: g, B: b, A: a}, true
		}
	}
	switch s {
	case "black":
		return Color{R: 0, G: 0, B: 0, A: 255}, true
	case "white":
		return Color{R: 255, G: 255, B: 255, A: 255}, true
	case "red":
		return Color{R: 255, G: 0, B: 0, A: 255}, true
	case "green":
		return Color{R: 0, G: 128, B: 0, A: 255}, true
	case "blue":
		return Color{R: 0, G: 0, B: 255, A: 255}, true
	case "yellow":
		return Color{R: 255, G: 255, B: 0, A: 255}, true
	case "gray", "grey":
		return Color{R: 128, G: 128, B: 128, A: 255}, true
	case "silver":
		return Color{R: 192, G: 192, B: 192, A: 255}, true
	case "cyan", "aqua":
		return Color{R: 0, G: 255, B: 255, A: 255}, true
	case "magenta", "fuchsia":
		return Color{R: 255, G: 0, B: 255, A: 255}, true
	case "transparent":
		return ColorTransparent, true
	}
	return ColorTransparent, false
}

func svgColorComp(s string) float64 {
	v := strings.TrimSpace(s)
	if strings.HasSuffix(v, "%") {
		p := svgClamp01(svgParseNumber(v, 0))
		return p * 255
	}
	n := svgParseNumber(v, 0)
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

func svgParseTransform(s string) svgMatrix {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return svgIdentity()
	}
	m := svgIdentity()
	for len(s) > 0 {
		i := strings.IndexByte(s, '(')
		j := strings.IndexByte(s, ')')
		if i <= 0 || j <= i {
			break
		}
		name := strings.TrimSpace(s[:i])
		vals := svgParseFloatList(strings.ReplaceAll(s[i+1:j], ",", " "))
		tm := svgIdentity()
		switch name {
		case "matrix":
			if len(vals) == 6 {
				tm = svgMatrix{a: vals[0], b: vals[1], c: vals[2], d: vals[3], e: vals[4], f: vals[5]}
			}
		case "translate":
			tx, ty := 0.0, 0.0
			if len(vals) > 0 {
				tx = vals[0]
			}
			if len(vals) > 1 {
				ty = vals[1]
			}
			tm = svgMatrix{a: 1, d: 1, e: tx, f: ty}
		case "scale":
			sx, sy := 1.0, 1.0
			if len(vals) > 0 {
				sx = vals[0]
				sy = sx
			}
			if len(vals) > 1 {
				sy = vals[1]
			}
			tm = svgMatrix{a: sx, d: sy}
		case "rotate":
			if len(vals) > 0 {
				ang := vals[0] * math.Pi / 180.0
				ca, sa := math.Cos(ang), math.Sin(ang)
				rot := svgMatrix{a: ca, b: sa, c: -sa, d: ca}
				if len(vals) > 2 {
					cx, cy := vals[1], vals[2]
					tm = svgMul(svgMul(svgMatrix{a: 1, d: 1, e: cx, f: cy}, rot), svgMatrix{a: 1, d: 1, e: -cx, f: -cy})
				} else {
					tm = rot
				}
			}
		case "skewx":
			if len(vals) > 0 {
				tm = svgMatrix{a: 1, c: math.Tan(vals[0] * math.Pi / 180.0), d: 1}
			}
		case "skewy":
			if len(vals) > 0 {
				tm = svgMatrix{a: 1, b: math.Tan(vals[0] * math.Pi / 180.0), d: 1}
			}
		}
		m = svgMul(m, tm)
		if j+1 >= len(s) {
			break
		}
		s = strings.TrimSpace(s[j+1:])
	}
	return m
}

func svgPathToRasterizer(z *vector.Rasterizer, d string, ctm svgMatrix) bool {
	tok := svgPathTokenizer{s: d}
	var cmd byte
	var cx, cy float64
	var sx, sy float64
	var pcx, pcy float64
	var hasPrevCtrl bool
	started := false

	for {
		tok.skipSep()
		if tok.done() {
			break
		}
		if ch := tok.peek(); svgIsCmd(ch) {
			cmd = ch
			tok.i++
		} else if cmd == 0 {
			return false
		}

		switch cmd {
		case 'M', 'm':
			x, ok := tok.nextNumber()
			if !ok {
				return started
			}
			y, ok := tok.nextNumber()
			if !ok {
				return started
			}
			if cmd == 'm' {
				x += cx
				y += cy
			}
			cx, cy = x, y
			sx, sy = x, y
			tx, ty := ctm.apply(x, y)
			z.MoveTo(float32(tx), float32(ty))
			started = true
			hasPrevCtrl = false
			if cmd == 'M' {
				cmd = 'L'
			} else {
				cmd = 'l'
			}
		case 'L', 'l':
			for {
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					return started
				}
				if cmd == 'l' {
					x += cx
					y += cy
				}
				cx, cy = x, y
				tx, ty := ctm.apply(x, y)
				z.LineTo(float32(tx), float32(ty))
				started = true
				hasPrevCtrl = false
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'H', 'h':
			for {
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'h' {
					x += cx
				}
				cx = x
				tx, ty := ctm.apply(cx, cy)
				z.LineTo(float32(tx), float32(ty))
				started = true
				hasPrevCtrl = false
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'V', 'v':
			for {
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'v' {
					y += cy
				}
				cy = y
				tx, ty := ctm.apply(cx, cy)
				z.LineTo(float32(tx), float32(ty))
				started = true
				hasPrevCtrl = false
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'C', 'c':
			for {
				x1, ok := tok.nextNumber()
				if !ok {
					break
				}
				y1, ok := tok.nextNumber()
				if !ok {
					return started
				}
				x2, ok := tok.nextNumber()
				if !ok {
					return started
				}
				y2, ok := tok.nextNumber()
				if !ok {
					return started
				}
				x, ok := tok.nextNumber()
				if !ok {
					return started
				}
				y, ok := tok.nextNumber()
				if !ok {
					return started
				}
				if cmd == 'c' {
					x1 += cx
					y1 += cy
					x2 += cx
					y2 += cy
					x += cx
					y += cy
				}
				tx1, ty1 := ctm.apply(x1, y1)
				tx2, ty2 := ctm.apply(x2, y2)
				tx, ty := ctm.apply(x, y)
				z.CubeTo(float32(tx1), float32(ty1), float32(tx2), float32(ty2), float32(tx), float32(ty))
				pcx, pcy = x2, y2
				hasPrevCtrl = true
				cx, cy = x, y
				started = true
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'S', 's':
			for {
				x2, ok := tok.nextNumber()
				if !ok {
					break
				}
				y2, ok := tok.nextNumber()
				if !ok {
					return started
				}
				x, ok := tok.nextNumber()
				if !ok {
					return started
				}
				y, ok := tok.nextNumber()
				if !ok {
					return started
				}
				x1, y1 := cx, cy
				if hasPrevCtrl {
					x1 = 2*cx - pcx
					y1 = 2*cy - pcy
				}
				if cmd == 's' {
					x2 += cx
					y2 += cy
					x += cx
					y += cy
				}
				tx1, ty1 := ctm.apply(x1, y1)
				tx2, ty2 := ctm.apply(x2, y2)
				tx, ty := ctm.apply(x, y)
				z.CubeTo(float32(tx1), float32(ty1), float32(tx2), float32(ty2), float32(tx), float32(ty))
				pcx, pcy = x2, y2
				hasPrevCtrl = true
				cx, cy = x, y
				started = true
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'Q', 'q':
			for {
				x1, ok := tok.nextNumber()
				if !ok {
					break
				}
				y1, ok := tok.nextNumber()
				if !ok {
					return started
				}
				x, ok := tok.nextNumber()
				if !ok {
					return started
				}
				y, ok := tok.nextNumber()
				if !ok {
					return started
				}
				if cmd == 'q' {
					x1 += cx
					y1 += cy
					x += cx
					y += cy
				}
				tx1, ty1 := ctm.apply(x1, y1)
				tx, ty := ctm.apply(x, y)
				z.QuadTo(float32(tx1), float32(ty1), float32(tx), float32(ty))
				pcx, pcy = x1, y1
				hasPrevCtrl = true
				cx, cy = x, y
				started = true
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'T', 't':
			for {
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					return started
				}
				x1, y1 := cx, cy
				if hasPrevCtrl {
					x1 = 2*cx - pcx
					y1 = 2*cy - pcy
				}
				if cmd == 't' {
					x += cx
					y += cy
				}
				tx1, ty1 := ctm.apply(x1, y1)
				tx, ty := ctm.apply(x, y)
				z.QuadTo(float32(tx1), float32(ty1), float32(tx), float32(ty))
				pcx, pcy = x1, y1
				hasPrevCtrl = true
				cx, cy = x, y
				started = true
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'A', 'a':
			for {
				rx, ok := tok.nextNumber()
				if !ok {
					break
				}
				ry, ok := tok.nextNumber()
				if !ok {
					return started
				}
				rot, ok := tok.nextNumber()
				if !ok {
					return started
				}
				large, ok := tok.nextNumber()
				if !ok {
					return started
				}
				sweep, ok := tok.nextNumber()
				if !ok {
					return started
				}
				x, ok := tok.nextNumber()
				if !ok {
					return started
				}
				y, ok := tok.nextNumber()
				if !ok {
					return started
				}
				if cmd == 'a' {
					x += cx
					y += cy
				}
				curves := svgArcToCubicSegments(cx, cy, rx, ry, rot, large >= 0.5, sweep >= 0.5, x, y)
				if len(curves) == 0 {
					tx, ty := ctm.apply(x, y)
					z.LineTo(float32(tx), float32(ty))
				} else {
					for _, c := range curves {
						x1, y1 := ctm.apply(c[0], c[1])
						x2, y2 := ctm.apply(c[2], c[3])
						x3, y3 := ctm.apply(c[4], c[5])
						z.CubeTo(float32(x1), float32(y1), float32(x2), float32(y2), float32(x3), float32(y3))
					}
				}
				cx, cy = x, y
				hasPrevCtrl = false
				started = true
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'Z', 'z':
			z.ClosePath()
			cx, cy = sx, sy
			hasPrevCtrl = false
			started = true
		default:
			// Unsupported command (e.g. Arc). Skip gracefully.
			return started
		}
	}
	return started
}

func svgPathToStrokePolylines(d string, ctm svgMatrix) []svgPolyline {
	tok := svgPathTokenizer{s: d}
	var cmd byte
	var cx, cy float64
	var sx, sy float64
	var pcx, pcy float64
	var hasPrevCtrl bool

	paths := make([]svgPolyline, 0, 4)
	cur := make([][2]float64, 0, 64)
	pushCur := func(closed bool) {
		if len(cur) < 2 {
			cur = cur[:0]
			return
		}
		cp := make([][2]float64, len(cur))
		copy(cp, cur)
		paths = append(paths, svgPolyline{pts: cp, closed: closed})
		cur = cur[:0]
	}
	addPoint := func(x, y float64) {
		tx, ty := ctm.apply(x, y)
		cur = append(cur, [2]float64{tx, ty})
	}

	for {
		tok.skipSep()
		if tok.done() {
			break
		}
		if ch := tok.peek(); svgIsCmd(ch) {
			cmd = ch
			tok.i++
		} else if cmd == 0 {
			break
		}

		switch cmd {
		case 'M', 'm':
			x, ok := tok.nextNumber()
			if !ok {
				break
			}
			y, ok := tok.nextNumber()
			if !ok {
				break
			}
			if cmd == 'm' {
				x += cx
				y += cy
			}
			pushCur(false)
			cx, cy = x, y
			sx, sy = x, y
			addPoint(x, y)
			hasPrevCtrl = false
			if cmd == 'M' {
				cmd = 'L'
			} else {
				cmd = 'l'
			}
		case 'L', 'l':
			for {
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'l' {
					x += cx
					y += cy
				}
				cx, cy = x, y
				addPoint(x, y)
				hasPrevCtrl = false
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'H', 'h':
			for {
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'h' {
					x += cx
				}
				cx = x
				addPoint(cx, cy)
				hasPrevCtrl = false
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'V', 'v':
			for {
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'v' {
					y += cy
				}
				cy = y
				addPoint(cx, cy)
				hasPrevCtrl = false
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'C', 'c':
			for {
				x1, ok := tok.nextNumber()
				if !ok {
					break
				}
				y1, ok := tok.nextNumber()
				if !ok {
					break
				}
				x2, ok := tok.nextNumber()
				if !ok {
					break
				}
				y2, ok := tok.nextNumber()
				if !ok {
					break
				}
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'c' {
					x1 += cx
					y1 += cy
					x2 += cx
					y2 += cy
					x += cx
					y += cy
				}
				segPts := svgApproxCubic(cx, cy, x1, y1, x2, y2, x, y, 14)
				for _, p := range segPts {
					addPoint(p[0], p[1])
				}
				pcx, pcy = x2, y2
				hasPrevCtrl = true
				cx, cy = x, y
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'S', 's':
			for {
				x2, ok := tok.nextNumber()
				if !ok {
					break
				}
				y2, ok := tok.nextNumber()
				if !ok {
					break
				}
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				x1, y1 := cx, cy
				if hasPrevCtrl {
					x1 = 2*cx - pcx
					y1 = 2*cy - pcy
				}
				if cmd == 's' {
					x2 += cx
					y2 += cy
					x += cx
					y += cy
				}
				segPts := svgApproxCubic(cx, cy, x1, y1, x2, y2, x, y, 14)
				for _, p := range segPts {
					addPoint(p[0], p[1])
				}
				pcx, pcy = x2, y2
				hasPrevCtrl = true
				cx, cy = x, y
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'Q', 'q':
			for {
				x1, ok := tok.nextNumber()
				if !ok {
					break
				}
				y1, ok := tok.nextNumber()
				if !ok {
					break
				}
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'q' {
					x1 += cx
					y1 += cy
					x += cx
					y += cy
				}
				segPts := svgApproxQuad(cx, cy, x1, y1, x, y, 12)
				for _, p := range segPts {
					addPoint(p[0], p[1])
				}
				pcx, pcy = x1, y1
				hasPrevCtrl = true
				cx, cy = x, y
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'T', 't':
			for {
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				x1, y1 := cx, cy
				if hasPrevCtrl {
					x1 = 2*cx - pcx
					y1 = 2*cy - pcy
				}
				if cmd == 't' {
					x += cx
					y += cy
				}
				segPts := svgApproxQuad(cx, cy, x1, y1, x, y, 12)
				for _, p := range segPts {
					addPoint(p[0], p[1])
				}
				pcx, pcy = x1, y1
				hasPrevCtrl = true
				cx, cy = x, y
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'A', 'a':
			for {
				rx, ok := tok.nextNumber()
				if !ok {
					break
				}
				ry, ok := tok.nextNumber()
				if !ok {
					break
				}
				rot, ok := tok.nextNumber()
				if !ok {
					break
				}
				large, ok := tok.nextNumber()
				if !ok {
					break
				}
				sweep, ok := tok.nextNumber()
				if !ok {
					break
				}
				x, ok := tok.nextNumber()
				if !ok {
					break
				}
				y, ok := tok.nextNumber()
				if !ok {
					break
				}
				if cmd == 'a' {
					x += cx
					y += cy
				}
				curves := svgArcToCubicSegments(cx, cy, rx, ry, rot, large >= 0.5, sweep >= 0.5, x, y)
				for _, c := range curves {
					segPts := svgApproxCubic(cx, cy, c[0], c[1], c[2], c[3], c[4], c[5], 10)
					for _, p := range segPts {
						addPoint(p[0], p[1])
					}
					cx, cy = c[4], c[5]
				}
				cx, cy = x, y
				hasPrevCtrl = false
				tok.skipSep()
				if tok.done() || svgIsCmd(tok.peek()) {
					break
				}
			}
		case 'Z', 'z':
			cx, cy = sx, sy
			pushCur(true)
			hasPrevCtrl = false
		default:
			break
		}
	}
	pushCur(false)
	return paths
}

func svgApproxQuad(x0, y0, x1, y1, x2, y2 float64, steps int) [][2]float64 {
	if steps < 1 {
		steps = 1
	}
	out := make([][2]float64, 0, steps)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		mt := 1 - t
		x := mt*mt*x0 + 2*mt*t*x1 + t*t*x2
		y := mt*mt*y0 + 2*mt*t*y1 + t*t*y2
		out = append(out, [2]float64{x, y})
	}
	return out
}

func svgApproxCubic(x0, y0, x1, y1, x2, y2, x3, y3 float64, steps int) [][2]float64 {
	if steps < 1 {
		steps = 1
	}
	out := make([][2]float64, 0, steps)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		mt := 1 - t
		x := mt*mt*mt*x0 + 3*mt*mt*t*x1 + 3*mt*t*t*x2 + t*t*t*x3
		y := mt*mt*mt*y0 + 3*mt*mt*t*y1 + 3*mt*t*t*y2 + t*t*t*y3
		out = append(out, [2]float64{x, y})
	}
	return out
}

func svgArcToCubicSegments(x1, y1, rx, ry, xAxisRot float64, largeArc, sweep bool, x2, y2 float64) [][6]float64 {
	if (x1 == x2 && y1 == y2) || rx == 0 || ry == 0 {
		return nil
	}
	rx = math.Abs(rx)
	ry = math.Abs(ry)
	phi := xAxisRot * math.Pi / 180.0
	cphi, sphi := math.Cos(phi), math.Sin(phi)

	dx2 := (x1 - x2) / 2.0
	dy2 := (y1 - y2) / 2.0
	x1p := cphi*dx2 + sphi*dy2
	y1p := -sphi*dx2 + cphi*dy2

	lambda := (x1p*x1p)/(rx*rx) + (y1p*y1p)/(ry*ry)
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx *= scale
		ry *= scale
	}

	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	if den == 0 {
		return nil
	}
	k := num / den
	if k < 0 {
		k = 0
	}
	sign := 1.0
	if largeArc == sweep {
		sign = -1.0
	}
	coef := sign * math.Sqrt(k)

	cxp := coef * (rx * y1p / ry)
	cyp := coef * (-ry * x1p / rx)

	cx := cphi*cxp - sphi*cyp + (x1+x2)/2.0
	cy := sphi*cxp + cphi*cyp + (y1+y2)/2.0

	ux := (x1p - cxp) / rx
	uy := (y1p - cyp) / ry
	vx := (-x1p - cxp) / rx
	vy := (-y1p - cyp) / ry

	theta1 := svgVecAngle(1, 0, ux, uy)
	dTheta := svgVecAngle(ux, uy, vx, vy)
	if !sweep && dTheta > 0 {
		dTheta -= 2 * math.Pi
	} else if sweep && dTheta < 0 {
		dTheta += 2 * math.Pi
	}

	segs := int(math.Ceil(math.Abs(dTheta) / (math.Pi / 2)))
	if segs < 1 {
		segs = 1
	}
	step := dTheta / float64(segs)
	out := make([][6]float64, 0, segs)
	for i := 0; i < segs; i++ {
		t1 := theta1 + float64(i)*step
		t2 := t1 + step
		sin1, cos1 := math.Sin(t1), math.Cos(t1)
		sin2, cos2 := math.Sin(t2), math.Cos(t2)
		alpha := 4.0 / 3.0 * math.Tan((t2-t1)/4.0)

		p1x, p1y := cos1, sin1
		p2x, p2y := cos2, sin2
		c1x, c1y := p1x-alpha*p1y, p1y+alpha*p1x
		c2x, c2y := p2x+alpha*p2y, p2y-alpha*p2x

		xc1, yc1 := svgMapEllipsePoint(c1x, c1y, rx, ry, cphi, sphi, cx, cy)
		xc2, yc2 := svgMapEllipsePoint(c2x, c2y, rx, ry, cphi, sphi, cx, cy)
		xe, ye := svgMapEllipsePoint(p2x, p2y, rx, ry, cphi, sphi, cx, cy)
		out = append(out, [6]float64{xc1, yc1, xc2, yc2, xe, ye})
	}
	return out
}

func svgMapEllipsePoint(u, v, rx, ry, cphi, sphi, cx, cy float64) (float64, float64) {
	x := cphi*(rx*u) - sphi*(ry*v) + cx
	y := sphi*(rx*u) + cphi*(ry*v) + cy
	return x, y
}

func svgVecAngle(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	det := ux*vy - uy*vx
	return math.Atan2(det, dot)
}

type svgPathTokenizer struct {
	s string
	i int
}

func (t *svgPathTokenizer) done() bool { return t.i >= len(t.s) }
func (t *svgPathTokenizer) peek() byte { return t.s[t.i] }

func (t *svgPathTokenizer) skipSep() {
	for !t.done() {
		c := t.s[t.i]
		if c == ',' || c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			t.i++
			continue
		}
		break
	}
}

func (t *svgPathTokenizer) nextNumber() (float64, bool) {
	t.skipSep()
	if t.done() {
		return 0, false
	}
	start := t.i
	if t.s[t.i] == '+' || t.s[t.i] == '-' {
		t.i++
	}
	dot := false
	exp := false
	for !t.done() {
		c := t.s[t.i]
		switch {
		case c >= '0' && c <= '9':
			t.i++
		case c == '.' && !dot:
			dot = true
			t.i++
		case (c == 'e' || c == 'E') && !exp:
			exp = true
			t.i++
			if !t.done() && (t.s[t.i] == '+' || t.s[t.i] == '-') {
				t.i++
			}
		default:
			goto done
		}
	}
done:
	if t.i == start {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(t.s[start:t.i]), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func svgIsCmd(c byte) bool {
	switch c {
	case 'M', 'm', 'L', 'l', 'H', 'h', 'V', 'v', 'C', 'c', 'S', 's', 'Q', 'q', 'T', 't', 'A', 'a', 'Z', 'z':
		return true
	default:
		return false
	}
}
