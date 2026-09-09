package ui

import (
	"fmt"
	"time"

	"politicaldissidence/csvrefresh"
	"politicaldissidence/data"
	"politicaldissidence/ui/panel"

	"github.com/jroimartin/gocui"
)

func (ui *UI) csvLogger() csvrefresh.Logger {
	return csvrefresh.LogFn{
		OnInfo: func(msg string) {
			ui.csvLog(msg, false)
		},
		OnDebug: func(msg string) {
			ui.csvLog(msg, false)
		},
		OnError: func(msg string) {
			ui.csvLog(msg, true)
		},
	}
}

func (ui *UI) csvLog(msg string, isError bool) {
	if ui.gui == nil || !ui.started {
		if isError {
			_ = ui.log(msg, true)
			return
		}
		_ = ui.logPlain(msg)
		return
	}
	ui.gui.Update(func(g *gocui.Gui) error {
		if isError {
			return ui.log(msg, true)
		}
		return ui.logPlain(msg)
	})
}

// loadCsvRefreshConfig loads csv_refresh.json once. Missing/invalid → ERROR; Ctrl+L/ticker no-ops.
func (ui *UI) loadCsvRefreshConfig() {
	cfg, err := csvrefresh.LoadFile(csvrefresh.DefaultConfigPath)
	if err != nil {
		ui.csvConfig = nil
		_ = ui.logPlain(csvrefresh.FormatConfigError(err.Error()))
		return
	}
	ui.csvConfig = cfg
}

// armCsvRefreshTicker starts the hourly ticker after one full interval. Does not run on startup.
func (ui *UI) armCsvRefreshTicker() {
	if ui.csvConfig == nil {
		return
	}
	if !csvrefresh.TickerEnabled() {
		_ = ui.logPlain(csvrefresh.FormatTickerSkipped())
		return
	}
	interval := ui.csvConfig.IntervalDuration
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			ui.tryCsvRefresh()
		}
	}()
}

// tryCsvRefresh runs one CSV refresh (Ctrl+L or ticker). Never writes disk.
func (ui *UI) tryCsvRefresh() {
	deps := csvrefresh.Deps{
		Log:    ui.csvLogger(),
		Config: ui.csvConfig,
		Current: func() []data.MP {
			if ui.state.all == nil {
				return nil
			}
			return *ui.state.all
		},
		Apply: ui.applyCsvMerge,
	}
	if _, ok := ui.csvRunner.TryRun(deps); !ok {
		// already running — TryRun logged
		_ = ok
	}
}

func (ui *UI) applyCsvMerge(merged []data.MP) {
	apply := func() {
		mps := merged
		ui.state.all = &mps
		ui.applyFilter(ui.state.filter)
		ui.state.currentIndex = -1
		if v, ok := panelViews[LIST_PANEL]; ok {
			v.text = panel.DrawListMpPanel(ui.gui, ui.state.visible)
		}
		if _, err := ui.initPanelView(LIST_PANEL); err == nil {
			if v, err := ui.gui.View(LIST_PANEL); err == nil && v != nil {
				_ = setListCursor(v, 0)
			}
		}
		_ = ui.selectMp(0)
	}

	if ui.gui == nil || !ui.started {
		apply()
		return
	}
	done := make(chan struct{})
	ui.gui.Update(func(g *gocui.Gui) error {
		defer close(done)
		apply()
		return nil
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		ui.csvLog(fmt.Sprintf("ERROR CSV refresh: timed out applying merge"), true)
	}
}
