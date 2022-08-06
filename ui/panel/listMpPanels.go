package panel

import (
	"fmt"
	"politicaldissidence/data"

	"github.com/jroimartin/gocui"
)

func DrawListMpPanel(g *gocui.Gui, mps *[]data.MP) string {
	// var sb strings.Builder
	var sb string
	for i, mp := range *mps {
		sb += fmt.Sprintf("%d \t%s\n", i, mp.ToString())
	}
	return sb
}
