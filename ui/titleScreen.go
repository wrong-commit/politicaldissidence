package ui

import (
	"fmt"
	"politicaldissidence/ui/panel"
	"time"

	"github.com/jroimartin/gocui"
)

const titleMinDuration = time.Second * 1

// titleScreenBlocking reports whether the startup title panel is showing.
func (ui *UI) titleScreenBlocking() bool {
	return ui.titleOnly || ui.currentModal == TITLE_PANEL
}

// openTitleScreen opens the centered title modal with embedded ASCII art.
func (ui *UI) openTitleScreen(g *gocui.Gui) error {
	art := effectiveTitleASCII()
	artW, artH := panel.TitleASCIIDims(art)
	maxX, maxY := g.Size()
	if maxX < 4 {
		maxX = 4
	}
	if maxY < 4 {
		maxY = 4
	}

	const margin = 2
	w := artW + margin*2
	h := artH + margin*2
	if w > maxX-2 {
		w = maxX - 2
	}
	if h > maxY-2 {
		h = maxY - 2
	}
	if w < 10 {
		w = 10
		if w > maxX-2 {
			w = maxX - 2
		}
	}
	if h < 3 {
		h = 3
		if h > maxY-2 {
			h = maxY - 2
		}
	}

	v, err := ui.openModal(TITLE_PANEL, w, h, false)
	if err != nil {
		return err
	}
	ui.gui.Cursor = false
	v.Editable = false
	v.Wrap = false
	v.Highlight = false
	v.Frame = false
	v.Title = ""
	v.FgColor = gocui.ColorRed

	cw, ch := v.Size()
	body := panel.DrawTitleASCII(art, cw, ch)
	v.Clear()
	fmt.Fprint(v, body)
	return nil
}

// dismissTitleScreen removes the title and reveals the main layout for the first time.
func (ui *UI) dismissTitleScreen(g *gocui.Gui) error {
	if _, err := ui.gui.View(TITLE_PANEL); err == nil {
		_ = ui.gui.DeleteView(TITLE_PANEL)
	} else if err != gocui.ErrUnknownView {
		return err
	}
	ui.currentModal = ""
	ui.titleOnly = false
	ui.gui.Cursor = true

	ui.started = true
	_ = ui.log(ui.startupLog, false)

	if err := ui.Layout(g); err != nil {
		return err
	}
	// Force panel materialization for the MP selected during title-only Load.
	ui.state.currentIndex = -1
	if err := ui.selectMp(0); err != nil {
		return err
	}
	if _, err := ui.initPanelView(LIST_PANEL); err != nil && err != gocui.ErrUnknownView {
		return err
	}
	return ui.activatePanelView(ui.currentView)
}

// runStartupAfterInit performs Load, log flush, CSV config, and background kicks.
// Used when the title screen is skipped.
func (ui *UI) runStartupAfterInit() {
	if err := ui.Load(); err != nil {
		_ = ui.log(fmt.Sprintf("Could not load MPs <%s>", err.Error()), true)
	}
	ui.started = true
	_ = ui.log(ui.startupLog, false)
	ui.loadCsvRefreshConfig()
	ui.armCsvRefreshTicker()
	go ui.startBackgroundWhois(false)
	go ui.startBackgroundDns(false)
	go ui.startBackgroundHttps(false)
	go ui.startBackgroundRegistrar(false)
	ui.armPeriodicDomainChecksTicker()
}

// startupWithTitle opens the title panel alone, runs startup while it is visible,
// holds at least titleMinDuration, then dismisses the title and shows the main UI.
func (ui *UI) startupWithTitle() {
	ui.titleOnly = true

	opened := make(chan error, 1)
	ui.gui.Update(func(g *gocui.Gui) error {
		err := ui.openTitleScreen(g)
		opened <- err
		return err
	})
	if err := <-opened; err != nil {
		ui.titleOnly = false
		// Fall back to normal startup without a title panel.
		ui.gui.Update(func(*gocui.Gui) error {
			ui.runStartupAfterInit()
			return nil
		})
		return
	}
	shownAt := time.Now()

	loaded := make(chan struct{})
	ui.gui.Update(func(*gocui.Gui) error {
		defer close(loaded)
		if err := ui.Load(); err != nil {
			_ = ui.log(fmt.Sprintf("Could not load MPs <%s>", err.Error()), true)
		}
		ui.loadCsvRefreshConfig()
		ui.armCsvRefreshTicker()
		go ui.startBackgroundWhois(false)
		go ui.startBackgroundDns(false)
		go ui.startBackgroundHttps(false)
		go ui.startBackgroundRegistrar(false)
		ui.armPeriodicDomainChecksTicker()
		return nil
	})
	<-loaded

	if rem := titleMinDuration - time.Since(shownAt); rem > 0 {
		time.Sleep(rem)
	}

	done := make(chan struct{})
	ui.gui.Update(func(g *gocui.Gui) error {
		defer close(done)
		return ui.dismissTitleScreen(g)
	})
	<-done
}
