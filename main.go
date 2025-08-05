package main

import (
	"fmt"
	"os"

	"go-toolgit/cmd/cli"
	fynegui "go-toolgit/internal/fyne-gui"
)

func main() {
	if shouldLaunchGUI() {
		launchGUI()
	} else {
		cli.Execute()
	}
}

func shouldLaunchGUI() bool {
	for _, arg := range os.Args {
		if arg == "--gui" {
			return true
		}
	}
	return false
}

func launchGUI() {
	fmt.Println("Launching Fyne GUI interface...")
	app := fynegui.NewFyneApp()
	app.Run()
}