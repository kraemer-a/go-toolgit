//go:build !wails

package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"go-toolgit/cmd/cli"
	"go-toolgit/internal/gui"
	fynegui "go-toolgit/internal/fyne-gui"
)

//go:embed frontend
var assets embed.FS

func main() {
	if shouldLaunchGUI() {
		launchGUI()
	} else {
		cli.Execute()
	}
}

func shouldLaunchGUI() bool {
	for _, arg := range os.Args {
		if arg == "--gui" || arg == "--fyne-gui" {
			return true
		}
	}
	return false
}

func shouldLaunchFyneGUI() bool {
	for _, arg := range os.Args {
		if arg == "--fyne-gui" {
			return true
		}
	}
	return false
}

func launchGUI() {
	fmt.Println("Launching GUI interface...")

	// Check if user wants Fyne GUI or Wails GUI
	if shouldLaunchFyneGUI() {
		fmt.Println("Starting Fyne native GUI...")
		app := fynegui.NewFyneApp()
		app.Run()
	} else {
		fmt.Println("Starting Wails web GUI...")
		app := gui.NewApp()
		err := app.RunWithAssets(context.Background(), &assets)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GUI error: %v\n", err)
			os.Exit(1)
		}
	}
}