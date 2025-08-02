package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	"go-toolgit/cmd/cli"
	"go-toolgit/internal/gui"
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
		if arg == "--gui" {
			return true
		}
	}
	return false
}

func launchGUI() {
	fmt.Println("Launching GUI interface...")

	app := gui.NewApp()
	err := app.RunWithAssets(context.Background(), &assets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GUI error: %v\n", err)
		os.Exit(1)
	}
}
