package ui

import (
	"fmt"
	"politicaldissidence/data"
	"strings"

	"github.com/jroimartin/gocui"
)

// TODO: clicking in this updates cursor but currentMp not reset
type list struct {
	ui *UI
	// maybe not needed a direct reference too ?
	editor gocui.Editor
	// define remaining list editor properties
}

func newList(ui *UI) *list {
	ui.log("newList()", false)
	return &list{ui, gocui.DefaultEditor}
}

func ListText(selected int, mps *[]data.MP) string {
	var sb strings.Builder
	for i, mp := range *mps {
		sb.WriteString(fmt.Sprintf("%d \t%s\n", i, mp.ToString()))
	}
	return sb.String()
}

func (l *list) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	l.ui.log("list.Edit called()\n", true)
	_, y := v.Cursor()
	maxY := strings.Count(v.Buffer(), "\n")
	switch key {
	case gocui.KeyArrowDown:
		if y < maxY {
			v.MoveCursor(0, 1, true)
		}
	case gocui.KeyArrowUp:
		if y > 0 {
			v.MoveCursor(0, -1, false)
		}
	}
	//case gocui.KeyArrowLeft:
	//	v.MoveCursor(-1, 0, false)
	//case gocui.KeyArrowRight:
	//	v.MoveCursor(1, 0, false)
	//}
}
