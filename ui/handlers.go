package ui

import (
	"bytes"
	"fmt"
	"politicaldissidence/data"
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
	{listUrlView, gocui.KeyEnter, "Enter", "Add Domain", func(ui *UI, wrap bool) Fn {
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
	// LIST_PANEL:
	//	up/down -  keys to navigate MPs
	{listView, gocui.KeyArrowUp, "<UP>", "Previous Mp", onPrevMp},
	{listView, gocui.KeyArrowDown, "<DOWN>", "Next Mp", onNextMp},
	// 	ctrl f - 	change filters
	{listView, gocui.KeyCtrlF, "Ctrl+F", "Change filter of visible MPs (all/with domains/no domains)", func(ui *UI, wrap bool) Fn {
		// ui.log("[*] register LIST_URLS_PANEL:gocui.KeyCtrlF", false)
		onFilter := func(*gocui.Gui, *gocui.View) error {
			nextFilter := ui.nextFilter()
			nextVisi := make([]data.MP, 0)
			switch nextFilter {
			case "all":
				nextVisi = *ui.state.all
			case "have domains":
				for _, x := range *ui.state.all {
					if !x.NeedsDomain() {
						nextVisi = append(nextVisi, x)
					}
				}
			case "no domains":
				for _, x := range *ui.state.all {
					if x.NeedsDomain() {
						nextVisi = append(nextVisi, x)
					}
				}
			}
			ui.state.visible = &nextVisi
			ui.state.filter = nextFilter
			ui.selectMp(0)
			return nil
		}
		return onFilter
	}},
	// DOMAIN_PANEL:
	//	up/down -  keys to select domains
	{domainViews, gocui.KeyArrowUp, "<UP>", "Previous Domain", onPrevDomain},
	{domainViews, gocui.KeyArrowDown, "<DOWN>", "Domain Mp", onNextDomain},
	//	ctrl a - add a new domain
	{tabViews, gocui.KeyCtrlA, "Ctrl+A", "Add Domain", onOpenAddDomain},
	//	ctrl g - Guess domain
	{tabViews, gocui.KeyCtrlG, "Ctrl+G", "Guess domain", onGuessDomain},
	//	ctrl u - update domain
	{domainViews, gocui.KeyCtrlU, "Ctrl+U", "Check Domain", onCheckDomain},
	// ADD_DOMAIN_PANEL:
	// 	enter - confirm url to search
	{addDomainView, gocui.KeyEnter, "Enter", "Confirm Domain", onConfirmNewDomain},
	// 	tab - cycle main panels
	{tabViews, gocui.KeyTab, "Tab", "Next Panel", onNextPanel},
	// 	ctrl c - close modal or quit application
	{nil, gocui.KeyCtrlC, "Ctrl+C", "Quit/Close Modal", onQuit},
	// 	ctrl s - save
	{nil, gocui.KeyCtrlS, "Ctrl+S", "Save ", onSave},
	// 	ctrl r - reload
	{nil, gocui.KeyCtrlR, "Ctrl+R", "Reload ", onReload},
}

// onCheckDomain updates the expiry
func onCheckDomain(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onCheckDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.checkDomain()
	}
}

// onOpenGuessDomain opens the "Add Domain" modal
func onGuessDomain(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onOpenGuessDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.toggleSearchingModal(g)
	}
}

// onOpenAddDomain opens the "Add Domain" modal
func onOpenAddDomain(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onOpenAddDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.toggleNewDomain(g)
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
		return ui.prevMp(v)
	}
}

// onNextPanel retrieves the next panel.
func onNextMp(ui *UI, _ bool) Fn {
	// ui.log("[*] register onNextMp", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.nextMp(v)
	}
}

// onPrevDomain retrieves the next panel.
func onPrevDomain(ui *UI, _ bool) Fn {
	// ui.log("[*] register onPrevDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.prevDomain(v)
	}
}

// onNextDomain retrieves the next panel.
func onNextDomain(ui *UI, _ bool) Fn {
	// ui.log("[*] register onNextDomain", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.nextDomain(v)
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
		return ui.toggleHelp(g, handler.HelpContent(v.Name()))
	}
}

// onSave persists the MPs and domains to disk
func onSave(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onSave", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.Save()
	}
}

// onReload reeads the MPs and domains from disk
func onReload(ui *UI, wrap bool) Fn {
	// ui.log("[*] register onReload", false)
	return func(g *gocui.Gui, v *gocui.View) error {
		return ui.Reload()
	}
}

// onQuit is an event listener which get triggered when a quit action is performed.
func onQuit(ui *UI, wrap bool) Fn {
	//ui.log("[*] register onQuit", false)
	return func(*gocui.Gui, *gocui.View) error {
		if ui.currentModal == "" {
			return gocui.ErrQuit
		} else {
			return ui.closeOpenedModals([]string{ui.currentModal})
		}
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
	fmt.Fprintf(w, "  %s\t: %s\n", "Ctrl+h", "Toggle Help")
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
