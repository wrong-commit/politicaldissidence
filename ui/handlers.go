package ui

import (
	"bytes"
	"fmt"
	"politicaldissidence/ui/panel"
	"strings"
	"text/tabwriter"

	"github.com/jroimartin/gocui"
)

// Fn is a generic function acting as a closure function for event handlers.
type Fn func(*gocui.Gui, *gocui.View) error

type handler struct {
	views   []string
	key     interface{}
	keyName string
	help    string
	action  func(*UI, bool) Fn
}

type handlers []handler

var listView = []string{LIST_PANEL}

var addDomainView = []string{ADD_DOMAIN_PANEL}

var tabViews = []string{LIST_PANEL, DOMAIN_PANEL}
var domainViews = []string{DOMAIN_PANEL}
var listUrlView = []string{LIST_URLS_MODAL}

// TODO: handlers should be passed a Shift modifier
var keyHandlers = &handlers{
	// LIST_URLS_PANEL :
	// up/down - keys to navigate URLs
	// enter - choose URL
	{listUrlView, gocui.KeyArrowDown, "<DOWN>", "Next Url", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			return ui.moveURLListSelection(v, 1)
		}
	}},
	{listUrlView, gocui.KeyArrowUp, "<UP>", "Prev Url", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			return ui.moveURLListSelection(v, -1)
		}
	}},
	{listUrlView, gocui.KeyEnter, "<ENTER>", "Add Domain", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			link, err := ui.selectedURLListLink(v)
			if err != nil {
				return ui.log(err.Error(), true)
			}
			domain, ok := panel.HostFromURL(link[0])
			if !ok || strings.TrimSpace(domain) == "" {
				return ui.log(fmt.Sprintf("Could not parse domain from <%s>", link[0]), true)
			}
			defer ui.closeListUrlsModal()
			return ui.addDomain(domain, ui.state.currentIndex, true)
		}
	}},
	{listUrlView, 'c', "c", "Copy Link", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			link, err := ui.selectedURLListLink(v)
			if err != nil {
				return ui.log(err.Error(), true)
			}
			url := strings.TrimSpace(link[0])
			if url == "" {
				return ui.log("Selected link has empty URL", true)
			}
			if err := copyToClipboard(url); err != nil {
				return ui.log(fmt.Sprintf("Could not copy to clipboard: %v", err), true)
			}
			return ui.log(fmt.Sprintf("Copied to clipboard: %s", url), false)
		}
	}},
	{listUrlView, gocui.KeyArrowRight, "<RIGHT>", "Next search page", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			return ui.changeURLSearchPage(g, 1)
		}
	}},
	{listUrlView, gocui.KeyArrowLeft, "<LEFT>", "Prev search page", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			return ui.changeURLSearchPage(g, -1)
		}
	}},
	{listUrlView, 'e', "e", "Toggle search engine", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			return ui.toggleURLSearchEngine(g)
		}
	}},
	{listUrlView, 't', "t", "Toggle search term", func(ui *UI, wrap bool) Fn {
		return func(g *gocui.Gui, v *gocui.View) error {
			return ui.toggleURLSearchTerm(g)
		}
	}},
	// LIST_PANEL:
	//	up/down -  keys to navigate MPs
	{listView, gocui.KeyArrowUp, "<UP>", "Previous Mp", onPrevMp},
	{listView, gocui.KeyArrowDown, "<DOWN>", "Next Mp", onNextMp},
	// 	f - change filters
	{listView, 'f', "f", "Change filter of visible MPs (all/with domains/no domains/have alerts)", func(ui *UI, wrap bool) Fn {
		onFilter := func(_ *gocui.Gui, v *gocui.View) error {
			ui.applyFilter(ui.nextFilter())
			// Force selectMp to refresh domainState for the new filtered list.
			ui.state.currentIndex = -1
			if _, err := ui.initPanelView(LIST_PANEL); err != nil {
				return err
			}
			if err := setListCursor(v, 0); err != nil {
				return err
			}
			return ui.selectMp(0)
		}
		return onFilter
	}},
	// DOMAIN_PANEL:
	//	up/down -  keys to select domains
	{domainViews, gocui.KeyArrowUp, "<UP>", "Previous Domain", onPrevDomain},
	{domainViews, gocui.KeyArrowDown, "<DOWN>", "Domain Mp", onNextDomain},
	//	g - Guess domain
	{tabViews, 'g', "g", "Guess domain", onGuessDomain},
	//	u - update domain
	{domainViews, 'u', "u", "Check Domain", onCheckDomain},
	//	ctrl d - remove selected domain from MP
	{domainViews, gocui.KeyCtrlD, "<CTRL>+d", "Remove Domain", onRemoveDomain},
	// ADD_DOMAIN_PANEL:
	// 	enter - confirm url to search
	{addDomainView, gocui.KeyEnter, "Enter", "Confirm Domain", onConfirmNewDomain},
	// 	tab - cycle main panels
	{tabViews, gocui.KeyTab, "<TAB>", "Next Panel", onNextPanel},
	// 	ctrl c - close modal or quit application
	{nil, gocui.KeyCtrlC, "<CTRL>+c", "Quit/Close Modal", onQuit},
	// 	ctrl s - save
	{nil, gocui.KeyCtrlS, "<CTRL>+s", "Save ", onSave},
	// 	ctrl r - reload
	{nil, gocui.KeyCtrlR, "<CTRL>+r", "Reload ", onReload},
	// 	ctrl p - force recheck all domains (ignore lastChecked)
	{nil, gocui.KeyCtrlP, "<CTRL>+p", "Force recheck all domains (WHOIS/DNS/HTTPS)", onForceRecheckDomains},
	// 	ctrl l - refresh MPs from configured CSV
	{nil, gocui.KeyCtrlL, "<CTRL>+l", "Reload csv_refresh.json + refresh MPs", onCsvRefresh},
	// 	l - expand / restore log panel
	{nil, 'l', "l", "Toggle log panel full height", onToggleLog},
	// Domain Information / Log panel scroll — global; panels are not focusable
	{nil, gocui.KeyPgup, "<PGUP>", "Scroll Domain Information or Log up", onPageUp},
	{nil, gocui.KeyPgdn, "<PGDN>", "Scroll Domain Information or Log down", onPageDown},
}

// onCheckDomain updates the expiry
func onCheckDomain(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onCheckDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.checkDomain()
	}
}

// onRemoveDomain deletes the selected domain from the current MP.
func onRemoveDomain(ui *UI, wrap bool) Fn {
	return func(*gocui.Gui, *gocui.View) error {
		return ui.removeDomain()
	}
}

// onForceRecheckDomains force-rechecks all domains, ignoring lastChecked / freshness.
func onForceRecheckDomains(ui *UI, wrap bool) Fn {
	return func(*gocui.Gui, *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		return ui.rerunBackgroundChecks(true)
	}
}

// onCsvRefresh fetches/parses the configured CSV and merges into memory (Ctrl+S to save).
func onCsvRefresh(ui *UI, wrap bool) Fn {
	return func(*gocui.Gui, *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		go ui.tryCsvRefresh()
		return nil
	}
}

// onOpenGuessDomain opens the "Add Domain" modal
func onGuessDomain(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onOpenGuessDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.toggleSearchingModal(g)
	}
}

// onConfirmNewDomain opens the "Add Domain" modal
func onConfirmNewDomain(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onAddDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		ui.log(fmt.Sprintf("Calling onAddDomain(%s)", v.Buffer()), false)
		return ui.addDomainModalTest(v)
	}
}

// onPrevPanel retrieves the previous panel
func onPrevPanel(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onPrevPanel", false)
	return func(*gocui.Gui, *gocui.View) error {
		return ui.prevView(wrap)
	}
}

// onNextPanel retrieves the next panel.
func onNextPanel(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onNexPanel", false)
	return func(*gocui.Gui, *gocui.View) error {
		return ui.nextView(wrap)
	}
}

// onPrevMp retrieves the next panel.
func onPrevMp(ui *UI, _ bool) Fn {
	// ui.log("[*] register onPrevMp", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.logExpanded {
			return nil
		}
		return ui.prevMp(v)
	}
}

// onNextPanel retrieves the next panel.
func onNextMp(ui *UI, _ bool) Fn {
	// ui.log("[*] register onNextMp", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.logExpanded {
			return nil
		}
		return ui.nextMp(v)
	}
}

// onPrevDomain retrieves the next panel.
func onPrevDomain(ui *UI, _ bool) Fn {
	// ui.log("[*] register onPrevDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.logExpanded {
			return nil
		}
		return ui.prevDomain(v)
	}
}

// onNextDomain retrieves the next panel.
func onNextDomain(ui *UI, _ bool) Fn {
	// ui.log("[*] register onNextDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.logExpanded {
			return nil
		}
		return ui.nextDomain(v)
	}
}

// onToggleLog expands or restores the log panel height.
func onToggleLog(ui *UI, _ bool) Fn {
	return func(*gocui.Gui, *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		return ui.toggleLogExpanded()
	}
}

// onPageUp scrolls the expanded log panel, or Domain Information when log is minimized.
func onPageUp(ui *UI, _ bool) Fn {
	return func(*gocui.Gui, *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		if ui.logExpanded {
			return ui.scrollLogPage(-1)
		}
		return ui.scrollWhoisPage(-1)
	}
}

// onPageDown scrolls the expanded log panel, or Domain Information when log is minimized.
func onPageDown(ui *UI, _ bool) Fn {
	return func(*gocui.Gui, *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		if ui.logExpanded {
			return ui.scrollLogPage(1)
		}
		return ui.scrollWhoisPage(1)
	}
}

// ApplyKeyBindings applies key bindings to panel views.
func (handlers handlers) ApplyKeyBindings(ui *UI, g *gocui.Gui) error {
	// ui.log("Applying keybindings", false)
	for _, h := range handlers {
		if len(h.views) == 0 {
			h.views = []string{""}
		}
		if h.action == nil {
			continue
		}
		for _, view := range h.views {
			if err := g.SetKeybinding(view, h.key, gocui.ModNone, h.action(ui, true)); err != nil {
				return err
			}
			// ui.log(fmt.Sprintf("KB->%s %s", h.views, h.keyName), false)
		}
	}

	if err := g.SetKeybinding("", gocui.KeyCtrlH, gocui.ModNone, onHelp(ui, handlers)); err != nil {
		return err
	}
	return nil
}

// onHelp opens the Help Content modl
func onHelp(ui *UI, handler handlers) Fn {
	// ui.log("[*] register onHelp", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		return ui.toggleHelp(g, handler.HelpContent(v.Name()))
	}
}

// onSave persists the MPs and domains to disk
func onSave(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onSave", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		return ui.Save()
	}
}

// onReload reeads the MPs and domains from disk
func onReload(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onReload", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.titleScreenBlocking() {
			return nil
		}
		return ui.Reload()
	}
}

// onQuit is an event listener which get triggered when a quit action is performed.
func onQuit(ui *UI, wrap bool) Fn {
	//ui.log("[*] register onQuit", false)
	return func(*gocui.Gui, *gocui.View) error {
		// Title screen is not dismissible — quit the app (same as no modal).
		if ui.currentModal == "" || ui.currentModal == TITLE_PANEL {
			return gocui.ErrQuit
		}
		return ui.closeOpenedModals([]string{ui.currentModal})
	}
}

const HELP_TABLE_TITLE = "Key \t | Desc \t \n"

// HelpContent populates the help panel.
func (handlers handlers) HelpContent(activePanel string) string {
	buf := &bytes.Buffer{}
	minWidth := 10
	tabWidth := 2
	padding := 1
	w := tabwriter.NewWriter(buf, minWidth, tabWidth, padding, ' ', tabwriter.DiscardEmptyColumns)

	// handlers by view
	var handlersWhere map[string][]handler = make(map[string][]handler)
	// global handlers
	var anywhere []handler = make([]handler, 0)

	fmt.Fprint(w, "\n")
	// put handlers in map
	for _, h := range handlers {
		if h.views == nil {
			anywhere = append(anywhere, h)
			continue
		}
		// create map of views to handlers
		for _, view := range h.views {
			entry, ok := handlersWhere[view]
			if !ok {
				handlersWhere[view] = make([]handler, 0)
			}
			handlersWhere[view] = append(entry, h)
		}
	}
	// print map into tables
	for view, viewHandlers := range handlersWhere {
		// LOOK AT MOI: filter out non current panel
		if view != activePanel {
			continue
		}

		fmt.Fprintf(w, "\n\tPanel '%s'\t\n", view)
		fmt.Fprint(w, HELP_TABLE_TITLE)
		for _, h := range viewHandlers {
			if h.keyName == "" || h.help == "" {
				continue
			}
			fmt.Fprintf(w, "  %s\t: %s\t%s\n", h.keyName, h.help, "")
		}
	}

	fmt.Fprintf(w, "\n\tGloba handlers\t\n")
	fmt.Fprint(w, HELP_TABLE_TITLE)
	fmt.Fprintf(w, "  %s\t: %s\n", "<CTRL>+h", "Toggle Help")
	for _, handler := range anywhere {
		if handler.keyName == "" || handler.help == "" {
			continue
		}
		fmt.Fprintf(w, "  %s\t: %s\t%s\n", handler.keyName, handler.help, "")
	}
	fmt.Fprint(w, "\n")
	w.Flush()
	return buf.String()

	// return text
}

func (h handler) whenIn() string {
	var whenIn string
	if len(h.views) == 0 {
		whenIn = " anywhere"
	} else if len(h.views) == 1 {
		whenIn = " when in " + h.views[0]
	} else {
		whenIn = " when in (" + strings.Join(h.views, ",") + ")"
	}
	return whenIn
}
