package main

import (
	"fmt"
	"html"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Charts are static SVG: GitHub renders them beside REPORT.md, and a diff
// between two snapshots shows what moved. Colors are the light and dark
// steps of the dataviz reference palette, one series each.
const chartStyle = `<style>
text{font-family:system-ui,-apple-system,"Segoe UI",sans-serif}
.bg{fill:#fcfcfb}.title{fill:#0b0b0b;font-size:15px;font-weight:600}.subtitle{fill:#52514e;font-size:12px}
.label{fill:#52514e;font-size:11px}.tick{fill:#898781;font-size:11px;font-variant-numeric:tabular-nums}
.grid{stroke:#e1e0d9;stroke-width:1}.axis{stroke:#c3c2b7;stroke-width:1}.ref{stroke:#52514e;stroke-width:1.5}
.zone{fill:#0b0b0b;fill-opacity:.05}.mark{fill:#2a78d6}.dot{fill:#2a78d6;stroke:#fcfcfb;stroke-width:2}.leader{stroke:#898781;stroke-width:1}
@media (prefers-color-scheme:dark){.bg{fill:#1a1a19}.title{fill:#fff}.subtitle,.label{fill:#c3c2b7}.grid{stroke:#2c2c2a}.axis{stroke:#383835}
.ref{stroke:#c3c2b7}.zone{fill:#fff;fill-opacity:.06}.mark{fill:#3987e5}.dot{fill:#3987e5;stroke:#1a1a19}}
</style>`

// charWidth over-estimates an 11px system-ui glyph so placed labels keep
// their distance.
const charWidth = 6.6

type canvas struct {
	strings.Builder
	width, height float64
}

func newCanvas(width, height float64, title string, subtitles ...string) *canvas {
	c := &canvas{width: width, height: height}
	fmt.Fprintf(c, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s" role="img" aria-label="%s">`+"\n",
		num(width), num(height), num(width), num(height), html.EscapeString(title))
	c.WriteString(chartStyle + "\n")
	fmt.Fprintf(c, `<rect class="bg" width="%s" height="%s"/>`+"\n", num(width), num(height))
	c.text(24, 30, "title", "start", title)
	for i, subtitle := range subtitles {
		c.text(24, 50+float64(i)*16, "subtitle", "start", subtitle)
	}
	return c
}

func (c *canvas) text(x, y float64, class, anchor, s string) {
	fmt.Fprintf(c, `<text x="%s" y="%s" class="%s" text-anchor="%s">%s</text>`+"\n", num(x), num(y), class, anchor, html.EscapeString(s))
}

func (c *canvas) line(x1, y1, x2, y2 float64, class string) {
	fmt.Fprintf(c, `<line x1="%s" y1="%s" x2="%s" y2="%s" class="%s"/>`+"\n", num(x1), num(y1), num(x2), num(y2), class)
}

func (c *canvas) svg() string { return c.String() + "</svg>\n" }

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func tick(v float64) string {
	if v == math.Trunc(v) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

type rect struct{ x, y, w, h float64 }

func (r rect) overlaps(o rect) bool {
	return r.x < o.x+o.w && o.x < r.x+r.w && r.y < o.y+o.h && o.y < r.y+r.h
}

// mainSequenceChart plots every package at (instability, abstractness)
// against the line A + I = 1. Packages at the same point share one dot.
func mainSequenceChart(packages []pkg) string {
	const size, left, top = 520.0, 80.0, 92.0
	c := newCanvas(left+size+230, top+size+72, "Abstractness vs instability",
		"Each dot is a package. The diagonal A + I = 1 is the main sequence; D is the distance from it.",
		"Shaded corners are where D > 0.7: the zone of pain (bottom left) and the zone of uselessness (top right).")
	x := func(i float64) float64 { return left + i*size }
	y := func(a float64) float64 { return top + (1-a)*size }

	fmt.Fprintf(c, `<path class="zone" d="M%s %sH%sL%s %sZ"/>`+"\n", num(x(0)), num(y(0)), num(x(0.3)), num(x(0)), num(y(0.3)))
	fmt.Fprintf(c, `<path class="zone" d="M%s %sH%sL%s %sZ"/>`+"\n", num(x(1)), num(y(1)), num(x(0.7)), num(x(1)), num(y(0.7)))
	for _, v := range []float64{0, 0.25, 0.5, 0.75, 1} {
		c.line(x(v), top, x(v), top+size, "grid")
		c.line(left, y(v), left+size, y(v), "grid")
		c.text(x(v), top+size+18, "tick", "middle", tick(v))
		c.text(left-10, y(v)+4, "tick", "end", tick(v))
	}
	c.line(left, top+size, left+size, top+size, "axis")
	c.line(left, top, left, top+size, "axis")
	c.text(left+size/2, top+size+46, "label", "middle", "Instability I = Ce / (Ca + Ce)")
	fmt.Fprintf(c, `<text x="0" y="0" class="label" text-anchor="middle" transform="translate(%s %s) rotate(-90)">Abstractness A = interfaces / types</text>`+"\n",
		num(left-48), num(top+size/2))
	c.line(x(0), y(1), x(1), y(0), "ref")
	fmt.Fprintf(c, `<text x="0" y="0" class="label" transform="translate(%s %s) rotate(45)">main sequence</text>`+"\n", num(x(0.1)+6), num(y(0.9)-6))

	type group struct {
		i, a  float64
		names []string
		tip   []string
	}
	var groups []*group
	byPoint := map[string]*group{}
	for _, p := range packages {
		i, a := p.instability(), p.abstractness()
		key := fixed(i) + "," + fixed(a)
		g := byPoint[key]
		if g == nil {
			g = &group{i: i, a: a}
			byPoint[key] = g
			groups = append(groups, g)
		}
		g.names = append(g.names, short(p.name))
		g.tip = append(g.tip, fmt.Sprintf("%s: A %s, I %s, D %s", p.name, fixed(a), fixed(i), fixed(p.distance())))
	}

	var taken []rect
	for _, g := range groups {
		taken = append(taken, rect{x(g.i) - 7, y(g.a) - 7, 14, 14})
	}
	bounds := rect{left + 4, top - 8, c.width - left - 12, size + 8}
	// On the diagonal, x - left equals y - top.
	crossesDiagonal := func(r rect) bool {
		return (r.x-left)-(r.y+r.h-top) < 0 && (r.x+r.w-left)-(r.y-top) > 0
	}
	for _, g := range groups {
		px, py := x(g.i), y(g.a)
		label := strings.Join(g.names, ", ")
		w, h := float64(len(label))*charWidth, 13.0
		placed, leader := placeLabel(px, py, w, h, bounds, taken, crossesDiagonal)
		taken = append(taken, placed)
		if leader {
			end := placed.x - 2
			if placed.x+w < px {
				end = placed.x + w + 2
			}
			c.line(px, py, end, placed.y+h/2, "leader")
		}
		fmt.Fprintf(c, `<g><title>%s</title><circle cx="%s" cy="%s" r="5" class="dot"/>`, html.EscapeString(strings.Join(g.tip, "\n")), num(px), num(py))
		fmt.Fprintf(c, `<text x="%s" y="%s" class="label">%s</text></g>`+"\n", num(placed.x), num(placed.y+h-3), html.EscapeString(label))
	}
	return c.svg()
}

// placeLabel tries the spots next to a dot first, then spots further out
// joined to the dot by a leader line, and takes the first that overlaps no
// dot, placed label or blocked area.
func placeLabel(px, py, w, h float64, bounds rect, taken []rect, blocked func(rect) bool) (rect, bool) {
	candidates := []rect{
		{px + 9, py - h/2, w, h},
		{px + 6, py - h - 5, w, h},
		{px - 9 - w, py - h/2, w, h},
		{px + 6, py + 5, w, h},
		{px - 6 - w, py - h - 5, w, h},
		{px - 6 - w, py + 5, w, h},
	}
	near := len(candidates)
	for _, distance := range []float64{26, 44, 62, 80, 98} {
		for _, angle := range []float64{-45, -90, -20, 20, 45, 90, -135, 135} {
			rad := angle * math.Pi / 180
			ax, ay := px+distance*math.Cos(rad), py+distance*math.Sin(rad)
			if math.Cos(rad) < -0.1 {
				ax -= w
			}
			candidates = append(candidates, rect{ax, ay - h/2, w, h})
		}
	}
	for n, candidate := range candidates {
		inside := candidate.x >= bounds.x && candidate.y >= bounds.y &&
			candidate.x+candidate.w <= bounds.x+bounds.w && candidate.y+candidate.h <= bounds.y+bounds.h
		if inside && !blocked(candidate) && !slices.ContainsFunc(taken, candidate.overlaps) {
			return candidate, n >= near
		}
	}
	return candidates[0], false
}

type bar struct {
	label string
	value float64
	shown string
	tip   string
}

// barChart draws one horizontal bar per row from a shared zero baseline,
// with the value at the tip. reference, when positive, draws a labelled
// vertical rule at that value.
func barChart(title, subtitle string, bars []bar, domain float64, ticks []float64, reference float64, referenceLabel string) string {
	const row, thickness, plot, top = 24.0, 14.0, 440.0, 96.0
	longest := 0
	for _, b := range bars {
		longest = max(longest, len(b.label))
	}
	left := 24 + float64(longest)*charWidth + 12
	bottom := top + float64(len(bars))*row
	c := newCanvas(left+plot+64, bottom+44, title, subtitle)
	x := func(v float64) float64 { return left + v/domain*plot }

	for _, v := range ticks {
		c.line(x(v), top-6, x(v), bottom, "grid")
		c.text(x(v), bottom+18, "tick", "middle", tick(v))
	}
	if reference > 0 {
		c.line(x(reference), top-10, x(reference), bottom, "ref")
		c.text(x(reference)+4, top-14, "tick", "start", referenceLabel)
	}
	for n, b := range bars {
		y := top + float64(n)*row + (row-thickness)/2
		length := b.value / domain * plot
		fmt.Fprintf(c, `<g><title>%s</title>`, html.EscapeString(b.tip))
		if length > 0 {
			r := min(4, length/2, thickness/2)
			fmt.Fprintf(c, `<path class="mark" d="M%s %sH%sQ%s %s %s %sV%sQ%s %s %s %sH%sZ"/>`,
				num(left), num(y), num(left+length-r), num(left+length), num(y), num(left+length), num(y+r),
				num(y+thickness-r), num(left+length), num(y+thickness), num(left+length-r), num(y+thickness), num(left))
		}
		fmt.Fprintf(c, `<text x="%s" y="%s" class="tick">%s</text></g>`+"\n", num(left+length+6), num(y+thickness-3), html.EscapeString(b.shown))
		c.text(left-10, y+thickness-3, "label", "end", b.label)
	}
	c.line(left, top-6, left, bottom, "axis")
	return c.svg()
}

func distanceChart(packages []pkg) string {
	ranked := slices.Clone(packages)
	slices.SortStableFunc(ranked, func(a, b pkg) int {
		if c := compareDesc(a.distance(), b.distance()); c != 0 {
			return c
		}
		return strings.Compare(a.name, b.name)
	})
	bars := make([]bar, len(ranked))
	for n, p := range ranked {
		bars[n] = bar{short(p.name), p.distance(), fixed(p.distance()),
			fmt.Sprintf("%s: D %s (A %s, I %s)", p.name, fixed(p.distance()), fixed(p.abstractness()), fixed(p.instability()))}
	}
	return barChart("Distance from the main sequence", "D = |A + I − 1| per package, 0 on the line, 1 at a corner.",
		bars, 1, []float64{0, 0.25, 0.5, 0.75, 1}, 0, "")
}

// complexityChart ranks the most complex functions against McCabe's
// recommended limit of 10.
func complexityChart(functions []function, count int) string {
	top := functions[:min(count, len(functions))]
	highest := 0
	for _, f := range top {
		highest = max(highest, f.complexity)
	}
	domain := math.Max(10, math.Ceil(float64(highest)/10)*10)
	var ticks []float64
	for v := 0.0; v <= domain; v += domain / 5 {
		ticks = append(ticks, v)
	}
	bars := make([]bar, len(top))
	for n, f := range top {
		bars[n] = bar{short(f.pkg) + "." + f.name, float64(f.complexity), strconv.Itoa(f.complexity),
			fmt.Sprintf("%s.%s: %d at %s", f.pkg, f.name, f.complexity, f.position)}
	}
	return barChart(fmt.Sprintf("The %d most complex functions", len(top)), "Cyclomatic complexity: independent paths through the function.",
		bars, domain, ticks, 10, "McCabe limit 10")
}

func compareDesc(a, b float64) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	}
	return 0
}

// short drops the internal/ prefix every product package shares.
func short(name string) string { return strings.TrimPrefix(name, "internal/") }
