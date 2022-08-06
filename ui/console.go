package ui

import (
	"strings"
	"time"
)

const timeFormat = "06-01-02 15:04:05.000"

// decorate changes the color of a string
func decorate(s string, color string) string {
	switch color {
	case "green":
		s = "\x1b[0;32m" + s
	case "red":
		s = "\x1b[0;31m" + s
	default:
		return s
	}
	return s + "\x1b[0m"
}

// log writes the log message
func (ui *UI) log(message string, isError bool) error {
	if isError {
		message = decorate(message, "red")
	} else {
		message = decorate(message, "green")
	}
	if !ui.started {
		ui.startupLog += time.Now().Format(timeFormat) + message + "\n"
		return nil
	}
	ui.consoleLog += time.Now().Format(timeFormat) + message + "\n"
	return ui.writeContent(LOG_PANEL, strings.TrimSuffix(ui.consoleLog, "\n"))
}

// clearLog clears the log message.
func (ui *UI) clearLog() error {
	return ui.writeContent(LOG_PANEL, "[JUST CLEARED CONSOLE]")
}
