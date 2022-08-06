package ui

import (
	"fmt"
)

// pad betweeen name and party within width. if len(name) + len("[${party}]") +  is longer than width, don't pad
func WriteMpName(width int, name string, party string) string {
	// padded := fmt.Sprintf("%s [%s]", name, party)
	var padded string
	outputsize := len(name) + len(party) + 3
	if outputsize > width {
		padded = fmt.Sprintf("%s [%s]", name, party)
	} else {
		// pad party and name a bit
		padSize := width - outputsize
		padded = name
		for i := 0; i < padSize; i++ {
			padded += " "
		}
		padded += fmt.Sprintf("[%s]", party)
	}

	return padded
}
