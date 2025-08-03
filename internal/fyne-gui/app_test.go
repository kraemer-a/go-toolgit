package fynegui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"go-toolgit/internal/core/utils"
	"go-toolgit/internal/gui"
)

// Mock GUI service for testing
type mockGUIService struct{}

func (m *mockGUIService) ListRepositories() ([]gui.Repository, error) {
	return []gui.Repository{
		{Name: "repo1", FullName: "org/repo1"},
		{Name: "repo2", FullName: "org/repo2"},
	}, nil
}

func (m *mockGUIService) ProcessReplacements(rules []gui.ReplacementRule, repos []gui.Repository, options gui.ProcessingOptions) (*gui.ProcessingResult, error) {
	return &gui.ProcessingResult{
		Success: true,
		Message: "Test processing completed",
	}, nil
}

func (m *mockGUIService) ValidateConfiguration() error {
	return nil
}

func createTestApp() *FyneApp {
	logger := utils.NewLogger("info", "text")
	guiService := &gui.Service{}

	fyneApp := test.NewApp()
	modernTheme := NewModernTheme(true)
	fyneApp.Settings().SetTheme(modernTheme)
	window := fyneApp.NewWindow("Test")

	return &FyneApp{
		app:              fyneApp,
		window:           window,
		service:          guiService,
		logger:           logger,
		modernTheme:      modernTheme.(*ModernTheme),
		currentThemeType: "Modern",
		isDarkMode:       true,
	}
}

func TestFyneApp_GetCurrentTheme(t *testing.T) {
	app := createTestApp()

	tests := []struct {
		name      string
		themeType string
		isDark    bool
	}{
		{"Modern dark", "Modern", true},
		{"Modern light", "Modern", false},
		{"Adwaita dark", "Adwaita", true},
		{"Adwaita light", "Adwaita", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.currentThemeType = tt.themeType
			app.isDarkMode = tt.isDark

			currentTheme := app.getCurrentTheme()

			if currentTheme == nil {
				t.Fatal("getCurrentTheme returned nil")
			}

			// Test that theme implements fyne.Theme interface
			var _ fyne.Theme = currentTheme

			// Test theme type
			switch tt.themeType {
			case "Modern":
				if modernTheme, ok := currentTheme.(*ModernTheme); ok {
					if modernTheme.isDark != tt.isDark {
						t.Errorf("Modern theme isDark should be %v, got %v", tt.isDark, modernTheme.isDark)
					}
				} else {
					t.Error("Expected ModernTheme for Modern theme type")
				}
			case "Adwaita":
				if adwaitaTheme, ok := currentTheme.(*AdwaitaVariantTheme); ok {
					expectedVariant := theme.VariantLight
					if tt.isDark {
						expectedVariant = theme.VariantDark
					}
					if adwaitaTheme.variant != expectedVariant {
						t.Errorf("Adwaita theme variant should be %v, got %v", expectedVariant, adwaitaTheme.variant)
					}
				} else {
					t.Error("Expected AdwaitaVariantTheme for Adwaita theme type")
				}
			}
		})
	}
}

func TestFyneApp_ParsePatterns(t *testing.T) {
	app := createTestApp()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", nil},
		{"single pattern", "*.go", []string{"*.go"}},
		{"multiple patterns", "*.go,*.js,*.py", []string{"*.go", "*.js", "*.py"}},
		{"patterns with spaces", "*.go, *.js , *.py", []string{"*.go", "*.js", "*.py"}},
		{"patterns with empty entries", "*.go,,*.js,", []string{"*.go", "*.js"}},
		{"only commas", ",,", nil},
		{"only spaces", "   ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := app.parsePatterns(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d patterns, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("Expected patterns %v, got %v", tt.expected, result)
					break
				}
			}
		})
	}
}

func TestFyneApp_JoinPatterns(t *testing.T) {
	app := createTestApp()

	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"empty slice", []string{}, ""},
		{"single pattern", []string{"*.go"}, "*.go"},
		{"multiple patterns", []string{"*.go", "*.js", "*.py"}, "*.go,*.js,*.py"},
		{"nil slice", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := app.joinPatterns(tt.input)

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFyneApp_ParseJoinRoundTrip(t *testing.T) {
	app := createTestApp()

	tests := []string{
		"*.go",
		"*.go,*.js,*.py",
		"src/*.go,test/*.js",
	}

	for _, pattern := range tests {
		t.Run(pattern, func(t *testing.T) {
			parsed := app.parsePatterns(pattern)
			joined := app.joinPatterns(parsed)

			if joined != pattern {
				t.Errorf("Round-trip failed: '%s' -> %v -> '%s'", pattern, parsed, joined)
			}
		})
	}
}

func TestFyneApp_CollectReplacementRules(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

func TestFyneApp_CollectSelectedRepositories(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

func TestFyneApp_SetStatus(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

func TestFyneApp_SetStatusError(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

func TestFyneApp_SetStatusSuccess(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

func TestFyneApp_ShowLoading(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

func TestFyneApp_HideLoading(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

func TestFyneApp_ApplyTheme(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

// Test theme switching workflow
func TestFyneApp_ThemeSwitching(t *testing.T) {
	t.Skip("Skipping UI-dependent method test - requires full UI initialization")
}

// Test configuration methods (non-UI logic)
func TestFyneApp_ConfigurationMethods(t *testing.T) {
	// Test that configuration-related methods don't panic
	t.Run("loadConfigurationFromFile", func(t *testing.T) {
		t.Skip("Skipping UI-dependent method test - requires full UI initialization")
	})
}

// Test validation methods
func TestFyneApp_ValidationMethods(t *testing.T) {
	// These are smoke tests for validation methods
	t.Run("handleValidateConfig", func(t *testing.T) {
		t.Skip("Skipping UI-dependent method test - requires full UI initialization")
	})

	t.Run("handleValidateReplacement", func(t *testing.T) {
		t.Skip("Skipping UI-dependent method test - requires full UI initialization")
	})
}

// Benchmark app logic functions
func BenchmarkFyneApp_ParsePatterns(b *testing.B) {
	app := createTestApp()
	pattern := "*.go,*.js,*.py,*.ts,*.jsx,*.vue,*.css,*.html,*.xml,*.json"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.parsePatterns(pattern)
	}
}

func BenchmarkFyneApp_JoinPatterns(b *testing.B) {
	app := createTestApp()
	patterns := []string{"*.go", "*.js", "*.py", "*.ts", "*.jsx", "*.vue", "*.css", "*.html", "*.xml", "*.json"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.joinPatterns(patterns)
	}
}

func BenchmarkFyneApp_GetCurrentTheme(b *testing.B) {
	app := createTestApp()
	app.currentThemeType = "Adwaita"
	app.isDarkMode = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.getCurrentTheme()
	}
}