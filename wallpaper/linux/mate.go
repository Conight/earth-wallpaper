//go:build linux

package linux

import (
	"os/exec"
	"strconv"

	"earth-wallpaper/wallpaper/modes"
)

func SetMate(file string, mode modes.FillStyle) error {
	err := exec.Command("dconf", "write", "/org/mate/desktop/background/picture-options",
		strconv.Quote(getGNOMEString(mode))).Run()
	if err != nil {
		return err
	}

	return exec.Command("dconf", "write", "/org/mate/desktop/background/picture-filename", strconv.Quote(file)).Run()

}
