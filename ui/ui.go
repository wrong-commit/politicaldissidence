package ui

import (
	// 	"fmt"
	"fmt"
	"log"
	"time"

	"politicaldissidence/data"
	"politicaldissidence/jobs"
	"politicaldissidence/refresh"
	"politicaldissidence/searching"

	"github.com/jroimartin/gocui"
)

// UI defines the basic UI components.
type UI struct {
	gui          *gocui.Gui
	currentView  int
	nextItem     int
	currentModal string
	// Capture log() message calls
	consoleLog string
	// Capture startup logs separate. Clear after init
	startupLog string
	cursors    Cursors
	modalTimer *time.Timer
	logTimer   *time.Timer
	// True when application has started
	started bool
	// UI/application state
	state *State
	// Single-flight background WHOIS refresh
	whoisRunner refresh.Runner
	// Jobs kicked when a domain is added (WHOIS, calc demo, …)
	domainAddedJobs []jobs.Job
	// fontPath     string
}

// Top level application state
type State struct {
	all     *[]data.MP
	visible *[]data.MP
	// all | have domains | no domains
	filter       string
	currentIndex int
	//
	domainState *DomainState
	// search state
	searchState *SearchState
	// session search prefs (engine / term); survive closing Select a URL
	searchPrefs SearchPrefs
}

// State for the Domain list
type DomainState struct {
	// all domains
	domains *[]data.Domain
	// current index
	index int
}

// NewUI returns a new UI component (what this mean?)
func NewUI() *UI {
	var err error
	ui := new(UI)
	ui.state = &State{filter: "all", currentIndex: -1, searchPrefs: SearchPrefs{
		engine:    searching.EngineBing,
		termIndex: 1,
	}}
	ui.gui, err = gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
		ui.log("NewUI() -> "+fmt.Sprint(err.Error()), true)
	}
	ui.cursors = NewCursors()
	ui.domainAddedJobs = []jobs.Job{
		jobs.NewWhoisOnAdd(ui.refreshDomainWhois),
		// jobs.NewCalcDemo(),
	}
	return ui
}

// Init sets up gui views and layout manager
func (ui *UI) Init() {
	if err := ui.initGui(ui.gui); err != nil {
		log.Panicln(err)
		ui.log("Init() -> "+fmt.Sprint(err.Error()), true)
	}
}

// initGui initializes the GUI.
func (ui *UI) initGui(g *gocui.Gui) error {
	// Default Panel settings
	ui.gui.Highlight = true
	ui.gui.InputEsc = false
	ui.gui.SelFgColor = gocui.ColorGreen

	// Mouse settings
	ui.gui.Cursor = true
	ui.gui.Mouse = false

	// Set current view to list of MPs
	ui.currentView = 0
	// TODO: remove
	ui.nextItem = 0

	// Set Layout function
	ui.gui.SetManager(ui)

	ui.state.searchState = &SearchState{term: "", result: &[]searching.Link{}}
	ui.ensureSearchPrefs()

	searching.DebugLog = func(msg string) {
		ui.searchDebugLog(msg)
	}

	// Register keybindings
	err := keyHandlers.ApplyKeyBindings(ui, g)
	return err
}

// searchDebugLog writes a searching DEBUG line to the console (safe from worker goroutines).
func (ui *UI) searchDebugLog(msg string) {
	if ui.gui == nil || !ui.started {
		_ = ui.logPlain(msg)
		return
	}
	ui.gui.Update(func(g *gocui.Gui) error {
		return ui.logPlain(msg)
	})
}

// Loop starts the GUI loop.
func (ui *UI) Loop() {
	if err := ui.gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
		ui.log("PANIC -> "+fmt.Sprint(err.Error()), true)
	}
}

// Close closes the app.
func (ui *UI) Close() {
	ui.gui.Close()
	// dump to console
	log.Print(ui.startupLog)
	// log.Print(ui.consoleLog)
}
