package svg

import (
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"sort"
	"strings"

	"golang.org/x/image/vector"
)

type svgNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Text    string     `xml:",chardata"`
	Nodes   []svgNode  `xml:",any"`
}

type svgStyle struct {
	fill        string
	fillOpacity float64
	fillRule    string
	clipRule    string
	stroke      string
	strokeWidth float64
	strokeOp    float64
	lineCap     string
	lineJoin    string
	miterLimit  float64
	dashArray   []float64
	dashOffset  float64
	opacity     float64
	color       string
	displayNone bool
}

type svgGradientStop struct {
	offset float64
	color  Color
}

type svgGradient struct {
	id        string
	kind      string // linear|radial
	href      string
	x1, y1    float64
	x2, y2    float64
	cx, cy, r float64
	units     string
	spread    string
	xform     svgMatrix
	stops     []svgGradientStop
}

type svgPolyline struct {
	pts    [][2]float64
	closed bool
}

type svgTextChunk struct {
	text   string
	x, y   float64
	hasAbs bool
	dx, dy float64
	anchor string
}

type svgBBox struct {
	minX  float64
	minY  float64
	maxX  float64
	maxY  float64
	valid bool
}

type svgDefs struct {
	gradients   map[string]svgGradient
	patterns    map[string]svgNode
	classColors map[string]string
	symbols     map[string]svgNode
	nodesByID   map[string]svgNode
	clipPaths   map[string]svgNode
	masks       map[string]svgNode
	textFace    *TextFace
	decodeImage func(path string, reqSize int) (*Buffer, error)
}

type TextFace struct {
	Width  int
	Height int
	Glyphs map[rune][]byte
}

type Options struct {
	TextFace    *TextFace
	DecodeImage func(path string, reqSize int) (*Buffer, error)
}

type svgMatrix struct {
	a, b, c, d, e, f float64
}

func svgIdentity() svgMatrix {
	return svgMatrix{a: 1, d: 1}
}

func (m svgMatrix) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

func svgMul(a, b svgMatrix) svgMatrix {
	return svgMatrix{
		a: a.a*b.a + a.c*b.b,
		b: a.b*b.a + a.d*b.b,
		c: a.a*b.c + a.c*b.d,
		d: a.b*b.c + a.d*b.d,
		e: a.a*b.e + a.c*b.f + a.e,
		f: a.b*b.e + a.d*b.f + a.f,
	}
}

// Decode decodes and rasterizes SVG using an in-tree parser/rasterizer.
func Decode(path string, reqSize int, opts Options) (*Buffer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read svg %q: %w", path, err)
	}
	var root svgNode
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse svg %q: %w", path, err)
	}
	if !strings.EqualFold(root.XMLName.Local, "svg") {
		return nil, fmt.Errorf("invalid svg root in %q", path)
	}

	vw, vh := svgViewBoxSize(root)
	w, h := inferSVGSize(vw, vh, reqSize)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	defs := &svgDefs{
		gradients:   map[string]svgGradient{},
		patterns:    map[string]svgNode{},
		classColors: map[string]string{},
		symbols:     map[string]svgNode{},
		nodesByID:   map[string]svgNode{},
		clipPaths:   map[string]svgNode{},
		masks:       map[string]svgNode{},
		textFace:    opts.TextFace,
		decodeImage: opts.DecodeImage,
	}
	svgCollectDefs(root, defs)

	dst := NewBuffer(w, h)
	fit := svgFitMatrix(vw, vh, w, h)
	style := svgStyle{
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
	svgRenderNode(dst, root, fit, style, defs, 0)
	return dst, nil
}

func inferSVGSize(vw, vh float64, reqSize int) (int, int) {
	if reqSize > 0 {
		return reqSize, reqSize
	}
	if vw <= 0 {
		vw = 256
	}
	if vh <= 0 {
		vh = 256
	}
	w := int(vw + 0.5)
	h := int(vh + 0.5)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if w > 2048 {
		w = 2048
	}
	if h > 2048 {
		h = 2048
	}
	return w, h
}

func svgViewBoxSize(root svgNode) (float64, float64) {
	if vb := svgAttr(root, "viewBox"); vb != "" {
		v := svgParseFloatList(vb)
		if len(v) == 4 && v[2] > 0 && v[3] > 0 {
			return v[2], v[3]
		}
	}
	w := svgParseLength(svgAttr(root, "width"))
	h := svgParseLength(svgAttr(root, "height"))
	if w > 0 && h > 0 {
		return w, h
	}
	return 256, 256
}

func svgFitMatrix(vw, vh float64, w, h int) svgMatrix {
	if vw <= 0 {
		vw = 1
	}
	if vh <= 0 {
		vh = 1
	}
	s := math.Min(float64(w)/vw, float64(h)/vh)
	tx := (float64(w) - vw*s) * 0.5
	ty := (float64(h) - vh*s) * 0.5
	return svgMatrix{a: s, d: s, e: tx, f: ty}
}

func svgCollectDefs(n svgNode, defs *svgDefs) {
	tag := strings.ToLower(n.XMLName.Local)
	if id := strings.TrimSpace(svgAttr(n, "id")); id != "" {
		defs.nodesByID[id] = n
	}
	switch tag {
	case "style":
		svgParseClassColors(n.Text, defs.classColors)
	case "lineargradient", "radialgradient":
		g := svgParseGradient(n)
		if g.id != "" {
			defs.gradients[g.id] = g
		}
	case "pattern":
		if id := strings.TrimSpace(svgAttr(n, "id")); id != "" {
			defs.patterns[id] = n
		}
	case "symbol":
		if id := strings.TrimSpace(svgAttr(n, "id")); id != "" {
			defs.symbols[id] = n
		}
	case "clippath":
		if id := strings.TrimSpace(svgAttr(n, "id")); id != "" {
			defs.clipPaths[id] = n
		}
	case "mask":
		if id := strings.TrimSpace(svgAttr(n, "id")); id != "" {
			defs.masks[id] = n
		}
	}
	for _, c := range n.Nodes {
		svgCollectDefs(c, defs)
	}
}

func svgRenderNode(dst *Buffer, n svgNode, ctm svgMatrix, inherited svgStyle, defs *svgDefs, depth int) {
	if depth > 20 {
		return
	}
	tag := strings.ToLower(n.XMLName.Local)
	if tag == "defs" || tag == "style" || tag == "lineargradient" || tag == "radialgradient" || tag == "stop" || tag == "pattern" || tag == "symbol" || tag == "clippath" || tag == "mask" {
		return
	}

	style := svgMergeStyle(inherited, n, defs.classColors)
	if style.displayNone {
		return
	}

	if t := svgAttr(n, "transform"); t != "" {
		ctm = svgMul(ctm, svgParseTransform(t))
	}
	clipID := svgParseURLID(svgAttr(n, "clip-path"))
	maskID := svgParseURLID(svgAttr(n, "mask"))
	if clipID != "" || maskID != "" {
		tmp := NewBuffer(dst.Width, dst.Height)
		nNoFx := svgNode{
			XMLName: n.XMLName,
			Attrs:   svgFilterAttrs(n.Attrs, "clip-path", "mask"),
			Text:    n.Text,
			Nodes:   n.Nodes,
		}
		svgRenderNode(tmp, nNoFx, ctm, inherited, defs, depth+1)
		targetBBox := svgBufferAlphaBBox(tmp)
		if clipID != "" {
			if cp, ok := defs.clipPaths[clipID]; ok {
				clipMask := svgBuildClipMask(dst.Width, dst.Height, cp, ctm, targetBBox, defs, depth+1)
				svgModulateBufferAlpha(tmp, clipMask)
			}
		}
		if maskID != "" {
			if m, ok := defs.masks[maskID]; ok {
				maskAlpha := svgBuildMaskAlpha(dst.Width, dst.Height, m, ctm, targetBBox, defs, depth+1)
				svgModulateBufferAlpha(tmp, maskAlpha)
			}
		}
		svgCompositeBuffer(dst, tmp)
		return
	}

	switch tag {
	case "path":
		d := svgAttr(n, "d")
		if d != "" {
			svgFillPath(dst, d, ctm, style, defs)
			svgStrokePath(dst, d, ctm, style, defs)
		}
	case "circle":
		cx := svgParseLength(svgAttr(n, "cx"))
		cy := svgParseLength(svgAttr(n, "cy"))
		r := svgParseLength(svgAttr(n, "r"))
		if r > 0 {
			svgFillCircle(dst, cx, cy, r, ctm, style, defs)
			svgStrokeCircle(dst, cx, cy, r, ctm, style, defs)
		}
	case "ellipse":
		cx := svgParseLength(svgAttr(n, "cx"))
		cy := svgParseLength(svgAttr(n, "cy"))
		rx := svgParseLength(svgAttr(n, "rx"))
		ry := svgParseLength(svgAttr(n, "ry"))
		if rx > 0 && ry > 0 {
			svgFillEllipse(dst, cx, cy, rx, ry, ctm, style, defs)
			svgStrokeEllipse(dst, cx, cy, rx, ry, ctm, style, defs)
		}
	case "rect":
		x := svgParseLength(svgAttr(n, "x"))
		y := svgParseLength(svgAttr(n, "y"))
		w := svgParseLength(svgAttr(n, "width"))
		h := svgParseLength(svgAttr(n, "height"))
		if w > 0 && h > 0 {
			svgFillRect(dst, x, y, w, h, ctm, style, defs)
			svgStrokeRect(dst, x, y, w, h, ctm, style, defs)
		}
	case "polygon":
		pts := svgParsePoints(svgAttr(n, "points"))
		if len(pts) >= 3 {
			svgFillPoints(dst, pts, true, ctm, style, defs)
			svgStrokePoints(dst, pts, true, ctm, style, defs)
		}
	case "polyline":
		pts := svgParsePoints(svgAttr(n, "points"))
		if len(pts) >= 2 {
			svgFillPoints(dst, pts, false, ctm, style, defs)
			svgStrokePoints(dst, pts, false, ctm, style, defs)
		}
	case "line":
		x1 := svgParseLength(svgAttr(n, "x1"))
		y1 := svgParseLength(svgAttr(n, "y1"))
		x2 := svgParseLength(svgAttr(n, "x2"))
		y2 := svgParseLength(svgAttr(n, "y2"))
		svgFillPoints(dst, [][2]float64{{x1, y1}, {x2, y2}}, false, ctm, style, defs)
		svgStrokePoints(dst, [][2]float64{{x1, y1}, {x2, y2}}, false, ctm, style, defs)
	case "text":
		svgRenderText(dst, n, ctm, style, defs)
	case "image":
		svgRenderImage(dst, n, ctm, style, defs)
	case "use":
		href := strings.TrimSpace(svgAttr(n, "href"))
		if href == "" {
			// xlink:href ends up with local name "href" in many XML parsers.
			href = strings.TrimSpace(svgAttr(n, "xlink:href"))
		}
		if strings.HasPrefix(href, "#") {
			refID := strings.TrimPrefix(href, "#")
			ref, ok := defs.symbols[refID]
			if !ok {
				ref, ok = defs.nodesByID[refID]
			}
			if ok {
				x := svgParseLength(svgAttr(n, "x"))
				y := svgParseLength(svgAttr(n, "y"))
				useCTM := svgMul(ctm, svgMatrix{a: 1, d: 1, e: x, f: y})
				svgRenderNode(dst, ref, useCTM, style, defs, depth+1)
			}
		}
	}

	for _, c := range n.Nodes {
		svgRenderNode(dst, c, ctm, style, defs, depth+1)
	}
}

func svgFillPath(dst *Buffer, d string, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	paths := svgPathToStrokePolylines(d, ctm)
	bbox := svgBBoxFromPolylines(paths)
	if strings.EqualFold(style.fillRule, "evenodd") {
		svgFillPathEvenOdd(dst, paths, style, defs, ctm, bbox)
		return
	}
	zr := vector.NewRasterizer(dst.Width, dst.Height)
	if !svgPathToRasterizer(zr, d, ctm) {
		return
	}
	svgPaintRaster(dst, zr, style, defs, ctm, bbox)
}

func svgFillPathEvenOdd(dst *Buffer, paths []svgPolyline, style svgStyle, defs *svgDefs, ctm svgMatrix, bbox svgBBox) {
	paint, ok := svgResolveFillPaint(style, defs, ctm, bbox)
	if !ok {
		return
	}
	if len(paths) == 0 {
		return
	}
	// Fill semantics implicitly close subpaths.
	for i := range paths {
		paths[i].closed = true
	}
	mask := svgBuildEvenOddMask(paths, dst.Width, dst.Height, bbox)
	svgBlendMaskRegion(dst, paint, mask, bbox)
}

func svgFillCircle(dst *Buffer, cx, cy, r float64, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	zr := vector.NewRasterizer(dst.Width, dst.Height)
	pts := make([][2]float64, 0, 65)
	const segs = 64
	for i := 0; i <= segs; i++ {
		a := (2.0 * math.Pi * float64(i)) / float64(segs)
		x := cx + r*math.Cos(a)
		y := cy + r*math.Sin(a)
		x, y = ctm.apply(x, y)
		pts = append(pts, [2]float64{x, y})
		if i == 0 {
			zr.MoveTo(float32(x), float32(y))
		} else {
			zr.LineTo(float32(x), float32(y))
		}
	}
	zr.ClosePath()
	svgPaintRaster(dst, zr, style, defs, ctm, svgBBoxFromPoints(pts))
}

func svgFillRect(dst *Buffer, x, y, w, h float64, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	zr := vector.NewRasterizer(dst.Width, dst.Height)
	x0, y0 := ctm.apply(x, y)
	x1, y1 := ctm.apply(x+w, y)
	x2, y2 := ctm.apply(x+w, y+h)
	x3, y3 := ctm.apply(x, y+h)
	zr.MoveTo(float32(x0), float32(y0))
	zr.LineTo(float32(x1), float32(y1))
	zr.LineTo(float32(x2), float32(y2))
	zr.LineTo(float32(x3), float32(y3))
	zr.ClosePath()
	svgPaintRaster(dst, zr, style, defs, ctm, svgBBoxFromPoints([][2]float64{{x0, y0}, {x1, y1}, {x2, y2}, {x3, y3}}))
}

func svgFillEllipse(dst *Buffer, cx, cy, rx, ry float64, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	zr := vector.NewRasterizer(dst.Width, dst.Height)
	pts := make([][2]float64, 0, 73)
	const segs = 72
	for i := 0; i <= segs; i++ {
		a := (2.0 * math.Pi * float64(i)) / float64(segs)
		x := cx + rx*math.Cos(a)
		y := cy + ry*math.Sin(a)
		x, y = ctm.apply(x, y)
		pts = append(pts, [2]float64{x, y})
		if i == 0 {
			zr.MoveTo(float32(x), float32(y))
		} else {
			zr.LineTo(float32(x), float32(y))
		}
	}
	zr.ClosePath()
	svgPaintRaster(dst, zr, style, defs, ctm, svgBBoxFromPoints(pts))
}

func svgFillPoints(dst *Buffer, pts [][2]float64, closePath bool, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	if len(pts) == 0 {
		return
	}
	zr := vector.NewRasterizer(dst.Width, dst.Height)
	x, y := ctm.apply(pts[0][0], pts[0][1])
	zr.MoveTo(float32(x), float32(y))
	tpts := make([][2]float64, 0, len(pts))
	tpts = append(tpts, [2]float64{x, y})
	for i := 1; i < len(pts); i++ {
		x, y = ctm.apply(pts[i][0], pts[i][1])
		zr.LineTo(float32(x), float32(y))
		tpts = append(tpts, [2]float64{x, y})
	}
	if closePath {
		zr.ClosePath()
	}
	svgPaintRaster(dst, zr, style, defs, ctm, svgBBoxFromPoints(tpts))
}

func svgStrokePath(dst *Buffer, d string, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	paths := svgPathToStrokePolylines(d, ctm)
	bbox := svgBBoxFromPolylines(paths)
	p, ok := svgResolveStrokePaint(style, defs, ctm, bbox)
	if !ok || style.strokeWidth <= 0 {
		return
	}
	svgRenderStrokePolylines(dst, paths, p, style)
}

func svgStrokeCircle(dst *Buffer, cx, cy, r float64, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	svgStrokeEllipse(dst, cx, cy, r, r, ctm, style, defs)
}

func svgStrokeEllipse(dst *Buffer, cx, cy, rx, ry float64, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	const segs = 96
	pts := make([][2]float64, 0, segs)
	for i := 0; i < segs; i++ {
		a := (2.0 * math.Pi * float64(i)) / float64(segs)
		x := cx + rx*math.Cos(a)
		y := cy + ry*math.Sin(a)
		x, y = ctm.apply(x, y)
		pts = append(pts, [2]float64{x, y})
	}
	p, ok := svgResolveStrokePaint(style, defs, ctm, svgBBoxFromPoints(pts))
	if !ok || style.strokeWidth <= 0 {
		return
	}
	svgRenderStrokePolylines(dst, []svgPolyline{{pts: pts, closed: true}}, p, style)
}

func svgStrokeRect(dst *Buffer, x, y, w, h float64, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	pts := [][2]float64{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
	svgStrokePoints(dst, pts, true, ctm, style, defs)
}

func svgStrokePoints(dst *Buffer, pts [][2]float64, closePath bool, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	tp := make([][2]float64, 0, len(pts))
	for _, q := range pts {
		x, y := ctm.apply(q[0], q[1])
		tp = append(tp, [2]float64{x, y})
	}
	p, ok := svgResolveStrokePaint(style, defs, ctm, svgBBoxFromPoints(tp))
	if !ok || style.strokeWidth <= 0 || len(pts) < 2 {
		return
	}
	svgRenderStrokePolylines(dst, []svgPolyline{{pts: tp, closed: closePath}}, p, style)
}

func svgRenderText(dst *Buffer, n svgNode, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	font := defs.textFace
	if font == nil {
		return
	}
	x := svgParseLength(svgAttr(n, "x"))
	y := svgParseLength(svgAttr(n, "y"))
	anchor := strings.ToLower(strings.TrimSpace(svgAttr(n, "text-anchor")))
	if anchor == "" {
		anchor = "start"
	}

	chunks := svgCollectTextChunks(n, x, y, anchor)
	if len(chunks) == 0 {
		return
	}
	mask := image.NewAlpha(image.Rect(0, 0, dst.Width, dst.Height))
	bbox := svgBBox{}

	cursorX := x
	cursorY := y
	for _, ch := range chunks {
		if ch.hasAbs {
			cursorX = ch.x
			cursorY = ch.y
		}
		cursorX += ch.dx
		cursorY += ch.dy
		txt := ch.text
		if txt == "" {
			continue
		}
		textW := float64(len([]rune(txt)) * font.Width)
		startX := cursorX
		switch ch.anchor {
		case "middle":
			startX -= textW / 2.0
		case "end":
			startX -= textW
		}
		topY := cursorY - float64(font.Height)
		svgRasterTextMask(mask, font, txt, startX, topY, ctm, &bbox)
		cursorX += textW
	}

	if !bbox.valid {
		return
	}
	paint, ok := svgResolveFillPaint(style, defs, ctm, bbox)
	if !ok {
		return
	}
	svgBlendMask(dst, paint, mask)
}

func svgCollectTextChunks(n svgNode, x, y float64, anchor string) []svgTextChunk {
	out := make([]svgTextChunk, 0, 8)
	baseText := svgNormalizeTextContent(n.Text)
	if baseText != "" {
		out = append(out, svgTextChunk{text: baseText, x: x, y: y, hasAbs: true, anchor: anchor})
	}
	for _, c := range n.Nodes {
		if !strings.EqualFold(c.XMLName.Local, "tspan") {
			continue
		}
		tc := svgTextChunk{
			text:   svgNormalizeTextContent(c.Text),
			x:      x,
			y:      y,
			hasAbs: false,
			anchor: anchor,
		}
		if xv := strings.TrimSpace(svgAttr(c, "x")); xv != "" {
			tc.x = svgParseLength(xv)
			tc.hasAbs = true
		}
		if yv := strings.TrimSpace(svgAttr(c, "y")); yv != "" {
			tc.y = svgParseLength(yv)
			tc.hasAbs = true
		}
		tc.dx = svgParseLength(svgAttr(c, "dx"))
		tc.dy = svgParseLength(svgAttr(c, "dy"))
		if a := strings.TrimSpace(svgAttr(c, "text-anchor")); a != "" {
			tc.anchor = strings.ToLower(a)
		}
		if tc.text != "" {
			out = append(out, tc)
		}
	}
	return out
}

func svgNormalizeTextContent(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

func svgRasterTextMask(mask *image.Alpha, font *TextFace, text string, x, y float64, ctm svgMatrix, bbox *svgBBox) {
	if mask == nil || font == nil || text == "" {
		return
	}
	cx := x
	for _, r := range text {
		if r == '\n' {
			cx = x
			y += float64(font.Height)
			continue
		}
		glyph, ok := font.Glyphs[r]
		if !ok {
			glyph = font.Glyphs['?']
			if glyph == nil {
				cx += float64(font.Width)
				continue
			}
		}
		for row := 0; row < font.Height && row < len(glyph); row++ {
			bits := glyph[row]
			for col := 0; col < font.Width; col++ {
				if bits&(0x80>>col) == 0 {
					continue
				}
				sx := cx + float64(col)
				sy := y + float64(row)
				tx, ty := ctm.apply(sx, sy)
				ix := int(tx + 0.5)
				iy := int(ty + 0.5)
				if ix < 0 || iy < 0 || ix >= mask.Rect.Dx() || iy >= mask.Rect.Dy() {
					continue
				}
				mask.SetAlpha(ix, iy, color.Alpha{A: 255})
				fx := float64(ix)
				fy := float64(iy)
				if bbox != nil {
					if !bbox.valid {
						*bbox = svgBBox{minX: fx, minY: fy, maxX: fx + 1, maxY: fy + 1, valid: true}
					} else {
						if fx < bbox.minX {
							bbox.minX = fx
						}
						if fy < bbox.minY {
							bbox.minY = fy
						}
						if fx+1 > bbox.maxX {
							bbox.maxX = fx + 1
						}
						if fy+1 > bbox.maxY {
							bbox.maxY = fy + 1
						}
					}
				}
			}
		}
		cx += float64(font.Width)
	}
}

func svgRenderImage(dst *Buffer, n svgNode, ctm svgMatrix, style svgStyle, defs *svgDefs) {
	href := strings.TrimSpace(svgAttr(n, "href"))
	if href == "" {
		href = strings.TrimSpace(svgAttr(n, "xlink:href"))
	}
	if href == "" || strings.HasPrefix(strings.ToLower(href), "data:") {
		return
	}
	if defs == nil || defs.decodeImage == nil {
		return
	}
	src, err := defs.decodeImage(href, 0)
	if err != nil || src == nil || src.Width <= 0 || src.Height <= 0 {
		return
	}
	x := svgParseLength(svgAttr(n, "x"))
	y := svgParseLength(svgAttr(n, "y"))
	w := svgParseLength(svgAttr(n, "width"))
	h := svgParseLength(svgAttr(n, "height"))
	if w <= 0 {
		w = float64(src.Width)
	}
	if h <= 0 {
		h = float64(src.Height)
	}
	p0x, p0y := ctm.apply(x, y)
	p1x, p1y := ctm.apply(x+w, y)
	p2x, p2y := ctm.apply(x+w, y+h)
	p3x, p3y := ctm.apply(x, y+h)
	bb := svgBBoxFromPoints([][2]float64{{p0x, p0y}, {p1x, p1y}, {p2x, p2y}, {p3x, p3y}})
	x0, y0, x1, y1 := svgBBoxToIntRect(bb, dst.Width, dst.Height, 0)
	if x1 <= x0 || y1 <= y0 {
		return
	}
	dstRect := Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	srcRect := Rect{X: 0, Y: 0, W: src.Width, H: src.Height}

	// Draw image through a temporary buffer so opacity can be applied uniformly.
	tmp := NewBuffer(dst.Width, dst.Height)
	tmp.BlitScaled(src, srcRect, dstRect)
	opacity := style.opacity
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			c := tmp.GetPixel(px, py)
			if c.A == 0 {
				continue
			}
			if opacity < 0.999 {
				c.A = uint8(float64(c.A)*opacity + 0.5)
				if c.A == 0 {
					continue
				}
			}
			bg := dst.GetPixel(px, py)
			dst.SetPixel(px, py, c.Blend(bg))
		}
	}
}

func svgRenderStrokePolylines(dst *Buffer, lines []svgPolyline, paint svgPaint, style svgStyle) {
	r := style.strokeWidth * 0.5
	if r <= 0 {
		return
	}
	capStyle := strings.ToLower(strings.TrimSpace(style.lineCap))
	if capStyle == "" {
		capStyle = "butt"
	}
	joinStyle := strings.ToLower(strings.TrimSpace(style.lineJoin))
	if joinStyle == "" {
		joinStyle = "miter"
	}
	dash := svgNormalizeDash(style.dashArray)
	dashOffset := style.dashOffset

	for _, pl := range lines {
		n := len(pl.pts)
		if n < 2 {
			continue
		}
		if len(dash) == 0 {
			for i := 0; i < n-1; i++ {
				a := pl.pts[i]
				b := pl.pts[i+1]
				svgStrokeSegment(dst, paint, a[0], a[1], b[0], b[1], r, capStyle)
			}
			if pl.closed {
				a := pl.pts[n-1]
				b := pl.pts[0]
				svgStrokeSegment(dst, paint, a[0], a[1], b[0], b[1], r, capStyle)
			}
		} else {
			svgStrokeDashedPolyline(dst, paint, pl, r, capStyle, dash, dashOffset)
		}

		if !pl.closed && capStyle == "round" {
			svgStrokeDisc(dst, paint, pl.pts[0][0], pl.pts[0][1], r)
			svgStrokeDisc(dst, paint, pl.pts[n-1][0], pl.pts[n-1][1], r)
		}

		// Join decorations are only applied for solid strokes.
		if len(dash) == 0 {
			if joinStyle == "round" {
				limit := n
				if !pl.closed {
					limit = n - 1
				}
				for i := 1; i < limit; i++ {
					p := pl.pts[i]
					svgStrokeDisc(dst, paint, p[0], p[1], r)
				}
				if pl.closed {
					p := pl.pts[0]
					svgStrokeDisc(dst, paint, p[0], p[1], r)
				}
			} else if joinStyle == "bevel" || joinStyle == "miter" {
				if pl.closed {
					for i := 0; i < n; i++ {
						pPrev := pl.pts[(i-1+n)%n]
						pCur := pl.pts[i]
						pNext := pl.pts[(i+1)%n]
						svgStrokeJoin(dst, paint, pPrev, pCur, pNext, r, joinStyle, style.miterLimit)
					}
				} else {
					for i := 1; i < n-1; i++ {
						pPrev := pl.pts[i-1]
						pCur := pl.pts[i]
						pNext := pl.pts[i+1]
						svgStrokeJoin(dst, paint, pPrev, pCur, pNext, r, joinStyle, style.miterLimit)
					}
				}
			}
		}
	}
}

func svgStrokeJoin(dst *Buffer, paint svgPaint, prev, cur, next [2]float64, r float64, joinStyle string, miterLimit float64) {
	if r <= 0 {
		return
	}
	upx, upy, okPrev := svgNormVec(cur[0]-prev[0], cur[1]-prev[1])
	unx, uny, okNext := svgNormVec(next[0]-cur[0], next[1]-cur[1])
	if !okPrev || !okNext {
		return
	}
	nplx, nply := -upy, upx
	nnlx, nnly := -uny, unx

	// Handle both sides of the stroke band.
	for _, side := range []float64{1, -1} {
		p1 := [2]float64{cur[0] + nplx*side*r, cur[1] + nply*side*r}
		p2 := [2]float64{cur[0] + nnlx*side*r, cur[1] + nnly*side*r}

		if joinStyle == "miter" {
			if ix, iy, ok := svgLineIntersection(p1[0], p1[1], upx, upy, p2[0], p2[1], unx, uny); ok {
				miterLen := math.Hypot(ix-cur[0], iy-cur[1]) / r
				if miterLen <= miterLimit {
					svgStrokeJoinTriangle(dst, paint, p1, [2]float64{ix, iy}, p2)
					continue
				}
			}
		}
		// Bevel fallback.
		svgStrokeJoinTriangle(dst, paint, p1, cur, p2)
	}
}

func svgLineIntersection(px, py, pdx, pdy, qx, qy, qdx, qdy float64) (float64, float64, bool) {
	den := pdx*qdy - pdy*qdx
	if math.Abs(den) < 1e-9 {
		return 0, 0, false
	}
	dx := qx - px
	dy := qy - py
	t := (dx*qdy - dy*qdx) / den
	return px + t*pdx, py + t*pdy, true
}

func svgNormVec(x, y float64) (float64, float64, bool) {
	l := math.Hypot(x, y)
	if l <= 1e-9 {
		return 0, 0, false
	}
	return x / l, y / l, true
}

func svgStrokeJoinTriangle(dst *Buffer, paint svgPaint, a, b, c [2]float64) {
	zr := vector.NewRasterizer(dst.Width, dst.Height)
	zr.MoveTo(float32(a[0]), float32(a[1]))
	zr.LineTo(float32(b[0]), float32(b[1]))
	zr.LineTo(float32(c[0]), float32(c[1]))
	zr.ClosePath()
	mask := image.NewAlpha(image.Rect(0, 0, dst.Width, dst.Height))
	zr.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	bb := svgBBoxFromPoints([][2]float64{a, b, c})
	svgBlendMaskRegion(dst, paint, mask, bb)
}

func svgNormalizeDash(d []float64) []float64 {
	if len(d) == 0 {
		return nil
	}
	out := make([]float64, 0, len(d))
	for _, x := range d {
		if x > 0 {
			out = append(out, x)
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

func svgStrokeDashedPolyline(dst *Buffer, paint svgPaint, pl svgPolyline, r float64, capStyle string, dash []float64, dashOffset float64) {
	if len(dash) == 0 || len(pl.pts) < 2 {
		return
	}
	patternLen := 0.0
	for _, d := range dash {
		patternLen += d
	}
	if patternLen <= 1e-9 {
		return
	}
	off := math.Mod(dashOffset, patternLen)
	if off < 0 {
		off += patternLen
	}
	di := 0
	for off > dash[di] && len(dash) > 0 {
		off -= dash[di]
		di = (di + 1) % len(dash)
	}
	dRemain := dash[di] - off

	segCount := len(pl.pts) - 1
	if pl.closed {
		segCount = len(pl.pts)
	}
	for i := 0; i < segCount; i++ {
		a := pl.pts[i]
		b := pl.pts[(i+1)%len(pl.pts)]
		x1, y1 := a[0], a[1]
		x2, y2 := b[0], b[1]
		dx := x2 - x1
		dy := y2 - y1
		segLen := math.Hypot(dx, dy)
		if segLen <= 1e-9 {
			continue
		}
		ux, uy := dx/segLen, dy/segLen
		pos := 0.0
		for pos < segLen-1e-9 {
			step := dRemain
			if step > segLen-pos {
				step = segLen - pos
			}
			if di%2 == 0 && step > 1e-9 {
				ax := x1 + ux*pos
				ay := y1 + uy*pos
				bx := x1 + ux*(pos+step)
				by := y1 + uy*(pos+step)
				svgStrokeSegment(dst, paint, ax, ay, bx, by, r, capStyle)
			}
			pos += step
			dRemain -= step
			if dRemain <= 1e-9 {
				di = (di + 1) % len(dash)
				dRemain = dash[di]
			}
		}
	}
}

func svgStrokeSegment(dst *Buffer, paint svgPaint, x1, y1, x2, y2, r float64, capStyle string) {
	if r <= 0 {
		return
	}
	dx := x2 - x1
	dy := y2 - y1
	segLen := math.Hypot(dx, dy)
	if segLen <= 1e-9 {
		svgStrokeDisc(dst, paint, x1, y1, r)
		return
	}
	ex1, ey1, ex2, ey2 := x1, y1, x2, y2
	if capStyle == "square" {
		ux, uy := dx/segLen, dy/segLen
		ex1, ey1 = x1-ux*r, y1-uy*r
		ex2, ey2 = x2+ux*r, y2+uy*r
		dx = ex2 - ex1
		dy = ey2 - ey1
		segLen = math.Hypot(dx, dy)
	}

	minX := int(math.Floor(math.Min(ex1, ex2) - r - 1))
	maxX := int(math.Ceil(math.Max(ex1, ex2) + r + 1))
	minY := int(math.Floor(math.Min(ey1, ey2) - r - 1))
	maxY := int(math.Ceil(math.Max(ey1, ey2) + r + 1))

	segLen2 := dx*dx + dy*dy
	if segLen2 <= 1e-9 {
		return
	}
	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			sx := float64(px) + 0.5
			sy := float64(py) + 0.5
			t := ((sx-ex1)*dx + (sy-ey1)*dy) / segLen2
			if capStyle == "butt" && (t < 0 || t > 1) {
				continue
			}
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			cx := ex1 + t*dx
			cy := ey1 + t*dy
			dist := math.Hypot(sx-cx, sy-cy)
			cov := r + 0.75 - dist
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			svgBlendPaintPixel(dst, paint, px, py, cov)
		}
	}
}

func svgStrokeDisc(dst *Buffer, paint svgPaint, cx, cy, r float64) {
	if r <= 0 {
		return
	}
	minX := int(math.Floor(cx - r - 1))
	maxX := int(math.Ceil(cx + r + 1))
	minY := int(math.Floor(cy - r - 1))
	maxY := int(math.Ceil(cy + r + 1))
	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			sx := float64(px) + 0.5
			sy := float64(py) + 0.5
			dist := math.Hypot(sx-cx, sy-cy)
			cov := r + 0.75 - dist
			if cov <= 0 {
				continue
			}
			if cov > 1 {
				cov = 1
			}
			svgBlendPaintPixel(dst, paint, px, py, cov)
		}
	}
}

func svgBlendPaintPixel(dst *Buffer, paint svgPaint, px, py int, cov float64) {
	if px < 0 || py < 0 || px >= dst.Width || py >= dst.Height || cov <= 0 {
		return
	}
	src := paint.sample(float64(px)+0.5, float64(py)+0.5)
	if src.A == 0 {
		return
	}
	if cov > 1 {
		cov = 1
	}
	src.A = uint8(float64(src.A)*cov + 0.5)
	if src.A == 0 {
		return
	}
	bg := dst.GetPixel(px, py)
	dst.SetPixel(px, py, src.Blend(bg))
}

func svgFilterAttrs(attrs []xml.Attr, dropKeys ...string) []xml.Attr {
	if len(attrs) == 0 {
		return nil
	}
	drop := map[string]struct{}{}
	for _, k := range dropKeys {
		if k == "" {
			continue
		}
		drop[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}
	out := make([]xml.Attr, 0, len(attrs))
	for _, a := range attrs {
		if _, ok := drop[strings.ToLower(strings.TrimSpace(a.Name.Local))]; ok {
			continue
		}
		out = append(out, a)
	}
	return out
}

func svgParseURLID(v string) string {
	s := strings.TrimSpace(v)
	ls := strings.ToLower(s)
	if strings.HasPrefix(ls, "url(#") && strings.HasSuffix(s, ")") && len(s) >= 6 {
		return s[len("url(#") : len(s)-1]
	}
	return ""
}

func svgClipNodeToRenderable(n svgNode, inheritedClipRule string) svgNode {
	out := n
	hasFillRule := false
	hasClipRule := false
	localClipRule := ""
	for _, a := range out.Attrs {
		k := strings.ToLower(strings.TrimSpace(a.Name.Local))
		if k == "fill-rule" {
			hasFillRule = true
		}
		if k == "clip-rule" {
			hasClipRule = true
			localClipRule = strings.TrimSpace(strings.ToLower(a.Value))
		}
	}
	if !hasFillRule {
		rule := inheritedClipRule
		if hasClipRule && localClipRule != "" {
			rule = localClipRule
		}
		if rule != "" {
			out.Attrs = append(out.Attrs, xml.Attr{Name: xml.Name{Local: "fill-rule"}, Value: rule})
		}
	}
	return out
}

func svgBufferAlphaBBox(buf *Buffer) svgBBox {
	if buf == nil || buf.Width <= 0 || buf.Height <= 0 {
		return svgBBox{}
	}
	b := svgBBox{}
	for y := 0; y < buf.Height; y++ {
		for x := 0; x < buf.Width; x++ {
			if buf.GetPixel(x, y).A == 0 {
				continue
			}
			fx := float64(x)
			fy := float64(y)
			if !b.valid {
				b = svgBBox{minX: fx, minY: fy, maxX: fx + 1, maxY: fy + 1, valid: true}
				continue
			}
			if fx < b.minX {
				b.minX = fx
			}
			if fy < b.minY {
				b.minY = fy
			}
			if fx+1 > b.maxX {
				b.maxX = fx + 1
			}
			if fy+1 > b.maxY {
				b.maxY = fy + 1
			}
		}
	}
	return b
}

func svgBuildClipMask(w, h int, cp svgNode, ctm svgMatrix, targetBBox svgBBox, defs *svgDefs, depth int) *image.Alpha {
	maskBuf := NewBuffer(w, h)
	cpRule := strings.TrimSpace(strings.ToLower(svgAttr(cp, "clip-rule")))
	if cpRule == "" {
		cpRule = "nonzero"
	}
	maskStyle := svgStyle{
		fill:        "#ffffff",
		fillOpacity: 1,
		fillRule:    cpRule,
		clipRule:    cpRule,
		stroke:      "none",
		strokeWidth: 0,
		strokeOp:    1,
		lineCap:     "butt",
		lineJoin:    "miter",
		miterLimit:  4,
		opacity:     1,
		color:       "#ffffff",
	}
	cpCTM := ctm
	if strings.EqualFold(strings.TrimSpace(svgAttr(cp, "clipPathUnits")), "objectboundingbox") {
		bx, by, bw, bh := svgBBoxOrNominal(targetBBox, ctm)
		cpCTM = svgMul(cpCTM, svgMatrix{a: bw, d: bh, e: bx, f: by})
	}
	if t := svgAttr(cp, "transform"); t != "" {
		cpCTM = svgMul(cpCTM, svgParseTransform(t))
	}
	for _, c := range cp.Nodes {
		svgRenderNode(maskBuf, svgClipNodeToRenderable(c, cpRule), cpCTM, maskStyle, defs, depth+1)
	}
	alpha := image.NewAlpha(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			alpha.SetAlpha(x, y, color.Alpha{A: maskBuf.GetPixel(x, y).A})
		}
	}
	return alpha
}

func svgBuildMaskAlpha(w, h int, m svgNode, ctm svgMatrix, targetBBox svgBBox, defs *svgDefs, depth int) *image.Alpha {
	maskBuf := NewBuffer(w, h)
	maskStyle := svgStyle{
		fill:        "#ffffff",
		fillOpacity: 1,
		fillRule:    "nonzero",
		clipRule:    "nonzero",
		stroke:      "none",
		strokeWidth: 0,
		strokeOp:    1,
		lineCap:     "butt",
		lineJoin:    "miter",
		miterLimit:  4,
		opacity:     1,
		color:       "#ffffff",
	}
	maskCTM := ctm
	bx, by, bw, bh := svgBBoxOrNominal(targetBBox, ctm)
	maskUnits := strings.ToLower(strings.TrimSpace(svgAttr(m, "maskUnits")))
	if maskUnits == "" {
		maskUnits = "objectboundingbox"
	}
	mx := svgParseLength(svgAttr(m, "x"))
	my := svgParseLength(svgAttr(m, "y"))
	mw := svgParseLength(svgAttr(m, "width"))
	mh := svgParseLength(svgAttr(m, "height"))
	if maskUnits == "objectboundingbox" {
		if mw == 0 {
			mw = 1
		}
		if mh == 0 {
			mh = 1
		}
		maskCTM = svgMul(maskCTM, svgMatrix{a: mw * bw, d: mh * bh, e: bx + mx*bw, f: by + my*bh})
	} else {
		if mw != 0 || mh != 0 || mx != 0 || my != 0 {
			sx := 1.0
			sy := 1.0
			if mw > 0 {
				sx = mw
			}
			if mh > 0 {
				sy = mh
			}
			maskCTM = svgMul(maskCTM, svgMatrix{a: sx, d: sy, e: mx, f: my})
		}
	}
	contentUnits := strings.ToLower(strings.TrimSpace(svgAttr(m, "maskContentUnits")))
	if contentUnits == "objectboundingbox" {
		maskCTM = svgMul(maskCTM, svgMatrix{a: bw, d: bh, e: bx, f: by})
	}
	if t := svgAttr(m, "transform"); t != "" {
		maskCTM = svgMul(maskCTM, svgParseTransform(t))
	}
	for _, c := range m.Nodes {
		svgRenderNode(maskBuf, c, maskCTM, maskStyle, defs, depth+1)
	}
	alpha := image.NewAlpha(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := maskBuf.GetPixel(x, y)
			// Basic luminance-alpha mask.
			l := (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000
			a := uint8((int(c.A)*l + 127) / 255)
			alpha.SetAlpha(x, y, color.Alpha{A: a})
		}
	}
	return alpha
}

func svgModulateBufferAlpha(buf *Buffer, mask *image.Alpha) {
	if buf == nil || mask == nil {
		return
	}
	w := buf.Width
	if mask.Rect.Dx() < w {
		w = mask.Rect.Dx()
	}
	h := buf.Height
	if mask.Rect.Dy() < h {
		h = mask.Rect.Dy()
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ma := mask.AlphaAt(x, y).A
			if ma == 255 {
				continue
			}
			c := buf.GetPixel(x, y)
			if c.A == 0 {
				continue
			}
			c.A = uint8((uint16(c.A)*uint16(ma) + 127) / 255)
			buf.SetPixel(x, y, c)
		}
	}
}

func svgCompositeBuffer(dst, src *Buffer) {
	if dst == nil || src == nil {
		return
	}
	w := dst.Width
	if src.Width < w {
		w = src.Width
	}
	h := dst.Height
	if src.Height < h {
		h = src.Height
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.GetPixel(x, y)
			if c.A == 0 {
				continue
			}
			bg := dst.GetPixel(x, y)
			dst.SetPixel(x, y, c.Blend(bg))
		}
	}
}

func svgCompositeMask(dst, src *Buffer, mask *image.Alpha) {
	if dst == nil || src == nil || mask == nil {
		return
	}
	w := dst.Width
	if src.Width < w {
		w = src.Width
	}
	if mask.Rect.Dx() < w {
		w = mask.Rect.Dx()
	}
	h := dst.Height
	if src.Height < h {
		h = src.Height
	}
	if mask.Rect.Dy() < h {
		h = mask.Rect.Dy()
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ma := mask.AlphaAt(x, y).A
			if ma == 0 {
				continue
			}
			c := src.GetPixel(x, y)
			if c.A == 0 {
				continue
			}
			c.A = uint8((uint16(c.A)*uint16(ma) + 127) / 255)
			if c.A == 0 {
				continue
			}
			bg := dst.GetPixel(x, y)
			dst.SetPixel(x, y, c.Blend(bg))
		}
	}
}

func svgBuildEvenOddMask(paths []svgPolyline, w, h int, bbox svgBBox) *image.Alpha {
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	x0, y0, x1, y1 := svgBBoxToIntRect(bbox, w, h, 1)
	if x1 <= x0 || y1 <= y0 {
		return mask
	}

	type edge struct {
		x1, y1 float64
		x2, y2 float64
	}
	edges := make([]edge, 0, 64)
	for _, pl := range paths {
		n := len(pl.pts)
		if n < 2 {
			continue
		}
		limit := n - 1
		if pl.closed {
			limit = n
		}
		for i := 0; i < limit; i++ {
			a := pl.pts[i]
			b := pl.pts[(i+1)%n]
			if a[1] == b[1] {
				continue
			}
			edges = append(edges, edge{x1: a[0], y1: a[1], x2: b[0], y2: b[1]})
		}
	}
	if len(edges) == 0 {
		return mask
	}

	rowW := x1 - x0
	rowH := y1 - y0
	coverage := make([]uint8, rowW*rowH)
	sampleX := [2]float64{0.25, 0.75}
	sampleY := [2]float64{0.25, 0.75}
	xs := make([]float64, 0, len(edges))

	for _, oy := range sampleY {
		for y := y0; y < y1; y++ {
			py := float64(y) + oy
			xs = xs[:0]
			for _, e := range edges {
				if (e.y1 > py) == (e.y2 > py) {
					continue
				}
				xi := e.x1 + (py-e.y1)*(e.x2-e.x1)/(e.y2-e.y1)
				xs = append(xs, xi)
			}
			if len(xs) < 2 {
				continue
			}
			sort.Float64s(xs)
			base := (y - y0) * rowW

			for _, ox := range sampleX {
				for i := 0; i+1 < len(xs); i += 2 {
					xa := xs[i]
					xb := xs[i+1]
					if xb <= xa {
						continue
					}
					start := int(math.Ceil(xa - ox))
					end := int(math.Ceil(xb - ox))
					if start < x0 {
						start = x0
					}
					if end > x1 {
						end = x1
					}
					if start >= end {
						continue
					}
					for x := start; x < end; x++ {
						coverage[base+x-x0]++
					}
				}
			}
		}
	}

	for y := y0; y < y1; y++ {
		base := (y - y0) * rowW
		rowPix := y*mask.Stride + x0
		for x := 0; x < rowW; x++ {
			n := coverage[base+x]
			if n == 0 {
				continue
			}
			mask.Pix[rowPix+x] = uint8((uint16(n)*255 + 2) / 4)
		}
	}

	return mask
}

func svgPointInEvenOdd(paths []svgPolyline, x, y float64) bool {
	crossings := 0
	for _, pl := range paths {
		n := len(pl.pts)
		if n < 2 {
			continue
		}
		limit := n - 1
		if pl.closed {
			limit = n
		}
		for i := 0; i < limit; i++ {
			a := pl.pts[i]
			b := pl.pts[(i+1)%n]
			x1, y1 := a[0], a[1]
			x2, y2 := b[0], b[1]
			if y1 == y2 {
				continue
			}
			if (y1 > y) == (y2 > y) {
				continue
			}
			xi := x1 + (y-y1)*(x2-x1)/(y2-y1)
			if xi > x {
				crossings++
			}
		}
	}
	return crossings%2 == 1
}

func svgPaintRaster(dst *Buffer, zr *vector.Rasterizer, style svgStyle, defs *svgDefs, ctm svgMatrix, bbox svgBBox) {
	paint, ok := svgResolveFillPaint(style, defs, ctm, bbox)
	if !ok {
		return
	}
	mask := image.NewAlpha(image.Rect(0, 0, dst.Width, dst.Height))
	zr.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	svgBlendMaskRegion(dst, paint, mask, bbox)
}

func svgBlendMask(dst *Buffer, paint svgPaint, mask *image.Alpha) {
	if dst == nil || paint == nil || mask == nil {
		return
	}
	svgBlendMaskRegion(dst, paint, mask, svgBBox{})
}

func svgBlendMaskRegion(dst *Buffer, paint svgPaint, mask *image.Alpha, bbox svgBBox) {
	if dst == nil || paint == nil || mask == nil {
		return
	}
	w := dst.Width
	h := dst.Height
	if mask.Rect.Dx() < w {
		w = mask.Rect.Dx()
	}
	if mask.Rect.Dy() < h {
		h = mask.Rect.Dy()
	}
	x0, y0, x1, y1 := svgBBoxToIntRect(bbox, w, h, 2)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			ma := mask.AlphaAt(x, y).A
			if ma == 0 {
				continue
			}
			src := paint.sample(float64(x)+0.5, float64(y)+0.5)
			if src.A == 0 {
				continue
			}
			src.A = uint8((uint16(src.A)*uint16(ma) + 127) / 255)
			if src.A == 0 {
				continue
			}
			bg := dst.GetPixel(x, y)
			dst.SetPixel(x, y, src.Blend(bg))
		}
	}
}

func svgBBoxToIntRect(b svgBBox, w, h, pad int) (x0, y0, x1, y1 int) {
	if !b.valid {
		return 0, 0, w, h
	}
	x0 = int(math.Floor(b.minX)) - pad
	y0 = int(math.Floor(b.minY)) - pad
	x1 = int(math.Ceil(b.maxX)) + pad
	y1 = int(math.Ceil(b.maxY)) + pad
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y1 > h {
		y1 = h
	}
	if x1 < x0 {
		x1 = x0
	}
	if y1 < y0 {
		y1 = y0
	}
	return
}
