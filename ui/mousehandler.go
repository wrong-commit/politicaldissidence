package ui

import (
	"fmt"
	"github.com/jroimartin/gocui"
)

func (ui *UI) ApplyMouseBindings(views []string) error {
	// Clicked into handler
	mouseIntoPanel := func(g *gocui.Gui, v *gocui.View) error {
		// ui.log("init clicked into ? ("+v.Name()+")", false)
		// Disable panel views selection with mouse in case the modal is activated
		if ui.currentModal == "" {
			// Clicked into a Panel
			// restore Curor position
			cx, cy := v.Cursor()
			line, err := v.Line(cy)
			if err != nil {
				ui.cursors.Restore(v)
				ui.setPanelView(v.Name())
			}
			if ui.currentView == ui.findViewByName(LIST_PANEL) && cy != ui.state.currentIndex {
				ui.log(fmt.Sprintf("Selected new MP %d", cy), false)
				// ui.currentMp = cy
				ui.selectMp(cy)
				// ui.afterChangingMp(v, ui.currentMp)
			}
			// if cursor X greater than line length reset to line end
			if cx > len(line) {
				ui.log("Random cursor reset block hit", false)
				// offset for emoji
				offset := 3
				v.SetCursor(offset, cy)
				// reset UI cursor to something sane ?
				ui.cursors.Set(v.Name(), offset, cy)
			}
			ui.currentView = ui.findViewByName(v.Name())
			ui.setPanelView(v.Name())
			view := panelViews[v.Name()]
			ui.gui.Cursor = view.cursor
		}
		return nil
	}

	//Setup click handlers on Panels
	for _, view := range clickableViews {
		if err := ui.gui.SetKeybinding(view, gocui.MouseLeft, gocui.ModNone, mouseIntoPanel); err != nil {
			return err
		}
		if err := ui.gui.SetKeybinding(view, gocui.MouseRelease, gocui.ModNone, mouseIntoPanel); err != nil {
			return err
		}
	}
	return nil
}
