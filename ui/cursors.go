package ui

/**
 * Define methods for Cursor persisting logic when switching between views
 */

import (
	"github.com/jroimartin/gocui"
)

// Cursors stores the cursor position for a specific panel view.
// Used to restore mouse position when click is detected.
type Cursors map[string]struct{ x, y int }

// NewCursors instantiate Cursors map which contains the cursor current position.
func NewCursors() Cursors {
	return make(Cursors)
}

// Restore restores cursor previous position.
func (c Cursors) Restore(view *gocui.View) error {
	return view.SetCursor(c.Get(view.Name()))
}

// Get returns the stored cursor position.
func (c Cursors) Get(view string) (int, int) {
	if v, ok := c[view]; ok {
		return v.x, v.y
	}
	return 0, 0
}

// Set stores the mouse position.
func (c Cursors) Set(view string, x, y int) {
	c[view] = struct{ x, y int }{x, y}
}
