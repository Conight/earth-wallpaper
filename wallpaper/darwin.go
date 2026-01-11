//go:build darwin

package wallpaper

import (
	"earth-wallpaper/wallpaper/modes"
	"os/exec"
	"strconv"
)

// SetFromFile uses AppleScript to tell Finder to set the desktop wallpaper to specified file.
func setFromFile(file string, _ modes.FillStyle) error {
	return exec.Command("osascript", "-e", `tell application "System Events" to tell every desktop to set picture to `+strconv.Quote(file)).Run()
}
