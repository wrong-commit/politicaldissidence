package ui

import (
	// 	"fmt"
	"fmt"
	"log"
	"sync"
	"time"

	"politicaldissidence/data"

	"github.com/jroimartin/gocui"
)

// UI defines the basic UI components.
type UI struct {
	gui          *gocui.Gui
	currentView  int
	nextItem     int
	currentModal string
	consoleLog   string
	startupLog   string
	cursors      Cursors
	modalTimer   *time.Timer
	logTimer     *time.Timer
	// True when application has started
	started bool
	mutex   *sync.Mutex
	// UI/application state
	state *State
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
	ui.state = &State{nil, nil, "all", -1, nil}
	ui.gui, err = gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
		ui.log("NewUI() -> "+fmt.Sprint(err.Error()), true)
	}
	ui.cursors = NewCursors()
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

	// Register keybindings
	err := keyHandlers.ApplyKeyBindings(ui, g)
	return err
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
