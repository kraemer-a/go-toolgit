package fynegui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// ModernTheme implements a custom modern theme with enhanced colors and styling
type ModernTheme struct {
	isDark bool
}

// NewModernTheme creates a new modern theme
func NewModernTheme(dark bool) fyne.Theme {
	return &ModernTheme{isDark: dark}
}

// Color returns theme colors
func (m *ModernTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Use the manually set isDark field as the primary source of truth
	// This allows the theme toggle to work properly
	if m.isDark {
		return m.darkColor(name)
	}
	return m.lightColor(name)
}

func (m *ModernTheme) darkColor(name fyne.ThemeColorName) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 26, G: 27, B: 38, A: 255} // Dark blue-gray
	case theme.ColorNameForeground:
		return color.RGBA{R: 248, G: 248, B: 242, A: 255} // Off-white
	case theme.ColorNameButton:
		return color.RGBA{R: 55, G: 65, B: 81, A: 255} // Darker button background for better contrast
	case theme.ColorNamePrimary:
		return color.RGBA{R: 88, G: 101, B: 242, A: 255} // Muted indigo blue
	case theme.ColorNameHover:
		return color.RGBA{R: 67, G: 80, B: 200, A: 255} // Darker hover blue
	case theme.ColorNameFocus:
		return color.RGBA{R: 99, G: 102, B: 241, A: 255} // Focus blue
	case theme.ColorNameSelection:
		return color.RGBA{R: 68, G: 71, B: 90, A: 200} // Semi-transparent selection
	case theme.ColorNameError:
		return color.RGBA{R: 255, G: 85, B: 85, A: 255} // Red
	case theme.ColorNameSuccess:
		return color.RGBA{R: 80, G: 250, B: 123, A: 255} // Green
	case theme.ColorNameWarning:
		return color.RGBA{R: 255, G: 184, B: 108, A: 255} // Orange
	case theme.ColorNameDisabled:
		return color.RGBA{R: 68, G: 71, B: 90, A: 128} // Semi-transparent gray
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 40, G: 42, B: 54, A: 255} // Darker background
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 98, G: 114, B: 164, A: 255} // Muted text
	case theme.ColorNameScrollBar:
		return color.RGBA{R: 68, G: 71, B: 90, A: 255}
	case theme.ColorNameShadow:
		return color.RGBA{R: 0, G: 0, B: 0, A: 100} // Semi-transparent shadow
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (m *ModernTheme) lightColor(name fyne.ThemeColorName) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 250, G: 250, B: 250, A: 255} // Light gray
	case theme.ColorNameForeground:
		return color.RGBA{R: 40, G: 42, B: 54, A: 255} // Dark blue-gray
	case theme.ColorNameButton:
		return color.RGBA{R: 248, G: 250, B: 252, A: 255} // Very light gray for better button visibility
	case theme.ColorNamePrimary:
		return color.RGBA{R: 79, G: 70, B: 229, A: 255} // Deeper indigo
	case theme.ColorNameHover:
		return color.RGBA{R: 67, G: 56, B: 202, A: 255} // Darker hover
	case theme.ColorNameFocus:
		return color.RGBA{R: 88, G: 80, B: 236, A: 255} // Focus indigo
	case theme.ColorNameSelection:
		return color.RGBA{R: 189, G: 147, B: 249, A: 100} // Semi-transparent purple
	case theme.ColorNameError:
		return color.RGBA{R: 255, G: 85, B: 85, A: 255} // Red
	case theme.ColorNameSuccess:
		return color.RGBA{R: 80, G: 250, B: 123, A: 255} // Green
	case theme.ColorNameWarning:
		return color.RGBA{R: 255, G: 184, B: 108, A: 255} // Orange
	case theme.ColorNameDisabled:
		return color.RGBA{R: 200, G: 200, B: 200, A: 255} // Light gray
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255} // White
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 150, G: 150, B: 150, A: 255} // Gray
	case theme.ColorNameScrollBar:
		return color.RGBA{R: 200, G: 200, B: 200, A: 255}
	case theme.ColorNameShadow:
		return color.RGBA{R: 0, G: 0, B: 0, A: 50} // Light shadow
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

// Font returns the font resource for the given text style
func (m *ModernTheme) Font(style fyne.TextStyle) fyne.Resource {
	// Use default fonts for now, but could load custom fonts here
	return theme.DefaultTheme().Font(style)
}

// Icon returns the icon resource for the given theme icon name
func (m *ModernTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	// Use default icons for now, but could provide custom icons here
	return theme.DefaultTheme().Icon(name)
}

// Size returns the size metric for the given size name
func (m *ModernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 16
	case theme.SizeNameScrollBarSmall:
		return 3
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameText:
		return 14
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputBorder:
		return 2
	}
	return theme.DefaultTheme().Size(name)
}
