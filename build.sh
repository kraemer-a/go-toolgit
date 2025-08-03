#!/bin/bash

# Build script for go-toolgit with multiple GUI options

if [ "$1" = "wails" ]; then
    echo "Building Wails GUI version..."
    wails build -tags wails
    echo "Wails app built at: build/bin/go-toolgit.app"
elif [ "$1" = "fyne" ]; then
    echo "Building Fyne GUI version..."
    go build -o go-toolgit
    echo "Fyne binary built at: ./go-toolgit"
else
    echo "Usage: ./build.sh [wails|fyne]"
    echo "  wails - Build with Wails web-based GUI"
    echo "  fyne  - Build with Fyne native GUI"
    exit 1
fi