package ui

import (
	"fmt"
	"politicaldissidence/data"
	"politicaldissidence/ui/panel"
	"runtime/debug"
	"strings"

	//"image"
	//"image/draw"
	//"log"
	"math"

	"time"
	// "tabwriter"
	"github.com/jroimartin/gocui"
)

type panelProperties struct {
	title    string
	text     string
	x1       float64
	y1       float64
	x2       float64
	y2       float64
	editable bool
	cursor   bool
	editor   *UI
}

// MinGUIWindow is the minimum window size
const MinGUIWindow = 600

const (
	// Panel constants
	WHOIS_PANEL     = "whois"
	LIST_PANEL      = "list"
	LOG_PANEL       = "log"
	DOMAIN_PANEL    = "domains"
	LIST_URLS_PANEL = "new_url_panel"
	// DIAGRAM_PANEL        = "diagram"
	// PROGRESS_PANEL       = "progress"
	HELP_PANEL       = "help"
	ADD_DOMAIN_PANEL = "adddomain"
	SEARCHING_MODAL  = "searching"
	LIST_URLS_MODAL  = "search"
	TITLE_PANEL      = "title"
	// SAVE_MODAL           = "save_modal"
	// PROGRESS_MODAL       = "progress_modal"
	// Log messages
	ERROR_EMPTY  = "The editor should not be empty!"
	DIAGRAMS_DIR = "/diagrams"
)

// Main views
var panelViews = map[string]panelProperties{
	// scrollable list of pabels
	LIST_PANEL: {
		title: "Parliament Members",
		text:  "", // populated from `list.go`
		// left panel
		x1:       0.0,
		y1:       0.0,
		x2:       1.0 / 3.0,
		y2:       0.7,
		editable: false,
		cursor:   true,
	},
	// 	Panel to list domains and expiries
	// Has functionality to add domain
	DOMAIN_PANEL: {
		title: "Member Domains",
		text:  "", // TODO: update
		// left panel
		x1:       1.0 / 3.0,
		y1:       0.0,
		x2:       2.0 / 3.0,
		y2:       0.7,
		editable: false,
		cursor:   true,
	},
	// Session-only latest WHOIS detail (not persisted)
	WHOIS_PANEL: {
		title: "Domain Information",
		text:  "",
		// right panel
		x1:       2.0 / 3.0,
		y1:       0.0,
		x2:       1.0,
		y2:       0.7,
		editable: false,
		cursor:   false,
	},
	// bottom
	LOG_PANEL: {
		title:    "Log Panel",
		text:     "", // updated with ui.consoleLog
		x1:       0.0,
		y1:       0.7,
		x2:       1.0,
		y2:       1.0,
		editable: false,
		cursor:   false,
	},
	LIST_URLS_PANEL: {
		title:    "List URL Panel",
		text:     "tmp",
		x1:       2.0 / 3.0,
		y1:       0.5,
		x2:       1.0,
		y2:       1.0,
		editable: false,
		cursor:   false,
	},
}

// Modal views
var modalViews = map[string]panelProperties{
	HELP_PANEL: {
		title:    "Key shortcuts",
		text:     "", // set dynamically from keyboard short cuts
		editable: false,
	},
	ADD_DOMAIN_PANEL: {
		title:    "Add domain panel",
		text:     "",
		editable: true,
		cursor:   true,
	},
	SEARCHING_MODAL: {
		title:    "Searching",
		text:     "",
		editable: false,
		cursor:   false,
	},
	LIST_URLS_MODAL: {
		title:    "Select a URL",
		text:     "",
		editable: false,
		cursor:   false,
	},
	TITLE_PANEL: {
		title:    "",
		text:     "",
		editable: false,
		cursor:   false,
	},
}

// Panel views to render
var (
	// Panel Views
	MainViews = []string{
		WHOIS_PANEL,
		LOG_PANEL,
		LIST_PANEL,
		DOMAIN_PANEL,
	}
)

// Panels that can be clicked into
var (
	// Panel Views
	clickableViews = []string{
		//LIST_PANEL,
		//DOMAIN_PANEL,
	}
)

// Layout initialize the panel views and associates the mouse click bindings with them.
func (ui *UI) Layout(g *gocui.Gui) error {
	// During startup title: do not create main panels — only the title modal is shown.
	if ui.titleOnly {
		if _, err := ui.gui.View(TITLE_PANEL); err == nil {
			if _, err := ui.gui.SetViewOnTop(TITLE_PANEL); err != nil && err != gocui.ErrUnknownView {
				return err
			}
		}
		return nil
	}

	if err := ui.ApplyMouseBindings(clickableViews); err != nil {
		return err
	}

	// When expanded, draw the log last so it covers the other panels.
	views := MainViews
	if ui.logExpanded {
		views = make([]string, 0, len(MainViews))
		for _, name := range MainViews {
			if name != LOG_PANEL {
				views = append(views, name)
			}
		}
		views = append(views, LOG_PANEL)
	}

	// Initialize each panel
	for _, view := range views {
		if _, err := ui.initPanelView(view); err != nil {
			return err
		}
	}

	// SetView only updates coordinates; z-order follows creation order, so
	// LIST/DOMAIN stay above LOG unless we explicitly raise it.
	if ui.logExpanded {
		if _, err := ui.gui.SetViewOnTop(LOG_PANEL); err != nil && err != gocui.ErrUnknownView {
			return err
		}
	}

	// Activate the first panel on first run
	if v := ui.gui.CurrentView(); v == nil {
		// ui.log("Setting default view to "+LIST_PANEL, false)
		v, err := ui.gui.SetCurrentView(LIST_PANEL)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		if ui.state.visible != nil && len(*ui.state.visible) != 0 {
			// ui.log("Setting selected MP to 0 ", false)
			v.SetCursor(0, 0)
			// draw
			listPanel := panelViews[LIST_PANEL]
			listPanel.text = panel.DrawListMpPanel(ui.gui, ui.state.visible)
			return ui.writeContent(LIST_PANEL, panelViews[LIST_PANEL].text)
		}
	}
	return nil
	// return g.SetKeybinding(DIAGRAM_PANEL, gocui.MouseWheelDown, gocui.ModNone, ui.scrollDown)
}

// initPanelView initializes the panel view.
func (ui *UI) initPanelView(name string) (*gocui.View, error) {
	if ui.titleOnly {
		return nil, gocui.ErrUnknownView
	}
	maxX, maxY := ui.gui.Size()

	if ui.logExpanded && name == LOG_PANEL {
		return ui.createPanelView(name, 0, 0, maxX-1, maxY-1)
	}

	p := panelViews[name]

	x1 := int(p.x1 * float64(maxX))
	y1 := int(p.y1 * float64(maxY))
	x2 := int(p.x2*float64(maxX)) - 1
	y2 := int(p.y2*float64(maxY)) - 1

	return ui.createPanelView(name, x1, y1, x2, y2)
}

// nextFilter returns next filter for LIST_PANEL
func (ui *UI) nextFilter() string {
	switch ui.state.filter {
	case "all":
		return "have domains"
	case "have domains":
		return "no domains"
	case "no domains":
		return "have alerts"
	case "have alerts":
	default:
	}
	return "all"
}

// mpHasAlert reports whether any of the MP's domains is flagged for review.
func mpHasAlert(mp data.MP) bool {
	for _, d := range mp.Domains {
		if d.Alert {
			return true
		}
	}
	return false
}

// filterIndices returns indexes into all for the given filter.
// nil means every MP (filter "all") — callers should alias visible to all.
func filterIndices(all []data.MP, filter string) []int {
	switch filter {
	case "have domains":
		out := make([]int, 0)
		for i := range all {
			if !all[i].NeedsDomain() {
				out = append(out, i)
			}
		}
		return out
	case "no domains":
		out := make([]int, 0)
		for i := range all {
			if all[i].NeedsDomain() {
				out = append(out, i)
			}
		}
		return out
	case "have alerts":
		out := make([]int, 0)
		for i := range all {
			if mpHasAlert(all[i]) {
				out = append(out, i)
			}
		}
		return out
	default:
		return nil
	}
}

// applyFilter sets the active filter and rebuilds the visible MP list.
// For "all", visible shares the same slice as all so edits are what Save writes.
func (ui *UI) applyFilter(filter string) {
	ui.state.filter = filter
	if ui.state.all == nil {
		empty := []data.MP{}
		ui.state.visible = &empty
		ui.state.visibleIdx = nil
		return
	}
	idxs := filterIndices(*ui.state.all, filter)
	ui.state.visibleIdx = idxs
	if idxs == nil {
		ui.state.visible = ui.state.all
		return
	}
	vis := make([]data.MP, len(idxs))
	for i, j := range idxs {
		vis[i] = (*ui.state.all)[j]
	}
	ui.state.visible = &vis
}

// mpAt returns a pointer to the MP for a visible (LIST_PANEL) index.
// Always points into all, so domain edits survive Ctrl+S.
func (ui *UI) mpAt(visIdx int) *data.MP {
	if ui.state.all == nil || visIdx < 0 {
		return nil
	}
	all := *ui.state.all
	if ui.state.visibleIdx == nil {
		if visIdx >= len(all) {
			return nil
		}
		return &all[visIdx]
	}
	if visIdx >= len(ui.state.visibleIdx) {
		return nil
	}
	j := ui.state.visibleIdx[visIdx]
	if j < 0 || j >= len(all) {
		return nil
	}
	return &all[j]
}

// createPanelView creates the panel view.
func (ui *UI) createPanelView(name string, x1, y1, x2, y2 int) (*gocui.View, error) {
	// FIXME: needed for any logs to be sent
	// ui.log(fmt.Sprintf("DEBUG createPanelView(%s, %d,%d,%d,%d)", name, x1, y1, x2, y2), false)
	v, err := ui.gui.SetView(name, x1, y1, x2, y2)
	if err != gocui.ErrUnknownView && err != nil {
		ui.log(fmt.Sprintf("Log that isn't unknown view %s", err), true)
		return nil, err
	}

	p := panelViews[name]
	v.Title = p.title
	if name == LIST_PANEL {
		v.Title = fmt.Sprintf("%s [%s]", p.title, ui.state.filter)
	}
	if name == LOG_PANEL {
		v.Title = ui.logPanelTitle()
	}
	v.Editable = p.editable

	// generate content for panels
	switch name {
	case LOG_PANEL:
		// Preserve console output across Layout redraws.
		p.text = strings.TrimSuffix(ui.consoleLog, "\n")
		if p.text == "" {
			p.text = strings.TrimSuffix(ui.startupLog, "\n")
		}
	case LIST_PANEL:
		if ui.state.visible != nil {
			p.text = panel.DrawListMpPanel(ui.gui, ui.state.visible)
		}
	case DOMAIN_PANEL:
		if ui.state.domainState != nil && ui.state.domainState.domains != nil {
			p.text = panel.DrawListDomainPanel(ui.gui, ui.state.domainState.domains)
		}
	case WHOIS_PANEL:
		p.text = ui.drawSelectedWhois()
	case LIST_URLS_PANEL:
		if ui.state.searchState != nil && ui.state.searchState.result != nil {
			results := *ui.state.searchState.result
			if ui.state.searchState.term != "" && results != nil {
				newBufferText, _, _, _, _ := panel.DrawListUrlPanel(ui.gui, results, nil, ui.state.searchState.term)
				p.text = newBufferText
			}
		}
	}

	if err := ui.writeContent(name, p.text); err != nil {
		return nil, err
	}

	switch name {
	case LOG_PANEL:
		// Minimized: always follow newest lines. Expanded: keep current Autoscroll
		// so live updates continue until the user pages away from the bottom.
		if !ui.logExpanded {
			v.Autoscroll = true
		}
		break
	case DOMAIN_PANEL:
		v.Highlight = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		v.Wrap = true
		break
		//v.Editor = newList(ui)
	case LIST_PANEL:
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		v.Highlight = true
		v.Editable = true
		v.Autoscroll = false
		break
	case WHOIS_PANEL:
		v.Wrap = true
		v.Editable = false
		v.Autoscroll = false
		break
	default:
		v.Editor = gocui.DefaultEditor //newEditor(ui, nil)
	}
	return v, nil
}

// createModalView creates the modal view.
func (ui *UI) createModalView(name string, x1, y1, x2, y2 int) (*gocui.View, error) {
	v, err := ui.gui.SetView(name, x1, y1, x2, y2)
	if err != gocui.ErrUnknownView {
		return nil, err
	}
	m := modalViews[name]

	v.Title = m.title
	v.Editable = m.editable

	if err := ui.writeContent(name, m.text); err != nil {
		return nil, err
	}

	return v, nil
}

// activatePanelView activates the view defined by id.
func (ui *UI) activatePanelView(id int) error {
	if id > len(tabViews[id]) {
		// dump stack trace to console log
		stack := string(debug.Stack())
		ui.log(fmt.Sprintf("Could not activate panel %d\n%s", id, stack), false)
	}
	if err := ui.setPanelView(tabViews[id]); err != nil {
		return err
	}
	v := panelViews[tabViews[id]]
	ui.gui.Cursor = v.cursor
	ui.currentView = id

	return nil
}

// setPanelView activates the panel view.
func (ui *UI) setPanelView(name string) error {
	//if err := ui.closeModal(ui.currentModal); err != nil {
	//	return err
	//}
	// Save cursor position before switch view (no-op when nothing is focused yet).
	if view := ui.gui.CurrentView(); view != nil {
		x, y := view.Cursor()
		ui.cursors.Set(view.Name(), x, y)
	}

	if _, err := ui.gui.SetCurrentView(name); err != nil {
		if err == gocui.ErrUnknownView {
			return nil
		}
		return err
	}
	return nil
}

// writeContent writes the content into the specific view and set the cursor to the buffer end.
func (ui *UI) writeContent(name, text string) error {
	return ui.writeContent2(name, text, ui.gui)
}

// writeContent writes the content into the specific view and set the cursor to the buffer end.
func (ui *UI) writeContent2(name, text string, g *gocui.Gui) error {
	var v *gocui.View
	var err error
	if g != nil {
		v, err = g.View(name)
	} else {
		v, err = ui.gui.View(name)
	}
	if err != nil {
		return err
	}
	v.Clear()
	fmt.Fprint(v, text)
	// TODO: check if return value should be checked with intellisense
	if v.SetCursor(len(text), 0) == nil {
		ui.cursors.Set(name, len(text), 0)
	}

	return nil
}

// findViewByName find the view defined by name and returns the view index.
func (ui *UI) findViewByName(name string) int {
	var viewId = -1
	for idx, v := range tabViews {
		if v == name {
			viewId = idx
			break
		}
	}
	return viewId
}

// updateView update the view content.
func (ui *UI) updateView(v *gocui.View, buffer string) error {
	if v != nil {
		v.Clear()
		if err := ui.writeContent(v.Name(), buffer); err != nil {
			return err
		}
	}
	return nil
}

// newEditor creates a new GUI editor
/*
func newEditor(ui *UI, handler gocui.Editor) *editor {
		if handler == nil {
					handler = gocui.DefaultEditor
						}
							return &editor{ui, handler, true}
						}
*/

// openModal creates and opens the modal window. If "autoHide" parameter is true, the modal will be automatically closed after 5 seconds.
func (ui *UI) openModal(name string, w, h int, autoHide bool) (*gocui.View, error) {
	v, err := ui.createModal(name, w, h)
	if err != nil {
		return nil, err
	}

	if err := ui.setPanelView(name); err != nil {
		return nil, err
	}
	ui.currentModal = name

	if autoHide {
		// Close the modal automatically after 10 seconds
		ui.modalTimer = time.AfterFunc(10*time.Second, func() {
			ui.gui.Update(func(*gocui.Gui) error {
				if err := ui.closeModal(name); err != nil {
					return err
				}
				return nil
			})
		})
	}
	return v, nil
}

// closeModal closes the modal window and restores the focus to the last accessed panel view.
func (ui *UI) closeModal(modals ...string) error {
	for _, name := range modals {
		if _, err := ui.gui.View(name); err != nil {
			if err == gocui.ErrUnknownView {
				return nil
			}
			return err
		}
		ui.gui.DeleteView(name)
		// Keep keybindings: they are registered once at startup for modal view
		// names (e.g. Select a URL). Deleting them here breaks navigation the
		// next time the modal is opened.
		ui.gui.Cursor = true
		ui.currentModal = ""
	}
	return ui.activatePanelView(ui.currentView)
}

// createModal initializes and creates the modal view.
func (ui *UI) createModal(name string, w, h int) (*gocui.View, error) {
	width, height := ui.gui.Size()
	x1, y1 := width/2-w/2, int(math.Ceil(float64(height/2-h/2-1)))
	x2, y2 := width/2+w/2, int(math.Ceil(float64(height/2+h/2+1)))

	return ui.createModalView(name, x1, y1, x2, y2)
}

// closeOpenedModals closes all the opened modal elements.
func (ui *UI) closeOpenedModals(views []string) error {
	for _, v := range views {
		if view, _ := ui.gui.View(v); view != nil {
			name := view.Name()
			if name == LIST_URLS_MODAL {
				ui.resetURLSearchState()
			}
			ui.closeModal(name)
		}
	}
	return nil
}

// nextView activate the next panel.
func (ui *UI) nextView(wrap bool) error {
	var index int
	index = ui.currentView + 1
	if index > len(tabViews)-1 {
		if wrap {
			index = 0
		} else {
			return nil
		}
	}
	ui.currentView = index % len(tabViews)
	return ui.activatePanelView(ui.currentView)
}

// prevView activate the previous panel.
func (ui *UI) prevView(wrap bool) error {
	var index int
	index = ui.currentView - 1
	if index < 0 {
		if wrap {
			index = len(tabViews) - 1
		} else {
			return nil
		}
	}
	ui.currentView = index % len(tabViews)
	return ui.activatePanelView(ui.currentView)
}

// scrollWhoisPage scrolls the Domain Information panel by one page.
// direction < 0 scrolls up; direction > 0 scrolls down.
func (ui *UI) scrollWhoisPage(direction int) error {
	return ui.scrollViewPage(WHOIS_PANEL, direction)
}

// scrollLogPage scrolls the Log panel by one page.
// direction < 0 scrolls up; direction > 0 scrolls down.
// Paging up disables autoscroll; reaching the bottom re-enables it.
func (ui *UI) scrollLogPage(direction int) error {
	v, err := ui.gui.View(LOG_PANEL)
	if err != nil {
		return nil
	}
	if direction < 0 {
		v.Autoscroll = false
	}
	if err := ui.scrollViewPage(LOG_PANEL, direction); err != nil {
		return err
	}
	_, sy := v.Size()
	if sy <= 0 {
		return nil
	}
	_, oy := v.Origin()
	lines := len(v.ViewBufferLines())
	maxOy := lines - sy
	if maxOy < 0 {
		maxOy = 0
	}
	if direction > 0 && oy >= maxOy {
		v.Autoscroll = true
	}
	return nil
}

// scrollViewPage scrolls named view by one page of visible rows.
// direction < 0 scrolls up; direction > 0 scrolls down.
func (ui *UI) scrollViewPage(name string, direction int) error {
	v, err := ui.gui.View(name)
	if err != nil {
		return nil
	}
	_, sy := v.Size()
	if sy <= 0 {
		return nil
	}
	ox, oy := v.Origin()
	lines := len(v.ViewBufferLines())
	maxOy := lines - sy
	if maxOy < 0 {
		maxOy = 0
	}
	newOy := oy + direction*sy
	if newOy < 0 {
		newOy = 0
	}
	if newOy > maxOy {
		newOy = maxOy
	}
	return v.SetOrigin(ox, newOy)
}

// logPanelTitle returns the Log Panel frame title, including expand/scroll hints.
func (ui *UI) logPanelTitle() string {
	if ui.logExpanded {
		return "Log Panel — l: collapse, <PGUP>: up, <PGDN>: down"
	}
	return "Log Panel — l: expand"
}

// toggleLogExpanded expands the log panel to full height, or restores the
// normal layout. Always resets the log viewport to the bottom.
func (ui *UI) toggleLogExpanded() error {
	ui.logExpanded = !ui.logExpanded
	if err := ui.Layout(ui.gui); err != nil {
		return err
	}
	return ui.resetLogToBottom()
}

// resetLogToBottom scrolls the log panel to the newest lines and restores autoscroll.
func (ui *UI) resetLogToBottom() error {
	v, err := ui.gui.View(LOG_PANEL)
	if err != nil {
		return nil
	}
	v.Autoscroll = true
	_, sy := v.Size()
	if sy <= 0 {
		return nil
	}
	ox, _ := v.Origin()
	lines := len(v.ViewBufferLines())
	maxOy := lines - sy
	if maxOy < 0 {
		maxOy = 0
	}
	return v.SetOrigin(ox, maxOy)
}

// ClearView clears the panel view.
func (ui *UI) ClearView(name string) {
	v, _ := ui.gui.View(name)
	v.Clear()
}

// DeleteView deletes the current view.
func (ui *UI) DeleteView(name string) {
	v, _ := ui.gui.View(name)
	ui.gui.DeleteView(v.Name())
}
