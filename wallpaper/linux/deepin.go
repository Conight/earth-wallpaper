//go:build linux

package linux

import (
	"earth-wallpaper/wallpaper/modes"
	"os/exec"
	"strconv"
)

func SetDeepin(file string, mode modes.FillStyle) error {
	err := exec.Command("dconf", "write", "/com/deepin/wrap/gnome/desktop/background/picture-options",
		strconv.Quote(getGNOMEString(mode))).Run()
	if err != nil {
		return err
	}

	return exec.Command("dconf", "write", "/com/deepin/wrap/gnome/desktop/background/picture-uri",
		strconv.Quote("file://"+file)).Run()
}
