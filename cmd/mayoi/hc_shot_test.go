package main

import (
	"image"
	"os"
	"testing"
	"time"
)

// TestShotHC shoots the terminals of herdr-core in each state: kept,
// starting and waiting for their machine, in the panel and a tab.
func TestShotHC(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS is not set")
	}
	for _, dark := range []bool{true, false} {
		hs := newHarness(t, dark)
		a := hs.a
		useFakeHC(t, nil)
		kept := hcSession(t, a, "k7")
		kept.title = "zsh"
		starting := hcSession(t, a, "")
		starting.title, starting.hcStarting = "zsh", true
		waiting := hcSession(t, a, "vps/t2")
		waiting.title, waiting.lost = "vps", true
		ready := hcSession(t, a, "r9")
		ready.title, ready.hcReady = "ready", time.Now()
		waiting.term.Feed([]byte("$ tail -f deploy.log\r\nbuild ok\r\n" + lostNote("vps")))
		a.terminalToColumn(kept, a.column(), 0)
		a.panelTerm = waiting
		a.window.PanelOpen = true
		hs.tt.SetScale(2)
		hs.settle()
		for range 20 {
			hs.tt.Advance(16 * time.Millisecond)
			hs.tt.Frame()
		}
		name := "hc-light"
		if dark {
			name = "hc-dark"
		}
		hs.shot(name)
		hs.shotRect(name+"-tab", image.Rect(248, 38, 700, 76), 2)
		a.panelTerm = ready
		for range 20 {
			hs.tt.Advance(16 * time.Millisecond)
			hs.tt.Frame()
		}
		hs.shotRect(name+"-ready", image.Rect(248, 476, 1200, 560), 2)
		hs.shotRect(name+"-panel", image.Rect(248, 476, 1200, 560), 2)
	}
}
