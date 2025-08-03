package fynegui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	xtheme "fyne.io/x/fyne/theme"
)

func TestNewModernTheme(t *testing.T) {
	tests := []struct {
		name   string
		isDark bool
	}{
		{"dark theme", true},
		{"light theme", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := NewModernTheme(tt.isDark)
			
			if theme == nil {
				t.Fatal("NewModernTheme returned nil")
			}
			
			modernTheme, ok := theme.(*ModernTheme)
			if !ok {
				t.Fatal("NewModernTheme did not return *ModernTheme")
			}
			
			if modernTheme.isDark != tt.isDark {
				t.Errorf("Expected isDark = %v, got %v", tt.isDark, modernTheme.isDark)
			}
		})
	}
}

func TestModernTheme_Color(t *testing.T) {
	tests := []struct {
		name      string
		isDark    bool
		colorName fyne.ThemeColorName
		expected  color.Color
	}{
		// Dark theme colors
		{"dark background", true, theme.ColorNameBackground, color.RGBA{26, 27, 38, 255}},
		{"dark foreground", true, theme.ColorNameForeground, color.RGBA{248, 248, 242, 255}},
		{"dark primary", true, theme.ColorNamePrimary, color.RGBA{88, 101, 242, 255}},
		{"dark error", true, theme.ColorNameError, color.RGBA{255, 85, 85, 255}},
		{"dark success", true, theme.ColorNameSuccess, color.RGBA{80, 250, 123, 255}},
		
		// Light theme colors
		{"light background", false, theme.ColorNameBackground, color.RGBA{250, 250, 250, 255}},
		{"light foreground", false, theme.ColorNameForeground, color.RGBA{40, 42, 54, 255}},
		{"light primary", false, theme.ColorNamePrimary, color.RGBA{79, 70, 229, 255}},
		{"light error", false, theme.ColorNameError, color.RGBA{255, 85, 85, 255}},
		{"light success", false, theme.ColorNameSuccess, color.RGBA{80, 250, 123, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modernTheme := &ModernTheme{isDark: tt.isDark}
			
			result := modernTheme.Color(tt.colorName, theme.VariantLight)
			
			if result != tt.expected {
				t.Errorf("Expected color %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestModernTheme_Font(t *testing.T) {
	modernTheme := &ModernTheme{isDark: true}
	
	// Test that it delegates to default theme
	expected := theme.DefaultTheme().Font(fyne.TextStyle{Bold: true})
	result := modernTheme.Font(fyne.TextStyle{Bold: true})
	
	if result != expected {
		t.Errorf("Font method should delegate to default theme")
	}
}

func TestModernTheme_Icon(t *testing.T) {
	modernTheme := &ModernTheme{isDark: true}
	
	// Test that it delegates to default theme
	expected := theme.DefaultTheme().Icon(theme.IconNameCancel)
	result := modernTheme.Icon(theme.IconNameCancel)
	
	if result != expected {
		t.Errorf("Icon method should delegate to default theme")
	}
}

func TestModernTheme_Size(t *testing.T) {
	tests := []struct {
		name     string
		sizeName fyne.ThemeSizeName
		expected float32
	}{
		{"padding", theme.SizeNamePadding, 8},
		{"inline icon", theme.SizeNameInlineIcon, 20},
		{"text", theme.SizeNameText, 14},
		{"caption text", theme.SizeNameCaptionText, 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modernTheme := &ModernTheme{isDark: true}
			
			result := modernTheme.Size(tt.sizeName)
			
			if result != tt.expected {
				t.Errorf("Expected size %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAdwaitaVariantTheme_Color(t *testing.T) {
	baseTheme := xtheme.AdwaitaTheme()
	
	tests := []struct {
		name    string
		variant fyne.ThemeVariant
	}{
		{"dark variant", theme.VariantDark},
		{"light variant", theme.VariantLight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adwaitaTheme := &AdwaitaVariantTheme{
				baseTheme: baseTheme,
				variant:   tt.variant,
			}
			
			// Test that it forces the variant regardless of input
			result := adwaitaTheme.Color(theme.ColorNamePrimary, theme.VariantLight)
			expected := baseTheme.Color(theme.ColorNamePrimary, tt.variant)
			
			if result != expected {
				t.Errorf("AdwaitaVariantTheme should force variant %v, got different color", tt.variant)
			}
		})
	}
}

func TestAdwaitaVariantTheme_DelegatesMethods(t *testing.T) {
	baseTheme := xtheme.AdwaitaTheme()
	adwaitaTheme := &AdwaitaVariantTheme{
		baseTheme: baseTheme,
		variant:   theme.VariantDark,
	}

	t.Run("Font delegation", func(t *testing.T) {
		style := fyne.TextStyle{Bold: true}
		expected := baseTheme.Font(style)
		result := adwaitaTheme.Font(style)
		
		if result != expected {
			t.Error("Font method should delegate to base theme")
		}
	})

	t.Run("Icon delegation", func(t *testing.T) {
		expected := baseTheme.Icon(theme.IconNameCancel)
		result := adwaitaTheme.Icon(theme.IconNameCancel)
		
		if result != expected {
			t.Error("Icon method should delegate to base theme")
		}
	})

	t.Run("Size delegation", func(t *testing.T) {
		expected := baseTheme.Size(theme.SizeNamePadding)
		result := adwaitaTheme.Size(theme.SizeNamePadding)
		
		if result != expected {
			t.Error("Size method should delegate to base theme")
		}
	})
}

func TestModernTheme_ColorVariantIgnored(t *testing.T) {
	// Test that ModernTheme ignores the variant parameter and uses isDark field
	modernTheme := &ModernTheme{isDark: true}
	
	// Should return dark color regardless of variant parameter
	darkResult := modernTheme.Color(theme.ColorNamePrimary, theme.VariantDark)
	lightVariantResult := modernTheme.Color(theme.ColorNamePrimary, theme.VariantLight)
	
	if darkResult != lightVariantResult {
		t.Error("ModernTheme should ignore variant parameter and use isDark field")
	}
	
	expected := color.RGBA{88, 101, 242, 255} // Dark primary color
	if darkResult != expected {
		t.Errorf("Expected dark primary color %v, got %v", expected, darkResult)
	}
}

func TestThemeInterface(t *testing.T) {
	// Test that both themes implement fyne.Theme interface
	var _ fyne.Theme = &ModernTheme{}
	var _ fyne.Theme = &AdwaitaVariantTheme{}
}

// Benchmark theme color lookups
func BenchmarkModernTheme_Color(b *testing.B) {
	modernTheme := &ModernTheme{isDark: true}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		modernTheme.Color(theme.ColorNamePrimary, theme.VariantDark)
	}
}

func BenchmarkAdwaitaVariantTheme_Color(b *testing.B) {
	adwaitaTheme := &AdwaitaVariantTheme{
		baseTheme: xtheme.AdwaitaTheme(),
		variant:   theme.VariantDark,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adwaitaTheme.Color(theme.ColorNamePrimary, theme.VariantLight)
	}
}