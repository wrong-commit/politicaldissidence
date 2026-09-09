package panel

import (
	"strings"
	"unicode/utf8"
)

// TitleASCIIDims returns the art block width (max line rune count) and height (line count).
func TitleASCIIDims(art string) (width, height int) {
	if art == "" {
		return 0, 0
	}
	lines := strings.Split(art, "\n")
	height = len(lines)
	for _, line := range lines {
		n := utf8.RuneCountInString(line)
		if n > width {
			width = n
		}
	}
	return width, height
}

// DrawTitleASCII centers art horizontally and vertically within a view of viewW×viewH cells.
// If art is empty, the result is empty. Lines longer than viewW are truncated; extra
// vertical lines beyond viewH are clipped after vertical centering.
func DrawTitleASCII(art string, viewW, viewH int) string {
	if viewW < 1 {
		viewW = 1
	}
	if viewH < 1 {
		viewH = 1
	}
	if art == "" {
		return ""
	}

	lines := strings.Split(art, "\n")
	artW, artH := TitleASCIIDims(art)

	padTop := 0
	if artH < viewH {
		padTop = (viewH - artH) / 2
	}
	leftPad := 0
	if artW < viewW {
		leftPad = (viewW - artW) / 2
	}
	padPrefix := strings.Repeat(" ", leftPad)

	out := make([]string, 0, viewH)
	for i := 0; i < padTop && len(out) < viewH; i++ {
		out = append(out, "")
	}
	for _, line := range lines {
		if len(out) >= viewH {
			break
		}
		runes := []rune(line)
		if len(runes) > viewW {
			runes = runes[:viewW]
			out = append(out, string(runes))
			continue
		}
		// If leftPad + line would exceed viewW, shrink pad.
		pad := leftPad
		if pad+len(runes) > viewW {
			pad = viewW - len(runes)
			if pad < 0 {
				pad = 0
			}
		}
		if pad == leftPad {
			out = append(out, padPrefix+string(runes))
		} else {
			out = append(out, strings.Repeat(" ", pad)+string(runes))
		}
	}
	for len(out) < viewH {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}
