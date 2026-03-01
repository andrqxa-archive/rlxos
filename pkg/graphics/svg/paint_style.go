package svg

import (
	"encoding/xml"
	"image/color"
	"math"
	"regexp"
	"sort"
	"strings"

	core "avyos.dev/pkg/graphics/pixmap"
)

type svgPaint interface {
	sample(x, y float64) color.NRGBA
}

type svgSolidPaint struct{ c color.NRGBA }

func (p svgSolidPaint) sample(_, _ float64) color.NRGBA { return p.c }

type svgLinearPaint struct {
	x1, y1 float64
	x2, y2 float64
	spread string
	stops  []svgGradientStop
}

func (p svgLinearPaint) sample(x, y float64) color.NRGBA {
	dx := p.x2 - p.x1
	dy := p.y2 - p.y1
	den := dx*dx + dy*dy
	t := 0.0
	if den > 1e-9 {
		t = ((x-p.x1)*dx + (y-p.y1)*dy) / den
	}
	t = svgApplySpread(t, p.spread)
	return svgSampleStops(p.stops, t)
}

type svgRadialPaint struct {
	cx, cy float64
	r      float64
	spread string
	stops  []svgGradientStop
}

func (p svgRadialPaint) sample(x, y float64) color.NRGBA {
	if p.r <= 1e-9 {
		return svgSampleStops(p.stops, 1)
	}
	dx := x - p.cx
	dy := y - p.cy
	t := math.Sqrt(dx*dx+dy*dy) / p.r
	t = svgApplySpread(t, p.spread)
	return svgSampleStops(p.stops, t)
}

func svgResolveGradient(id string, defs *svgDefs, depth int) (svgGradient, bool) {
	if defs == nil || depth > 12 {
		return svgGradient{}, false
	}
	g, ok := defs.gradients[id]
	if !ok {
		return svgGradient{}, false
	}
	if g.href == "" {
		return g, true
	}
	base, ok := svgResolveGradient(g.href, defs, depth+1)
	if !ok {
		return g, true
	}
	merged := base
	merged.id = g.id
	merged.kind = g.kind
	merged.href = ""
	if g.units != "" {
		merged.units = g.units
	}
	if g.spread != "" {
		merged.spread = g.spread
	}
	if g.xform != (svgMatrix{}) {
		merged.xform = g.xform
	}
	if gn, ok := defs.nodesByID[g.id]; ok {
		if g.kind == "linear" {
			if svgNodeHasAttr(gn, "x1") {
				merged.x1 = g.x1
			}
			if svgNodeHasAttr(gn, "y1") {
				merged.y1 = g.y1
			}
			if svgNodeHasAttr(gn, "x2") {
				merged.x2 = g.x2
			}
			if svgNodeHasAttr(gn, "y2") {
				merged.y2 = g.y2
			}
		} else if g.kind == "radial" {
			if svgNodeHasAttr(gn, "cx") {
				merged.cx = g.cx
			}
			if svgNodeHasAttr(gn, "cy") {
				merged.cy = g.cy
			}
			if svgNodeHasAttr(gn, "r") {
				merged.r = g.r
			}
		}
	} else {
		if g.kind == "linear" {
			merged.x1, merged.y1, merged.x2, merged.y2 = g.x1, g.y1, g.x2, g.y2
		} else if g.kind == "radial" {
			merged.cx, merged.cy, merged.r = g.cx, g.cy, g.r
		}
	}
	if len(g.stops) > 0 {
		merged.stops = g.stops
	}
	return merged, true
}

func svgNodeHasAttr(n svgNode, key string) bool {
	for _, a := range n.Attrs {
		if strings.EqualFold(strings.TrimSpace(a.Name.Local), strings.TrimSpace(key)) {
			return true
		}
	}
	return false
}

func svgApplySpread(t float64, spread string) float64 {
	switch spread {
	case "repeat":
		t = t - math.Floor(t)
		if t < 0 {
			t += 1
		}
		return t
	case "reflect":
		u := math.Mod(math.Abs(t), 2.0)
		if u > 1 {
			u = 2 - u
		}
		return u
	default: // pad
		if t < 0 {
			return 0
		}
		if t > 1 {
			return 1
		}
		return t
	}
}

type svgPatternPaint struct {
	tile  *core.Buffer
	ox    float64
	oy    float64
	alpha float64
}

func (p svgPatternPaint) sample(x, y float64) color.NRGBA {
	if p.tile == nil || p.tile.Width <= 0 || p.tile.Height <= 0 {
		return core.ColorTransparent
	}
	u := math.Mod(x-p.ox, float64(p.tile.Width))
	v := math.Mod(y-p.oy, float64(p.tile.Height))
	if u < 0 {
		u += float64(p.tile.Width)
	}
	if v < 0 {
		v += float64(p.tile.Height)
	}
	ix := int(u)
	iy := int(v)
	c := p.tile.GetPixel(ix, iy)
	if p.alpha < 1 {
		c.A = uint8(float64(c.A)*p.alpha + 0.5)
	}
	return c
}

func svgResolvePaintWith(paintValue string, op float64, defs *svgDefs, ctm svgMatrix, bbox svgBBox) (svgPaint, bool) {
	fill := strings.TrimSpace(strings.ToLower(paintValue))
	if fill == "" {
		fill = "#000000"
	}
	if op <= 0 {
		return nil, false
	}
	if op > 1 {
		op = 1
	}

	if id := svgParseURLID(paintValue); id != "" {
		if g, ok := svgResolveGradient(id, defs, 0); ok && len(g.stops) > 0 {
			stops := make([]svgGradientStop, len(g.stops))
			copy(stops, g.stops)
			for i := range stops {
				stops[i].color.A = uint8((float64(stops[i].color.A)*op + 0.5))
			}
			gt := g.xform
			if g.units == "userspaceonuse" || g.units == "" {
				gt = svgMul(ctm, gt)
			}
			switch g.kind {
			case "linear":
				x1, y1 := gt.apply(g.x1, g.y1)
				x2, y2 := gt.apply(g.x2, g.y2)
				return svgLinearPaint{x1: x1, y1: y1, x2: x2, y2: y2, spread: g.spread, stops: stops}, true
			case "radial":
				cx, cy := gt.apply(g.cx, g.cy)
				rx, ry := gt.apply(g.cx+g.r, g.cy)
				r := math.Hypot(rx-cx, ry-cy)
				if r <= 1e-9 {
					r = g.r
				}
				return svgRadialPaint{cx: cx, cy: cy, r: r, spread: g.spread, stops: stops}, true
			}
			return nil, false
		}
		if pn, ok := defs.patterns[id]; ok {
			pp, ok := svgBuildPatternPaint(pn, defs, ctm, bbox, op)
			if ok {
				return pp, true
			}
		}
		return nil, false
	}

	if fill == "none" {
		return nil, false
	}
	c, ok := svgParseColor(fill)
	if !ok {
		return nil, false
	}
	c.A = uint8((float64(c.A)*op + 0.5))
	return svgSolidPaint{c: c}, c.A > 0
}

func svgResolveFillPaint(style svgStyle, defs *svgDefs, ctm svgMatrix, bbox svgBBox) (svgPaint, bool) {
	return svgResolvePaintWith(style.fill, style.opacity*style.fillOpacity, defs, ctm, bbox)
}

func svgResolveStrokePaint(style svgStyle, defs *svgDefs, ctm svgMatrix, bbox svgBBox) (svgPaint, bool) {
	return svgResolvePaintWith(style.stroke, style.opacity*style.strokeOp, defs, ctm, bbox)
}

func svgBuildPatternPaint(pn svgNode, defs *svgDefs, ctm svgMatrix, bbox svgBBox, alpha float64) (svgPatternPaint, bool) {
	pn = svgResolvePatternNode(pn, defs, 0)
	w := svgParseLength(svgAttr(pn, "width"))
	h := svgParseLength(svgAttr(pn, "height"))
	x := svgParseLength(svgAttr(pn, "x"))
	y := svgParseLength(svgAttr(pn, "y"))
	units := strings.ToLower(strings.TrimSpace(svgAttr(pn, "patternUnits")))
	if units == "" {
		units = "objectboundingbox"
	}
	contentUnits := strings.ToLower(strings.TrimSpace(svgAttr(pn, "patternContentUnits")))
	if contentUnits == "" {
		contentUnits = "userSpaceOnUse"
	}
	if units == "objectboundingbox" {
		bw, bh := svgNominalPatternBox(ctm)
		bx, by := 0.0, 0.0
		if bbox.valid {
			bx = bbox.minX
			by = bbox.minY
			bw = bbox.maxX - bbox.minX
			bh = bbox.maxY - bbox.minY
			if bw <= 1e-6 {
				bw = 1
			}
			if bh <= 1e-6 {
				bh = 1
			}
		}
		if w <= 0 {
			w = 0.25
		}
		if h <= 0 {
			h = 0.25
		}
		// objectBoundingBox units are fractions in object-bbox space.
		w = w * bw
		h = h * bh
		x = bx + x*bw
		y = by + y*bh
	} else {
		if w <= 0 {
			w = 16
		}
		if h <= 0 {
			h = 16
		}
	}
	tw := int(w + 0.5)
	th := int(h + 0.5)
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	if tw > 1024 {
		tw = 1024
	}
	if th > 1024 {
		th = 1024
	}
	tile := core.NewBuffer(tw, th)
	ps := svgStyle{
		fill:        "#000000",
		fillOpacity: 1,
		fillRule:    "nonzero",
		clipRule:    "nonzero",
		stroke:      "none",
		strokeWidth: 1,
		strokeOp:    1,
		lineCap:     "butt",
		lineJoin:    "miter",
		miterLimit:  4,
		opacity:     1,
		color:       "#000000",
	}
	tctm := svgIdentity()
	viewBoxVals := svgParseFloatList(svgAttr(pn, "viewBox"))
	hasViewBox := len(viewBoxVals) == 4 && viewBoxVals[2] > 0 && viewBoxVals[3] > 0
	if hasViewBox {
		par := strings.TrimSpace(svgAttr(pn, "preserveAspectRatio"))
		tctm = svgMul(tctm, svgPatternViewBoxMatrix(viewBoxVals[0], viewBoxVals[1], viewBoxVals[2], viewBoxVals[3], float64(tw), float64(th), par))
	} else if strings.EqualFold(contentUnits, "objectboundingbox") {
		bx, by, bw, bh := svgBBoxOrNominal(bbox, ctm)
		tctm = svgMul(tctm, svgMatrix{a: bw, d: bh, e: bx, f: by})
	}
	if t := svgAttr(pn, "patternTransform"); t != "" {
		tctm = svgMul(tctm, svgParseTransform(t))
	}
	tctm = svgMul(tctm, svgMatrix{a: 1, d: 1, e: -x, f: -y})
	for _, c := range pn.Nodes {
		svgRenderNode(tile, c, tctm, ps, defs, 1)
	}
	return svgPatternPaint{tile: tile, ox: x, oy: y, alpha: alpha}, true
}

func svgResolvePatternNode(pn svgNode, defs *svgDefs, depth int) svgNode {
	if depth > 8 {
		return pn
	}
	href := strings.TrimSpace(svgAttr(pn, "href"))
	if href == "" {
		href = strings.TrimSpace(svgAttr(pn, "xlink:href"))
	}
	if !strings.HasPrefix(href, "#") {
		return pn
	}
	refID := strings.TrimPrefix(href, "#")
	ref, ok := defs.patterns[refID]
	if !ok {
		return pn
	}
	ref = svgResolvePatternNode(ref, defs, depth+1)

	// Merge attrs: referenced attrs provide defaults, local attrs override.
	mergedAttrs := make([]xml.Attr, 0, len(ref.Attrs)+len(pn.Attrs))
	for _, a := range ref.Attrs {
		mergedAttrs = append(mergedAttrs, a)
	}
	for _, a := range pn.Attrs {
		key := strings.ToLower(strings.TrimSpace(a.Name.Local))
		replaced := false
		for i := range mergedAttrs {
			if strings.ToLower(strings.TrimSpace(mergedAttrs[i].Name.Local)) == key {
				mergedAttrs[i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			mergedAttrs = append(mergedAttrs, a)
		}
	}

	merged := pn
	merged.Attrs = mergedAttrs
	if len(merged.Nodes) == 0 {
		merged.Nodes = ref.Nodes
	}
	return merged
}

func svgNominalPatternBox(ctm svgMatrix) (float64, float64) {
	sx := math.Hypot(ctm.a, ctm.b)
	sy := math.Hypot(ctm.c, ctm.d)
	if sx <= 1e-6 {
		sx = 1
	}
	if sy <= 1e-6 {
		sy = 1
	}
	// Nominal object box estimate in pixels for objectBoundingBox pattern units.
	return 64.0 * sx, 64.0 * sy
}

func svgBBoxOrNominal(bbox svgBBox, ctm svgMatrix) (bx, by, bw, bh float64) {
	if bbox.valid {
		bw = bbox.maxX - bbox.minX
		bh = bbox.maxY - bbox.minY
		if bw > 1e-6 && bh > 1e-6 {
			return bbox.minX, bbox.minY, bw, bh
		}
	}
	bw, bh = svgNominalPatternBox(ctm)
	return 0, 0, bw, bh
}

func svgBBoxFromPoints(pts [][2]float64) svgBBox {
	if len(pts) == 0 {
		return svgBBox{}
	}
	b := svgBBox{
		minX:  pts[0][0],
		minY:  pts[0][1],
		maxX:  pts[0][0],
		maxY:  pts[0][1],
		valid: true,
	}
	for i := 1; i < len(pts); i++ {
		p := pts[i]
		if p[0] < b.minX {
			b.minX = p[0]
		}
		if p[1] < b.minY {
			b.minY = p[1]
		}
		if p[0] > b.maxX {
			b.maxX = p[0]
		}
		if p[1] > b.maxY {
			b.maxY = p[1]
		}
	}
	return b
}

func svgBBoxFromPolylines(paths []svgPolyline) svgBBox {
	b := svgBBox{}
	for _, pl := range paths {
		pb := svgBBoxFromPoints(pl.pts)
		if !pb.valid {
			continue
		}
		if !b.valid {
			b = pb
			continue
		}
		if pb.minX < b.minX {
			b.minX = pb.minX
		}
		if pb.minY < b.minY {
			b.minY = pb.minY
		}
		if pb.maxX > b.maxX {
			b.maxX = pb.maxX
		}
		if pb.maxY > b.maxY {
			b.maxY = pb.maxY
		}
		b.valid = true
	}
	return b
}

func svgPatternViewBoxMatrix(vx, vy, vw, vh, dstW, dstH float64, par string) svgMatrix {
	if vw <= 0 || vh <= 0 || dstW <= 0 || dstH <= 0 {
		return svgIdentity()
	}
	align, mode := svgParsePreserveAspectRatio(par)
	if align == "none" {
		sx := dstW / vw
		sy := dstH / vh
		return svgMatrix{
			a: sx,
			d: sy,
			e: -vx * sx,
			f: -vy * sy,
		}
	}
	scale := math.Min(dstW/vw, dstH/vh)
	if mode == "slice" {
		scale = math.Max(dstW/vw, dstH/vh)
	}
	outW := vw * scale
	outH := vh * scale
	tx := -vx * scale
	ty := -vy * scale
	switch align {
	case "xminymin":
		// no extra offset
	case "xmidymin":
		tx += (dstW - outW) * 0.5
	case "xmaxymin":
		tx += dstW - outW
	case "xminymid":
		ty += (dstH - outH) * 0.5
	case "xmidymid":
		tx += (dstW - outW) * 0.5
		ty += (dstH - outH) * 0.5
	case "xmaxymid":
		tx += dstW - outW
		ty += (dstH - outH) * 0.5
	case "xminymax":
		ty += dstH - outH
	case "xmidymax":
		tx += (dstW - outW) * 0.5
		ty += dstH - outH
	case "xmaxymax":
		tx += dstW - outW
		ty += dstH - outH
	default:
		tx += (dstW - outW) * 0.5
		ty += (dstH - outH) * 0.5
	}
	return svgMatrix{a: scale, d: scale, e: tx, f: ty}
}

func svgParsePreserveAspectRatio(v string) (align string, mode string) {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "" {
		return "xmidymid", "meet"
	}
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "xmidymid", "meet"
	}
	align = parts[0]
	if align == "defer" {
		align = "xmidymid"
		if len(parts) > 1 {
			align = parts[1]
		}
	}
	if align == "none" {
		return "none", "meet"
	}
	mode = "meet"
	for _, p := range parts[1:] {
		if p == "slice" || p == "meet" {
			mode = p
		}
	}
	return align, mode
}

func svgSampleStops(stops []svgGradientStop, t float64) color.NRGBA {
	if len(stops) == 0 {
		return core.ColorTransparent
	}
	if t <= stops[0].offset {
		return stops[0].color
	}
	last := stops[len(stops)-1]
	if t >= last.offset {
		return last.color
	}
	for i := 1; i < len(stops); i++ {
		a := stops[i-1]
		b := stops[i]
		if t > b.offset {
			continue
		}
		u := 0.0
		den := b.offset - a.offset
		if den > 1e-9 {
			u = (t - a.offset) / den
		}
		return color.NRGBA{
			R: uint8(float64(a.color.R) + (float64(b.color.R)-float64(a.color.R))*u + 0.5),
			G: uint8(float64(a.color.G) + (float64(b.color.G)-float64(a.color.G))*u + 0.5),
			B: uint8(float64(a.color.B) + (float64(b.color.B)-float64(a.color.B))*u + 0.5),
			A: uint8(float64(a.color.A) + (float64(b.color.A)-float64(a.color.A))*u + 0.5),
		}
	}
	return last.color
}

func svgMergeStyle(parent svgStyle, n svgNode, classColors map[string]string) svgStyle {
	s := parent
	if cls := strings.TrimSpace(svgAttr(n, "class")); cls != "" {
		for _, cn := range strings.Fields(cls) {
			if c, ok := classColors[cn]; ok {
				s.color = c
				break
			}
		}
	}
	if v := strings.TrimSpace(svgAttr(n, "color")); v != "" {
		s.color = v
	}

	applyKV := func(k, v string) {
		k = strings.TrimSpace(strings.ToLower(k))
		v = strings.TrimSpace(v)
		switch k {
		case "fill":
			if strings.EqualFold(v, "currentcolor") {
				s.fill = s.color
			} else {
				s.fill = v
			}
		case "stroke":
			if strings.EqualFold(v, "currentcolor") {
				s.stroke = s.color
			} else {
				s.stroke = v
			}
		case "fill-opacity":
			s.fillOpacity = svgClamp01(svgParseNumber(v, s.fillOpacity))
		case "fill-rule":
			s.fillRule = strings.ToLower(strings.TrimSpace(v))
		case "clip-rule":
			s.clipRule = strings.ToLower(strings.TrimSpace(v))
		case "stroke-opacity":
			s.strokeOp = svgClamp01(svgParseNumber(v, s.strokeOp))
		case "stroke-width":
			s.strokeWidth = svgParseLength(v)
			if s.strokeWidth < 0 {
				s.strokeWidth = 0
			}
		case "stroke-linecap":
			s.lineCap = strings.ToLower(v)
		case "stroke-linejoin":
			s.lineJoin = strings.ToLower(v)
		case "stroke-miterlimit":
			s.miterLimit = svgParseNumber(v, s.miterLimit)
			if s.miterLimit < 1 {
				s.miterLimit = 1
			}
		case "stroke-dasharray":
			s.dashArray = svgParseDashArray(v)
		case "stroke-dashoffset":
			s.dashOffset = svgParseLength(v)
		case "opacity":
			s.opacity = svgClamp01(svgParseNumber(v, s.opacity))
		case "display":
			s.displayNone = strings.EqualFold(v, "none")
		case "color":
			s.color = v
		}
	}

	if st := strings.TrimSpace(svgAttr(n, "style")); st != "" {
		parts := strings.Split(st, ";")
		for _, p := range parts {
			kv := strings.SplitN(p, ":", 2)
			if len(kv) == 2 {
				applyKV(kv[0], kv[1])
			}
		}
	}
	for _, a := range []string{
		"fill", "fill-opacity", "fill-rule", "clip-rule",
		"stroke", "stroke-opacity", "stroke-width", "stroke-linecap", "stroke-linejoin", "stroke-miterlimit", "stroke-dasharray", "stroke-dashoffset",
		"opacity", "display", "color",
	} {
		if v := svgAttr(n, a); v != "" {
			applyKV(a, v)
		}
	}
	if strings.EqualFold(strings.TrimSpace(s.fill), "currentcolor") {
		s.fill = s.color
	}
	if strings.EqualFold(strings.TrimSpace(s.stroke), "currentcolor") {
		s.stroke = s.color
	}
	return s
}

func svgParseGradient(n svgNode) svgGradient {
	tag := strings.ToLower(n.XMLName.Local)
	href := strings.TrimSpace(svgAttr(n, "href"))
	if href == "" {
		href = strings.TrimSpace(svgAttr(n, "xlink:href"))
	}
	if strings.HasPrefix(href, "#") {
		href = strings.TrimPrefix(href, "#")
	} else {
		href = ""
	}
	g := svgGradient{
		id:     svgAttr(n, "id"),
		kind:   tag,
		href:   href,
		units:  strings.ToLower(svgAttr(n, "gradientUnits")),
		spread: strings.ToLower(strings.TrimSpace(svgAttr(n, "spreadMethod"))),
		xform:  svgParseTransform(svgAttr(n, "gradientTransform")),
	}
	if g.xform == (svgMatrix{}) {
		g.xform = svgIdentity()
	}
	if g.spread == "" {
		g.spread = "pad"
	}
	if tag == "lineargradient" {
		g.kind = "linear"
		g.x1 = svgParseNumber(svgAttr(n, "x1"), 0)
		g.y1 = svgParseNumber(svgAttr(n, "y1"), 0)
		g.x2 = svgParseNumber(svgAttr(n, "x2"), 1)
		g.y2 = svgParseNumber(svgAttr(n, "y2"), 0)
	} else {
		g.kind = "radial"
		g.cx = svgParseNumber(svgAttr(n, "cx"), 0.5)
		g.cy = svgParseNumber(svgAttr(n, "cy"), 0.5)
		g.r = svgParseNumber(svgAttr(n, "r"), 0.5)
	}
	for _, c := range n.Nodes {
		if strings.ToLower(c.XMLName.Local) != "stop" {
			continue
		}
		off := svgParseOffset(svgAttr(c, "offset"))
		stopColor := svgAttr(c, "stop-color")
		stopOpacity := svgParseNumber(svgAttr(c, "stop-opacity"), 1)
		if st := svgAttr(c, "style"); st != "" {
			for _, p := range strings.Split(st, ";") {
				kv := strings.SplitN(p, ":", 2)
				if len(kv) != 2 {
					continue
				}
				k := strings.TrimSpace(strings.ToLower(kv[0]))
				v := strings.TrimSpace(kv[1])
				switch k {
				case "stop-color":
					stopColor = v
				case "stop-opacity":
					stopOpacity = svgParseNumber(v, stopOpacity)
				}
			}
		}
		if stopColor == "" {
			stopColor = "#000000"
		}
		cc, ok := svgParseColor(stopColor)
		if !ok {
			continue
		}
		cc.A = uint8(float64(cc.A)*svgClamp01(stopOpacity) + 0.5)
		g.stops = append(g.stops, svgGradientStop{offset: off, color: cc})
	}
	sort.SliceStable(g.stops, func(i, j int) bool { return g.stops[i].offset < g.stops[j].offset })
	return g
}

func svgParseClassColors(css string, out map[string]string) {
	re := regexp.MustCompile(`\.([A-Za-z0-9_-]+)\s*\{[^}]*?color\s*:\s*([^;\}]+)`) // class + color prop
	for _, m := range re.FindAllStringSubmatch(css, -1) {
		if len(m) != 3 {
			continue
		}
		out[strings.TrimSpace(m[1])] = strings.TrimSpace(m[2])
	}
}
