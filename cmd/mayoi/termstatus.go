package main

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"mayoi/locales"
)

// A terminal whose shell runs in herdr-core, or waits for one, says so:
// a dot at the corner of its icon, in its tab and its chip of the panel,
// and a pill over the terminal while it waits.

// termState is what a terminal's dot says.
type termState int

const (
	termHere     termState = iota // runs in Mayoi: no dot
	termKept                      // its shell runs in herdr-core
	termStarting                  // its shell starts in herdr-core
	termWaiting                   // waits for the connection or the machine that went
)

// termStatus is what a terminal's dot says about it.
func termStatus(s *session) termState {
	switch {
	case s == nil:
		return termHere
	case s.hcStarting:
		return termStarting
	case s.lost:
		return termWaiting
	case s.hc != "":
		return termKept
	}
	return termHere
}

// termWait is what the pill over a terminal says while it waits, "" when
// it doesn't.
func termWait(s *session) string {
	switch termStatus(s) {
	case termStarting:
		return locales.HcStarting()
	case termWaiting:
		if c := sessionConn(s); c != nil {
			return locales.TermWaiting(c.Host)
		}
		if machine, _, ok := strings.Cut(s.hc, "/"); ok {
			return locales.TermWaiting(machine)
		}
		return locales.TermWaiting("herdr-core")
	}
	return ""
}

// hcReadyFor is how long a terminal says its shell in herdr-core is
// ready, once it attached having started or waited.
const hcReadyFor = 1600 * time.Millisecond

// hcJustReady reports whether a terminal's shell in herdr-core attached
// a moment ago, having started or waited.
func hcJustReady(s *session) bool {
	return s != nil && termStatus(s) == termKept && time.Since(s.hcReady) < hcReadyFor
}

// termIcon is a terminal's icon of size on bg, the color under it, with
// the badge of its state at its lower right corner: a dot cut out of the
// icon by a ring of bg, steady while its shell runs in herdr-core,
// sending out rings while it starts or waits, and once as it gets there.
// The badge grows in and shrinks away as the state comes and goes, and
// its color follows the state.
func (a *app) termIcon(c *ui.Context, s *session, size float32, tint, bg ui.Color) {
	p := a.pal
	st := termStatus(s)
	box := ui.Box(c).Size(size, size).Shrink(0)
	switch st {
	case termKept:
		box.Tooltip(locales.HcRunsIn(s.hc)).Label(locales.HcRunsIn(s.hc))
	case termStarting, termWaiting:
		box.Tooltip(termWait(s)).Label(termWait(s))
	}
	box.Children(func() {
		ui.Icon(c, iconTerminal).FontSize(size).TextColor(tint)
		if st == termHere {
			return
		}
		col := p.addedFg
		switch st {
		case termStarting:
			col = p.accent
		case termWaiting:
			col = p.modifiedFg
		}
		const d, cut = float32(6), float32(1.5)
		badge := ui.Box(c.Key("dot")).Absolute().Right(-2-cut).Bottom(-1-cut).Size(d+2*cut, d+2*cut).Transition(dotPop)
		col = badge.AnimateColor("col", col, ui.Smooth)
		bg = badge.AnimateColor("bg", bg, ui.Animation{Duration: hoverFade.Duration})
		// The rings stay still for those who ask for less motion.
		calm := c.Preferences().ReduceMotion
		pulse := (st == termStarting || st == termWaiting) && !calm
		var ready time.Time
		if !calm && hcJustReady(s) {
			ready = s.hcReady
		}
		badge.Draw(func(pt *ui.Painter, r ui.Rect) { statusDot(pt, r, cut, col, bg, pulse, ready) })
	})
}

// statusDot paints a dot of col in r, inside a ring of bg cut wide. While
// pulse is set, rings swell from the dot and fade, as a beacon, at 30
// frames a second; a moment after ready, one ring does, quicker and at
// 60, as the shell gets there. The window paints them without building
// the view.
func statusDot(pt *ui.Painter, r ui.Rect, cut float32, col, bg ui.Color, pulse bool, ready time.Time) {
	dot := ui.Rect{X: r.X + cut, Y: r.Y + cut, W: r.W - 2*cut, H: r.H - 2*cut}
	ring := func(t, reach, alpha float32) {
		grow := dot.W * reach * easeOutCubic(t)
		o := ui.Rect{X: dot.X - grow, Y: dot.Y - grow, W: dot.W + 2*grow, H: dot.H + 2*grow}
		pt.Fill(o, col.Alpha(alpha*(1-t)), o.W/2)
	}
	switch since := pt.Now().Sub(ready); {
	case pulse:
		const period = 1400
		ring(float32(pt.Now().UnixMilli()%period)/period, 0.9, 0.45)
		pt.AnimationFrameRate(30)
	case !ready.IsZero() && since < hcBurst:
		ring(float32(since)/float32(hcBurst), 1.3, 0.6)
		pt.AnimationFrameRate(60)
	}
	pt.Fill(r, bg, r.W/2)
	pt.Fill(dot, col, dot.W/2)
}

// hcBurst is how long the ring takes that a terminal's badge sends out as
// its shell gets to herdr-core.
const hcBurst = 700 * time.Millisecond

func easeOutCubic(t float32) float32 {
	return 1 - float32(math.Pow(float64(1-t), 3))
}

// hcMenu adds to the menu of a terminal whose shell runs in herdr-core
// an item that copies its address, for hc attach elsewhere.
func (a *app) hcMenu(m *ui.Menu, s *session) {
	if s.hc == "" {
		return
	}
	if m.Item(locales.TermCopyHC()).Chosen() {
		addr := s.hc
		a.post(func() {
			a.services.WriteClipboard(addr)
			a.tell(note{message: locales.CopiedText(addr)})
		})
	}
}

// pillMotion is how the pill over a terminal comes, changes and goes: it
// drops in, its width follows what it says, and it rises away.
var pillMotion = ui.ElementTransition{Duration: 160 * time.Millisecond, Colors: true, Size: true, Enter: &ui.Motion{Y: -6}, Exit: &ui.Motion{Y: -4}}

// waitPill is the pill over a terminal that waits, at its upper right:
// a spinner in the color of its badge and what it waits for. Once its
// shell in herdr-core gets there, the spinner turns into a check and the
// pill says so for a moment, then rises away.
func (a *app) waitPill(c *ui.Context, s *session) {
	msg, ready := termWait(s), hcJustReady(s)
	if msg == "" && !ready {
		return
	}
	p := a.pal
	col := p.accent
	switch {
	case ready:
		msg, col = locales.HcReady(), p.addedFg
	case termStatus(s) == termWaiting:
		col = p.modifiedFg
	}
	box := ui.Row(c.Key("term-wait")).Absolute().Top(8).Right(14).Padding(4, 10, 4, 8).Gap(7).Radius(11).AlignItems(ui.Center).Clip().Label(msg)
	a.floating(c, box, false).Transition(pillMotion).Children(func() {
		ui.Box(c).Size(11, 11).Shrink(0).Children(func() {
			if ready {
				ui.Icon(c.Key("ok"), iconCheck).FontSize(11).TextColor(col).Transition(dotPop)
			} else {
				ui.Spinner(c.Key("spin")).Size(11, 11).TextColor(col).Transition(dotPop)
			}
		})
		ui.Text(c, msg).FontSize(11.5).TextColor(p.muted).SingleLine().MaxWidth(320).ContentTransition(ui.ContentTransition{})
	})
}
